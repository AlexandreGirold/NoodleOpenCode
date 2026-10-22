// Package inventory finds architectural inputs and extracts supported declarations.
// It never executes a project, renders templates, reads environment files or edits a model.
package inventory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/TheGostsniperfr/Noodle/internal/adapter"
	"github.com/TheGostsniperfr/Noodle/internal/adapter/k8s"
	"github.com/TheGostsniperfr/Noodle/internal/model"
)

const APIVersion = "noodle.inventory/v1alpha1"

// ponytail: bound individual inputs at 8 MiB; revisit for larger rendered bundles.
const maxFileBytes = 8 << 20

type Report struct {
	APIVersion  string                       `yaml:"apiVersion"`
	Kind        string                       `yaml:"kind"`
	Roots       []Root                       `yaml:"roots"`
	Files       []File                       `yaml:"files,omitempty"`
	Elements    []model.DiscoveredElement    `yaml:"elements,omitempty"`
	Connections []model.DiscoveredConnection `yaml:"connections,omitempty"`
	References  []model.DiscoveredReference  `yaml:"references,omitempty"`
	Packages    []Package                    `yaml:"packages,omitempty"`
	Unresolved  []Issue                      `yaml:"unresolved,omitempty"`
}

type Root struct {
	ID   string `yaml:"id"`
	Path string `yaml:"path"`
}

type File struct {
	Root   string `yaml:"root"`
	Path   string `yaml:"path"`
	Format string `yaml:"format"`
	Status string `yaml:"status"`
	SHA256 string `yaml:"sha256,omitempty"`
	Bytes  int64  `yaml:"bytes"`
}

type Issue struct {
	Format string    `yaml:"format"`
	Reason string    `yaml:"reason"`
	Src    model.Src `yaml:"src"`
}

type Package struct {
	Ecosystem    string       `yaml:"ecosystem"`
	Name         string       `yaml:"name,omitempty"`
	Dependencies []Dependency `yaml:"dependencies,omitempty"`
	Src          model.Src    `yaml:"src"`
}

type Dependency struct {
	Name  string    `yaml:"name"`
	Scope string    `yaml:"scope"`
	Src   model.Src `yaml:"src"`
}

var ignoredDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, ".venv": true, "venv": true,
	"node_modules": true, "vendor": true, ".terraform": true, ".cache": true,
	"__pycache__": true, "dist": true, "build": true, "coverage": true,
	".next": true, ".nuxt": true, ".noodle": true, ".ua": true, ".understand-anything": true,
	".aws": true, ".ssh": true, ".kube": true,
}

// Scan produces stable output for the same roots/contents, independent of root order.
// Namespace supplies the default namespace of standalone Kubernetes objects.
func Scan(ctx context.Context, paths []string, namespace string) (*Report, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	roots, err := sourceRoots(paths)
	if err != nil {
		return nil, err
	}
	r := &Report{APIVersion: APIVersion, Kind: "Inventory", Roots: roots}
	for _, root := range roots {
		manifests := map[string][]string{}
		var renderDirs []string
		err := filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return fmt.Errorf("walk %s: %w", path, walkErr)
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if d.IsDir() {
				if path != root.Path && ignoredDirs[d.Name()] {
					return filepath.SkipDir
				}
				if hasRenderMarker(path) {
					renderDirs = append(renderDirs, path)
				}
				return nil
			}
			if sensitiveName(d.Name()) {
				return nil
			}
			format := namedFormat(d.Name(), path)
			ext := strings.ToLower(filepath.Ext(path))
			if format == "" && ext != ".yaml" && ext != ".yml" && ext != ".json" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("stat %s: %w", path, err)
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root.Path, path)
			if err != nil {
				return err
			}
			file := File{Root: root.ID, Path: filepath.ToSlash(rel), Format: format, Bytes: info.Size(), Status: "inventory-only"}
			src := model.Src{File: root.ID + "/" + file.Path, Line: 1}
			if info.Size() > maxFileBytes {
				if format == "" {
					file.Format = "yaml-or-json"
				}
				file.Status = "unresolved"
				r.Files = append(r.Files, file)
				r.issue(file.Format, "input-too-large", src)
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			var doc *yaml.Node
			if format == "" {
				for _, dir := range renderDirs {
					if inside(dir, path) {
						format, file.Format, file.Status = "render-input", "render-input", "render-required"
						break
					}
				}
			}
			if format == "" || format == "compose" || format == "npm" {
				doc, err = document(raw, format == "compose" || format == "npm")
				if format == "npm" && !json.Valid(raw) {
					err = fmt.Errorf("invalid JSON")
				}
				if err != nil {
					// Never copy parser errors containing snippets of input/credentials.
					if format == "" && doc != nil && field(doc, "apiVersion") != nil && field(doc, "kind") != nil && !strings.HasPrefix(field(doc, "apiVersion").Value, "noodle") {
						format, file.Format = "kubernetes", "kubernetes"
					}
					if format != "" {
						file.Status = "unresolved"
						r.issue(format, "invalid-document", src)
					} else {
						return nil
					}
				}
				if format == "" && doc != nil {
					api, kind := field(doc, "apiVersion"), field(doc, "kind")
					if api != nil && kind != nil && !strings.HasPrefix(api.Value, "noodle") {
						format, file.Format = "kubernetes", "kubernetes"
					} else {
						return nil
					}
				}
			}
			if format == "" {
				return nil
			}
			if format == "kubernetes" && file.Status != "unresolved" {
				if dynamic := dynamicKubernetes(raw); dynamic != nil {
					file.Status = "render-required"
					r.issue(format, "dynamic-declaration", at(src, dynamic))
				}
			}
			sum := sha256.Sum256(raw)
			file.SHA256 = fmt.Sprintf("%x", sum)
			if file.Status != "unresolved" {
				switch format {
				case "kubernetes":
					if file.Status == "render-required" {
						break
					}
					file.Status = "extracted"
					for _, dir := range renderDirs {
						if inside(dir, path) {
							file.Status = "render-required"
							break
						}
					}
					if file.Status == "extracted" {
						dir := filepath.Dir(path)
						manifests[dir] = append(manifests[dir], path)
					}
				case "compose":
					file.Status = "extracted"
					r.compose(doc, src)
				case "npm":
					file.Status = "extracted"
					r.npm(doc, raw, src)
				case "requirements":
					file.Status = "extracted"
					r.requirements(raw, src)
				case "helm", "kustomize":
					file.Status = "render-required"
					r.issue(format, "render-required", src)
				case "render-input": // its Helm/Kustomize marker already records the gap
				default:
					r.issue(format, "parser-not-implemented", src)
				}
			}
			r.Files = append(r.Files, file)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", root.Path, err)
		}
		var scopes []string
		for scope := range manifests {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)
		for _, scope := range scopes {
			fragment, stats, err := (k8s.Adapter{}).Discover(ctx, adapter.Source{Paths: manifests[scope], Namespace: namespace})
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				for _, path := range manifests[scope] {
					rel, _ := filepath.Rel(root.Path, path)
					r.issue("kubernetes", "extraction-failed-review-required", model.Src{File: root.ID + "/" + filepath.ToSlash(rel), Line: 1})
					for i := range r.Files {
						if r.Files[i].Root == root.ID && r.Files[i].Path == filepath.ToSlash(rel) {
							r.Files[i].Status = "unresolved"
						}
					}
				}
				continue
			}
			r.addKubernetes(root, scope, fragment)
			if stats.Skipped["duplicate"] > 0 {
				rel, _ := filepath.Rel(root.Path, manifests[scope][0])
				r.issue("kubernetes", "duplicate-declarations-review-required", model.Src{File: root.ID + "/" + filepath.ToSlash(rel), Line: 1})
			}
		}
	}
	sort.Slice(r.Elements, func(i, j int) bool { return r.Elements[i].ID < r.Elements[j].ID })
	sort.Slice(r.Connections, func(i, j int) bool { return r.Connections[i].ID < r.Connections[j].ID })
	sort.Slice(r.References, func(i, j int) bool { return r.References[i].ID < r.References[j].ID })
	sort.Slice(r.Unresolved, func(i, j int) bool {
		a, b := r.Unresolved[i], r.Unresolved[j]
		if a.Src.File != b.Src.File {
			return a.Src.File < b.Src.File
		}
		if a.Src.Line != b.Src.Line {
			return a.Src.Line < b.Src.Line
		}
		return a.Reason < b.Reason
	})
	return r, nil
}

func sourceRoots(paths []string) ([]Root, error) {
	unique := map[string]bool{}
	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		abs, err = filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("source root %s: %w", path, err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("source root %s is not a directory", path)
		}
		unique[abs] = true
	}
	var sorted []string
	for path := range unique {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	names := map[string]int{}
	for _, path := range sorted {
		names[filepath.Base(path)]++
	}
	var roots []Root
	for _, path := range sorted {
		name := filepath.Base(path)
		if names[name] > 1 {
			sum := sha256.Sum256([]byte(filepath.ToSlash(path)))
			name += fmt.Sprintf("-%x", sum[:4])
		}
		roots = append(roots, Root{ID: name, Path: filepath.ToSlash(path)})
	}
	return roots, nil
}

func inside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func hasRenderMarker(dir string) bool {
	for _, name := range []string{"Chart.yaml", "kustomization.yaml", "kustomization.yml", "Kustomization"} {
		if info, err := os.Lstat(filepath.Join(dir, name)); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func sensitiveName(name string) bool {
	n := strings.ToLower(name)
	return n == ".env" || strings.HasPrefix(n, ".env.") || n == "credentials" || n == "credentials.json" || n == "application_default_credentials.json" || n == "kubeconfig" || strings.HasSuffix(n, ".tfstate") || strings.Contains(n, ".tfstate.") || strings.HasSuffix(n, ".tfvars") || strings.HasSuffix(n, ".tfvars.json")
}

func namedFormat(name, path string) string {
	lower := strings.ToLower(name)
	switch {
	case lower == "package.json":
		return "npm"
	case requirementsName.MatchString(lower):
		return "requirements"
	case lower == "compose.yaml" || lower == "compose.yml" || lower == "docker-compose.yaml" || lower == "docker-compose.yml" || ((strings.HasPrefix(lower, "compose.") || strings.HasPrefix(lower, "docker-compose.")) && (strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml"))):
		return "compose"
	case name == "Chart.yaml" || name == "Chart.lock":
		return "helm"
	case lower == "kustomization.yaml" || lower == "kustomization.yml" || name == "Kustomization":
		return "kustomize"
	case strings.HasSuffix(lower, ".tf") || strings.HasSuffix(lower, ".tf.json"):
		return "terraform"
	case lower == "pyproject.toml" || lower == "pipfile":
		return "python-manifest"
	case lower == "go.mod":
		return "go-manifest"
	case lower == "cargo.toml":
		return "rust-manifest"
	case lower == "pom.xml" || lower == "build.gradle" || lower == "build.gradle.kts":
		return "jvm-manifest"
	case lower == "composer.json":
		return "php-manifest"
	case lower == "dockerfile" || strings.HasPrefix(lower, "dockerfile."):
		return "dockerfile"
	case lower == ".gitlab-ci.yml" || lower == ".gitlab-ci.yaml" || (strings.Contains(filepath.ToSlash(path), "/.github/workflows/") && (strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml"))):
		return "ci"
	}
	return ""
}

func (r *Report) issue(format, reason string, src model.Src) {
	issue := Issue{Format: format, Reason: reason, Src: src}
	for _, existing := range r.Unresolved {
		if existing == issue {
			return
		}
	}
	r.Unresolved = append(r.Unresolved, issue)
}

// Encode is compact, ordered YAML. It contains no wall-clock observation field.
func Encode(w io.Writer, report *Report) error {
	var doc yaml.Node
	if err := doc.Encode(report); err != nil {
		return fmt.Errorf("encode inventory: %w", err)
	}
	var compact func(*yaml.Node)
	compact = func(n *yaml.Node) {
		for i, child := range n.Content {
			if n.Kind == yaml.MappingNode && i%2 == 1 && n.Content[i-1].Value != "packages" && child.Kind == yaml.SequenceNode {
				for _, item := range child.Content {
					item.Style = yaml.FlowStyle
				}
			} else {
				compact(child)
			}
		}
	}
	compact(&doc)
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	return enc.Close()
}

func document(raw []byte, single bool) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	var first *yaml.Node
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				if first != nil {
					return first, nil
				}
				return nil, fmt.Errorf("empty document")
			}
			return first, err
		}
		if len(doc.Content) == 0 || doc.Content[0].Tag == "!!null" {
			continue
		}
		var checked any
		if err := doc.Decode(&checked); err != nil {
			return doc.Content[0], err
		}
		if first != nil && single {
			return nil, fmt.Errorf("multiple documents are unsupported for this format")
		}
		if first == nil {
			first = doc.Content[0]
		}
	}
}
