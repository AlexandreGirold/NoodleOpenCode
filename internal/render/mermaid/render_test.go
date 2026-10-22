package mermaid

import (
	"slices"
	"testing"

	"github.com/TheGostsniperfr/Noodle/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSystem() *model.System {
	s := &model.System{
		Model: &model.Model{APIVersion: model.ArchitectureVersion, Kind: "Model",
			Elements: []model.Element{
				{ID: "zone", Kind: "region", Title: "Production", Tech: "Kubernetes"},
				{ID: "api", Kind: "backend", Parent: "zone", Title: "API", Tech: "Go", Desc: "Serves requests"},
				{ID: "db", Kind: "database", Title: "Database", Shape: "cylinder", Ports: []model.Port{{Name: "sql", Protocol: "TCP", Number: 5432}}},
			},
			Connections: []model.Connection{{ID: "query", From: "api", To: "db", Kind: "flow", Verb: "query", Protocol: "SQL", Port: "sql"}},
			References:  []model.Reference{{ID: "config", From: "api", To: "db", Kind: "envFrom"}},
			Evidence:    map[string]model.Evidence{},
		},
		Views: map[string]*model.View{"runtime": {ID: "runtime", APIVersion: model.ArchitectureVersion, Kind: "View", Type: "topology"}},
	}
	for _, id := range []string{"zone", "api", "db", "query", "config"} {
		s.Model.Evidence[id] = model.Evidence{How: "config", Sources: []model.EvidenceSource{{File: "compose.yaml"}}}
	}
	return s
}

func TestTopologyMeaning(t *testing.T) {
	s := validSystem()
	s.Views["runtime"].Title = "Architecture"
	s.Views["runtime"].Subtitle = "Which services communicate?"
	s.Views["runtime"].Meta = []string{"Logical roles; environments are separate."}
	s.Model.Evidence["query"] = model.Evidence{How: "inferred", Sources: []model.EvidenceSource{{File: "client.go"}}}
	s.Model.Evidence["db"] = model.Evidence{How: "unknown", Note: "Confirm deployment"}
	s.Views["runtime"].Steps = []model.Step{{Connection: "query"}}
	s.Views["runtime"].Labels = map[string]string{"query": "load records"}
	s.Model.Annotations = []model.Annotation{{ID: "gap", Severity: "medium", Title: "Check egress", Text: "Connectivity not verified", Targets: []string{"query"}}}
	text, err := Render(s, "runtime")
	require.NoError(t, err)
	for _, want := range []string{
		`context["Architecture<br/>Which services communicate?<br/>Logical roles; environments are separate."]`,
		`subgraph n_7a6f6e65["Production<br/>Kubernetes"]`,
		`n_617069["API<br/>Go<br/>Serves requests"]`,
		`n_6462[("Database [unknown]")]`,
		`n_617069 -->|"[1] load records · SQL · flow · TCP 5432 [inferred]"| n_6462`,
		`n_617069 -.->|"reference: envFrom"| n_6462`,
		`note_676170["Check egress [medium]<br/>Connectivity not verified`,
		`%% source query: client.go`,
		`legend[`,
	} {
		assert.Contains(t, text, want)
	}
}

func TestDenialPreservesConnectionDetails(t *testing.T) {
	for _, tc := range []struct {
		name     string
		enforced []string
		want     string
	}{
		{"intent", nil, "DENIED intent only"},
		{"enforced", []string{"zone"}, "DENIED by zone"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := validSystem()
			s.Model.Connections[0].Denied, s.Model.Connections[0].EnforcedBy = true, tc.enforced
			text, err := Render(s, "runtime")
			require.NoError(t, err)
			assert.Contains(t, text, "query · SQL · flow · TCP 5432 · "+tc.want)
		})
	}
}

func TestEscapingAndIdentity(t *testing.T) {
	for _, text := range []string{`end`, `A "quote" | pipe`, "<script>run()</script>\n# back`tick\\", "a\r\nb"} {
		t.Run(text, func(t *testing.T) {
			got := label(text)
			assert.NotContains(t, got, "<script>")
			assert.NotContains(t, got, "\n")
			assert.NotContains(t, got, "\r")
			assert.True(t, len(got) >= 2 && got[0] == '"' && got[len(got)-1] == '"')
		})
	}
	assert.NotEqual(t, idOf("a-b"), idOf("ab"))
	assert.NotEqual(t, idOf("end"), "end")
	assert.Equal(t, "AA", letters(26))
}

func TestSelectionAndOrder(t *testing.T) {
	a, b := validSystem(), validSystem()
	slices.Reverse(b.Model.Elements)
	x, err := Render(a, "runtime")
	require.NoError(t, err)
	y, err := Render(b, "runtime")
	require.NoError(t, err)
	assert.Equal(t, x, y)
	a.Views["runtime"].Include = []string{"api"}
	x, err = Render(a, "runtime")
	require.NoError(t, err)
	assert.Contains(t, x, "n_617069[")
	assert.NotContains(t, x, "subgraph n_7a6f6e65")
	assert.NotContains(t, x, "n_6462[(")
	a.Views["runtime"].Include = []string{"zone"}
	x, err = Render(a, "runtime")
	require.NoError(t, err)
	assert.Contains(t, x, "subgraph n_7a6f6e65")
	assert.NotContains(t, x, "n_617069[")
}

func TestLandscapeProjectsSharedModel(t *testing.T) {
	s := validSystem()
	s.Views["stack"] = &model.View{ID: "stack", APIVersion: model.ArchitectureVersion, Kind: "View", Type: "landscape",
		Bands: []model.Band{{ID: "runtime", Title: "Runtime", Flow: true, Sections: []model.Section{
			{Title: "Services", Items: []string{"api"}}, {Title: "Data", Items: []string{"db"}},
		}}}, Labels: map[string]string{"api": "Service"},
	}
	text, err := Render(s, "stack")
	require.NoError(t, err)
	assert.Contains(t, text, `Service<br/>Go`)
	assert.Contains(t, text, `subgraph band_0_section_0["Services"]`)
	assert.Contains(t, text, "reading order, not traffic")
	assert.NotContains(t, text, `reference: envFrom`)
	assert.NotContains(t, text, "TCP 5432")
}

func TestLifecycleAndEvidenceVisibility(t *testing.T) {
	s := validSystem()
	s.Model.Elements[0].Status, s.Model.Elements[0].Target = "planned", "Q4"
	s.Model.Evidence["api"] = model.Evidence{How: "docs", Sources: []model.EvidenceSource{{File: "design.md"}}}
	text, err := Render(s, "runtime")
	require.NoError(t, err)
	assert.Contains(t, text, `API [docs]<br/>Go<br/>Serves requests<br/>planned · Q4`)
	assert.Contains(t, text, `Production<br/>Kubernetes<br/>planned · Q4`)
	s.Model.Connections[0].Port = ""
	text, err = Render(s, "runtime")
	require.NoError(t, err)
	assert.NotContains(t, text, "TCP 5432")
}

func TestRejectsUnsupportedAndInvalid(t *testing.T) {
	for _, typ := range []string{"sequence", "catalog", "matrix"} {
		t.Run(typ, func(t *testing.T) {
			s := validSystem()
			s.Views["runtime"].Type = typ
			_, err := Render(s, "runtime")
			assert.ErrorContains(t, err, "not supported")
		})
	}
	s := validSystem()
	s.Views["runtime"].Cards = []model.Card{{ID: "card", Title: "Unsupported"}}
	_, err := Render(s, "runtime")
	assert.ErrorContains(t, err, "unsupported cards or notes")
	_, err = Render(nil, "runtime")
	assert.Error(t, err)
	s = validSystem()
	s.Model.Connections[0].To = "absent"
	_, err = Render(s, "runtime")
	assert.ErrorContains(t, err, "invalid architecture")
}
