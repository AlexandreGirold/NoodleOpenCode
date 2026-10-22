package model

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// System is one directory: a model, its views and the layouts of its topology views.
type System struct {
	Dir     string
	Model   *Model
	Views   map[string]*View
	Layouts map[string]*Layout
	// Fragments are read from discovered/ when it exists (ADR-0015).
	Fragments map[string]*Fragment
}

func (s *System) ViewIDs() []string {
	ids := make([]string, 0, len(s.Views))
	for id := range s.Views {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func LoadSystem(dir string) (*System, error) {
	s := &System{Dir: dir, Model: &Model{}, Views: map[string]*View{}, Layouts: map[string]*Layout{}, Fragments: map[string]*Fragment{}}
	if err := decodeFile(filepath.Join(dir, "model.yaml"), "Model", s.Model); err != nil {
		return nil, fmt.Errorf("LoadSystem: %w", err)
	}
	views, err := yamlFiles(filepath.Join(dir, "views"))
	if err != nil {
		return nil, fmt.Errorf("LoadSystem: %w", err)
	}
	for _, path := range views {
		v := &View{ID: stem(path)}
		if err := decodeFile(path, "View", v); err != nil {
			return nil, fmt.Errorf("LoadSystem: %w", err)
		}
		if v.APIVersion != s.Model.APIVersion {
			return nil, fmt.Errorf("LoadSystem: %s: apiVersion %q differs from model apiVersion %q", path, v.APIVersion, s.Model.APIVersion)
		}
		s.Views[v.ID] = v
	}
	layouts, err := yamlFiles(filepath.Join(dir, "layouts"))
	if err != nil {
		return nil, fmt.Errorf("LoadSystem: %w", err)
	}
	for _, path := range layouts {
		l := &Layout{ID: stem(path)}
		if err := decodeFile(path, "Layout", l); err != nil {
			return nil, fmt.Errorf("LoadSystem: %w", err)
		}
		if l.APIVersion != s.Model.APIVersion {
			return nil, fmt.Errorf("LoadSystem: %s: apiVersion %q differs from model apiVersion %q", path, l.APIVersion, s.Model.APIVersion)
		}
		s.Layouts[l.ID] = l
	}
	fragments, err := yamlFiles(filepath.Join(dir, "discovered"))
	if err != nil {
		return nil, fmt.Errorf("LoadSystem: %w", err)
	}
	for _, path := range fragments {
		f := &Fragment{ID: stem(path)}
		if err := decodeFile(path, "Fragment", f); err != nil {
			return nil, fmt.Errorf("LoadSystem: %w", err)
		}
		s.Fragments[f.ID] = f
	}
	return s, nil
}

// header is decoded first so a file with the wrong kind fails on its kind, not on the
// first field that kind does not know.
type header struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

func decodeFile(path, kind string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var h header
	if err := yaml.Unmarshal(raw, &h); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if kind == "Fragment" && h.APIVersion != APIVersion {
		return fmt.Errorf("%s: apiVersion %q, want %q", path, h.APIVersion, APIVersion)
	}
	if kind != "Fragment" && h.APIVersion != APIVersion && h.APIVersion != ArchitectureVersion {
		return fmt.Errorf("%s: apiVersion %q, want %q or %q", path, h.APIVersion, APIVersion, ArchitectureVersion)
	}
	if h.Kind != kind {
		return fmt.Errorf("%s: kind %q, want %q", path, h.Kind, kind)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	// A v2 field in a legacy document must not be accepted merely because the Go
	// struct knows it. Presence matters even for an explicitly empty map.
	if h.APIVersion == APIVersion && kind == "Model" {
		var fields map[string]yaml.Node
		if err := yaml.Unmarshal(raw, &fields); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if _, exists := fields["evidence"]; exists {
			return fmt.Errorf("%s: evidence requires apiVersion %q", path, ArchitectureVersion)
		}
	}
	return nil
}

func yamlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths, nil
}

func stem(path string) string { return strings.TrimSuffix(filepath.Base(path), ".yaml") }
