package model_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/TheGostsniperfr/Noodle/internal/model"
)

func compileSchemas(t *testing.T) map[string]map[string]*jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	// Register canonical IDs as well as file locations so cross-version references
	// resolve offline, even when a schema's $id is an HTTPS URL.
	for _, version := range []string{"v1alpha1", "v1alpha2"} {
		for _, kind := range []string{"model", "view", "layout", "fragment"} {
			if version == "v1alpha2" && kind == "fragment" {
				continue
			}
			path, err := filepath.Abs(filepath.Join("../../schemas", version, kind+".schema.json"))
			require.NoError(t, err)
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			require.NoError(t, err)
			require.NoError(t, c.AddResource(path, doc))
			require.NoError(t, c.AddResource(doc.(map[string]any)["$id"].(string), doc))
		}
	}
	out := map[string]map[string]*jsonschema.Schema{}
	for _, version := range []string{"v1alpha1", "v1alpha2"} {
		out[version] = map[string]*jsonschema.Schema{}
		kinds := []string{"Model", "View", "Layout"}
		if version == "v1alpha1" {
			kinds = append(kinds, "Fragment")
		}
		for _, kind := range kinds {
			path, err := filepath.Abs(filepath.Join("../../schemas", version, strings.ToLower(kind)+".schema.json"))
			require.NoError(t, err)
			s, err := c.Compile(path)
			require.NoError(t, err, kind)
			out[version][kind] = s
		}
	}
	return out
}

// yamlToJSONValue goes through encoding/json so numbers and maps have the types the
// validator expects.
func yamlToJSONValue(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	require.NoError(t, yaml.Unmarshal(raw, &v))
	b, err := json.Marshal(v)
	require.NoError(t, err)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	require.NoError(t, err)
	return doc
}

// contractFiles returns every model.yaml, views/*.yaml, layouts/*.yaml and
// discovered/*.yaml under root.
func contractFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}
		switch {
		case d.Name() == "model.yaml":
			files[path] = "Model"
		case filepath.Base(filepath.Dir(path)) == "views":
			files[path] = "View"
		case filepath.Base(filepath.Dir(path)) == "layouts":
			files[path] = "Layout"
		case filepath.Base(filepath.Dir(path)) == "discovered":
			files[path] = "Fragment"
		}
		return nil
	})
	require.NoError(t, err)
	return files
}

func TestSchemas_AcceptEveryContractFileInTheRepository(t *testing.T) {
	t.Parallel()
	schemas := compileSchemas(t)
	files := contractFiles(t, "testdata/minimal")
	for path, kind := range contractFiles(t, "../../examples") {
		files[path] = kind
	}
	require.NotEmpty(t, files)

	for path, kind := range files {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			require.NoError(t, err)

			var version struct {
				APIVersion string `yaml:"apiVersion"`
			}
			require.NoError(t, yaml.Unmarshal(raw, &version))
			ver := strings.TrimPrefix(version.APIVersion, "noodle/")
			err = schemas[ver][kind].Validate(yamlToJSONValue(t, raw))

			assert.NoError(t, err)
		})
	}
}

func TestSchemas_AcceptEncodedFragment_WithEmptyFieldsLeftOut(t *testing.T) {
	t.Parallel()
	schemas := compileSchemas(t)
	s, err := model.LoadSystem("testdata/minimal")
	require.NoError(t, err)
	var buf bytes.Buffer

	require.NoError(t, model.EncodeFragment(&buf, s.Fragments["k8s-manifests"]))
	err = schemas["v1alpha1"]["Fragment"].Validate(yamlToJSONValue(t, buf.Bytes()))

	assert.NoError(t, err)
}

func TestArchitectureEvidenceSchemaRuntimeParity(t *testing.T) {
	schema := compileSchemas(t)["v1alpha2"]["Model"]
	for _, tc := range []struct {
		name, evidence string
		valid          bool
	}{
		{"file", "{how: code, sources: [{file: src/api.go, line: 0}]}", true},
		{"object", "{how: config, sources: [{object: prod/Deployment/api}]}", true},
		{"unknown", "{how: unknown, note: Confirm the deployment.}", true},
		{"aliased how", "{note: &kind code, how: *kind, sources: [{file: x}]}", true},
		{"aliased note", "{how: &kind unknown, note: *kind}", true},
		{"aliased file", "{how: config, note: &path compose.yaml, sources: [{file: *path}]}", true},
		{"aliased line", "{how: config, sources: [{file: a, line: &line 1}, {file: b, line: *line}]}", true},
		{"blank note", "{how: unknown, note: '   '}", false},
		{"empty note", "{how: unknown, note: ''}", false},
		{"numeric note", "{how: unknown, note: 123}", false},
		{"null sources", "{how: unknown, note: Unknown., sources: null}", false},
		{"missing source", "{how: code}", false},
		{"invalid classification", "{how: observed, sources: [{file: x}]}", false},
		{"both source alternatives", "{how: config, sources: [{file: x, object: ''}]}", false},
		{"blank source", "{how: config, sources: [{file: ' '}]}", false},
		{"negative line", "{how: config, sources: [{file: x, line: -1}]}", false},
		{"object with line", "{how: config, sources: [{object: x, line: 0}]}", false},
		{"unknown source field", "{how: config, sources: [{file: x, typo: y}]}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := []byte("apiVersion: noodle/v1alpha2\nkind: Model\nelements: [{id: api, kind: backend}]\nevidence: {api: " + tc.evidence + "}\n")
			schemaErr := schema.Validate(yamlToJSONValue(t, doc))
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "model.yaml"), doc, 0o644))
			s, loadErr := model.LoadSystem(dir)
			runtimeValid := loadErr == nil
			if runtimeValid {
				runtimeValid = len(model.CheckArchitecture(s)) == 0
			}
			assert.Equal(t, tc.valid, schemaErr == nil, "schema: %v", schemaErr)
			assert.Equal(t, tc.valid, runtimeValid, "load: %v", loadErr)
		})
	}
}

func TestArchitectureSharedEvidenceSources(t *testing.T) {
	doc := []byte(`apiVersion: noodle/v1alpha2
kind: Model
elements: [{id: a, kind: backend}, {id: b, kind: backend}]
evidence:
  a: {how: config, sources: &sources [{file: compose.yaml}]}
  b: {how: config, sources: *sources}
`)
	schema := compileSchemas(t)["v1alpha2"]["Model"]
	require.NoError(t, schema.Validate(yamlToJSONValue(t, doc)))
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "model.yaml"), doc, 0o644))
	s, err := model.LoadSystem(dir)
	require.NoError(t, err)
	assert.Empty(t, model.CheckArchitecture(s))
	assert.Equal(t, s.Model.Evidence["a"].Sources, s.Model.Evidence["b"].Sources)
}

func TestSchemas_Reject(t *testing.T) {
	t.Parallel()
	schemas := compileSchemas(t)

	tests := []struct {
		name, kind, doc string
	}{
		{"a blocked connection kind, replaced by denied", "Model", modelHead + "connections: [{id: c, from: a, to: b, kind: blocked}]\n"},
		{"enforced_by without denied", "Model", modelHead + "connections: [{id: c, from: a, to: b, kind: flow, enforced_by: [np]}]\n"},
		{"multiplicity as a boolean flag", "Model", modelHead + "elements: [{id: a, kind: backend, multiplicity: true}]\n"},
		{"a port number out of range", "Model", modelHead + "elements: [{id: a, kind: backend, ports: [{name: p, protocol: TCP, port: 70000}]}]\n"},
		{"a curated id in matches", "Model", modelHead + "elements: [{id: a, kind: backend, matches: [api]}]\n"},
		{"the same id twice in matches", "Model", modelHead + `elements: [{id: a, kind: backend, matches: ["k8s:p/service/a", "k8s:p/service/a"]}]` + "\n"},
		{"a fragment without provenance", "Fragment", fragmentHead},
		{"a curated id in a fragment", "Fragment", fragmentHead + fragmentProvenance + "elements: [{id: api, kind: backend, src: {object: p/Deployment/api}}]\n"},
		{"a fragment item without src", "Fragment", fragmentHead + fragmentProvenance + `elements: [{id: "k8s:p/deployment/api", kind: backend}]` + "\n"},
		{"a src with a line but no file", "Fragment", fragmentHead + fragmentProvenance + `elements: [{id: "k8s:p/deployment/api", kind: backend, src: {object: o, line: 3}}]` + "\n"},
		{"a src that is both a file and an object", "Fragment", fragmentHead + fragmentProvenance + `elements: [{id: "k8s:p/deployment/api", kind: backend, src: {file: f, object: o}}]` + "\n"},
		{"curated presentation in a fragment", "Fragment", fragmentHead + fragmentProvenance + `elements: [{id: "k8s:p/deployment/api", kind: backend, status: planned, src: {file: f}}]` + "\n"},
		{"matches in a fragment", "Fragment", fragmentHead + fragmentProvenance + `elements: [{id: "k8s:p/deployment/api", kind: backend, matches: ["k8s:x/y/z"], src: {file: f}}]` + "\n"},
		{"a view without type", "View", viewHead + "title: T\n"},
		{"participants on a topology view", "View", viewHead + "type: topology\nparticipants: [a]\n"},
		{"a message step in a topology view", "View", viewHead + "type: topology\nsteps: [{from: a, to: b, over: c}]\n"},
		{"a bare connection id in a sequence view", "View", viewHead + "type: sequence\nsteps: [c]\n"},
		{"a message without over", "View", viewHead + "type: sequence\nsteps: [{from: a, to: b}]\n"},
		{"a step that is both a reply and a note", "View", viewHead + "type: sequence\nsteps: [{reply: x, note: a}]\n"},
		{"an endpoint without side", "Layout", layoutHead + "edges: {c: {from: a, to: b.left}}\n"},
		{"a ratio above 100 %", "Layout", layoutHead + "edges: {c: {from: a.right@120%, to: b.left}}\n"},
		{"an offset without unit", "Layout", layoutHead + "edges: {c: {from: a.right@40, to: b.left}}\n"},
		{"a lane with both x and y", "Layout", layoutHead + "lanes: {bus: {x: 1, y: 2}}\n"},
		{"absolute v0 path field", "Layout", layoutHead + "edges: {c: {from: a.right, to: b.left, path: [[0, 0]]}}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var version struct {
				APIVersion string `yaml:"apiVersion"`
			}
			require.NoError(t, yaml.Unmarshal([]byte(tt.doc), &version))
			ver := strings.TrimPrefix(version.APIVersion, "noodle/")
			err := schemas[ver][tt.kind].Validate(yamlToJSONValue(t, []byte(tt.doc)))

			assert.Error(t, err)
		})
	}
}

const (
	fragmentHead       = "apiVersion: noodle/v1alpha1\nkind: Fragment\n"
	fragmentProvenance = `provenance: {adapter: k8s, source: manifests, observedAt: "2026-09-30T12:00:00Z"}` + "\n"
)
