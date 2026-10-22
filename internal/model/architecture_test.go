package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheGostsniperfr/Noodle/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchitectureLoadAndEvidence(t *testing.T) {
	tests := []struct{ name, evidence, want string }{
		{"complete", `a: {how: code, sources: [{file: src/a.go, line: 0}]}`, ""},
		{"missing fact", ``, "missing evidence"},
		{"unknown without note", `a: {how: unknown}`, "unknown evidence needs a note"},
		{"nonunknown without source", `a: {how: docs}`, "needs at least one source"},
		{"bad source shape", `a: {how: inferred, sources: [{file: x, object: y}]}`, "exactly one"},
		{"orphan", `a: {how: code, sources: [{file: x}]}
extra: {how: unknown, note: unclear}`, "orphan evidence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(d, "views"), 0755))
			body := "apiVersion: noodle/v1alpha2\nkind: Model\nelements: [{id: a, kind: backend}]\nevidence:\n"
			if tt.evidence != "" {
				for _, line := range strings.Split(tt.evidence, "\n") {
					body += "  " + line + "\n"
				}
			}
			require.NoError(t, os.WriteFile(filepath.Join(d, "model.yaml"), []byte(body), 0644))
			require.NoError(t, os.WriteFile(filepath.Join(d, "views", "top.yaml"), []byte("apiVersion: noodle/v1alpha2\nkind: View\ntype: topology\nsteps: []\n"), 0644))
			s, err := model.LoadSystem(d)
			if err != nil {
				require.NotEmpty(t, tt.want)
				assert.ErrorContains(t, err, tt.want)
				return
			}
			findings := model.CheckArchitecture(s)
			if tt.want == "" {
				assert.Empty(t, findings)
				assert.NotEmpty(t, model.Check(s), "drawio still requires a layout")
			} else {
				assert.Contains(t, strings.Join(func() []string {
					var out []string
					for _, f := range findings {
						out = append(out, f.Msg)
					}
					return out
				}(), "\n"), tt.want)
			}
		})
	}
}

func TestUpgradePreservesVersionAliases(t *testing.T) {
	for _, tc := range []struct{ name, view string }{
		{"header anchor", "apiVersion: &version noodle/v1alpha1\nkind: View\ntype: landscape\nsubtitle: *version\nbands: [{id: b, title: B, sections: [{items: [a]}]}]\n"},
		{"header alias", "subtitle: &version noodle/v1alpha1\napiVersion: *version\nkind: View\ntype: landscape\nbands: [{id: b, title: B, sections: [{items: [a]}]}]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(src, "views"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(src, "model.yaml"), []byte("apiVersion: noodle/v1alpha1\nkind: Model\nelements: [{id: a, kind: backend}]\n"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(src, "views", "stack.yaml"), []byte(tc.view), 0o644))
			dst := filepath.Join(t.TempDir(), "upgraded")
			require.NoError(t, model.UpgradeDirectory(src, dst))
			s, err := model.LoadSystem(dst)
			require.NoError(t, err)
			assert.Equal(t, model.APIVersion, s.Views["stack"].Subtitle)
			assert.Equal(t, model.ArchitectureVersion, s.Views["stack"].APIVersion)
		})
	}
}

func TestLoadSystem_RejectsMixedVersionsAndLegacyEvidence(t *testing.T) {
	d := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(d, "views"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(d, "model.yaml"), []byte("apiVersion: noodle/v1alpha2\nkind: Model\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(d, "views", "v.yaml"), []byte("apiVersion: noodle/v1alpha1\nkind: View\ntype: topology\n"), 0644))
	_, err := model.LoadSystem(d)
	assert.ErrorContains(t, err, "differs from model apiVersion")
}

func TestUpgradeDirectory_PreservesYAMLAndNeverOverwrites(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.CopyFS(src, os.DirFS("testdata/minimal")))
	viewPath := filepath.Join(src, "views", "request-path.yaml")
	viewRaw, err := os.ReadFile(viewPath)
	require.NoError(t, err)
	viewRaw = []byte(strings.Replace(string(viewRaw), `subtitle: "How a request reaches the database."`, `subtitle: 'noodle/v1alpha1'`, 1) + "# noodle/v1alpha1 in a comment\n")
	require.NoError(t, os.WriteFile(viewPath, viewRaw, 0644))
	dst := filepath.Join(t.TempDir(), "upgraded")
	require.NoError(t, model.UpgradeDirectory(src, dst))
	loaded, err := model.LoadSystem(dst)
	require.NoError(t, err)
	assert.Equal(t, model.ArchitectureVersion, loaded.Model.APIVersion)
	assert.Empty(t, model.CheckArchitecture(loaded))
	raw, err := os.ReadFile(filepath.Join(dst, "views", "request-path.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "noodle/v1alpha1")
	assert.Contains(t, string(raw), "# noodle/v1alpha1 in a comment")
	assert.Error(t, model.UpgradeDirectory(src, dst))
	empty := filepath.Join(t.TempDir(), "exists")
	require.NoError(t, os.Mkdir(empty, 0755))
	assert.Error(t, model.UpgradeDirectory(src, empty))
}
