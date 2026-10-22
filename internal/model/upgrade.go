package model

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// UpgradeDirectory creates a v1alpha2 copy of a valid v1alpha1 system. YAML nodes are
// edited in place so comments, ordering, custom view fields and expressions survive.
func UpgradeDirectory(src, dst string) error {
	s, err := LoadSystem(src)
	if err != nil {
		return fmt.Errorf("UpgradeDirectory: load source: %w", err)
	}
	if s.Model.APIVersion != APIVersion {
		return fmt.Errorf("UpgradeDirectory: source apiVersion %q, want %q", s.Model.APIVersion, APIVersion)
	}
	if fs := Check(s); len(fs) > 0 {
		return fmt.Errorf("UpgradeDirectory: source invalid: %s", fs[0])
	}
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("UpgradeDirectory: mkdir parent: %w", err)
	}
	stage, err := os.MkdirTemp(parent, ".upgrade-*")
	if err != nil {
		return fmt.Errorf("UpgradeDirectory: stage: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := upgradeYAML(filepath.Join(src, "model.yaml"), filepath.Join(stage, "model.yaml"), s.Model); err != nil {
		return fmt.Errorf("UpgradeDirectory: %w", err)
	}
	for _, sub := range []string{"views", "layouts"} {
		paths, err := yamlFiles(filepath.Join(src, sub))
		if err != nil {
			return fmt.Errorf("UpgradeDirectory: %s: %w", sub, err)
		}
		if len(paths) == 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Join(stage, sub), 0755); err != nil {
			return err
		}
		for _, p := range paths {
			if err := upgradeYAML(p, filepath.Join(stage, sub, filepath.Base(p)), nil); err != nil {
				return fmt.Errorf("UpgradeDirectory: %w", err)
			}
		}
	}
	if err := copyTree(filepath.Join(src, "discovered"), filepath.Join(stage, "discovered")); err != nil {
		return fmt.Errorf("UpgradeDirectory: copy discovered: %w", err)
	}
	readback, err := LoadSystem(stage)
	if err != nil {
		return fmt.Errorf("UpgradeDirectory: read-back load: %w", err)
	}
	if fs := CheckArchitecture(readback); len(fs) > 0 {
		return fmt.Errorf("UpgradeDirectory: read-back invalid: %s", fs[0])
	}
	// Mkdir is the exclusive destination claim. Only this invocation removes it.
	if err := os.Mkdir(dst, 0755); err != nil {
		return fmt.Errorf("UpgradeDirectory: destination %q: %w", dst, err)
	}
	owned := true
	defer func() {
		if owned {
			_ = os.RemoveAll(dst)
		}
	}()
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(stage, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return fmt.Errorf("UpgradeDirectory: install %s: %w", e.Name(), err)
		}
	}
	owned = false
	return nil
}

func upgradeYAML(src, dst string, sourceModel *Model) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%s: expected YAML mapping", src)
	}
	root := doc.Content[0]
	versionIndex := -1
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "apiVersion" {
			versionIndex = i + 1
			break
		}
	}
	if versionIndex == -1 {
		return fmt.Errorf("%s: apiVersion must be %q", src, APIVersion)
	}
	version := root.Content[versionIndex]
	var original string
	if err := version.Decode(&original); err != nil || original != APIVersion {
		return fmt.Errorf("%s: apiVersion must be %q", src, APIVersion)
	}
	// Other fields may alias an anchored apiVersion. Materialize those scalar
	// aliases before replacing the header so their original values do not change.
	var preserveAliases func(*yaml.Node)
	preserveAliases = func(node *yaml.Node) {
		if node.Kind == yaml.AliasNode && node.Alias == version {
			copy := *version
			copy.Anchor = ""
			*node = copy
			return
		}
		for _, child := range node.Content {
			preserveAliases(child)
		}
	}
	preserveAliases(root)
	root.Content[versionIndex] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ArchitectureVersion,
		HeadComment: version.HeadComment, LineComment: version.LineComment, FootComment: version.FootComment}
	if sourceModel != nil {
		facts := map[string]Evidence{}
		unknown := Evidence{How: "unknown", Note: "no evidence recorded"}
		for _, e := range sourceModel.Elements {
			facts[e.ID] = unknown
		}
		for _, e := range sourceModel.Connections {
			facts[e.ID] = unknown
		}
		for _, e := range sourceModel.References {
			facts[e.ID] = unknown
		}
		var evidence yaml.Node
		if err := evidence.Encode(facts); err != nil {
			return fmt.Errorf("%s: encode evidence: %w", src, err)
		}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "evidence"}, &evidence)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, out, 0644)
}

func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("discovered directory may not be a symlink")
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", src)
	}
	return filepath.Walk(src, func(p string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, e := filepath.Rel(src, p)
		if e != nil {
			return e
		}
		target := filepath.Join(dst, rel)
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in discovered tree: %s", p)
		}
		if fi.IsDir() {
			return os.MkdirAll(target, fi.Mode())
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		return os.WriteFile(target, b, fi.Mode())
	})
}
