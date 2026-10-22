package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheGostsniperfr/Noodle/internal/model"
)

func TestCheck_ReportsNothing_OnEveryExample(t *testing.T) {
	t.Parallel()
	models, err := filepath.Glob("../../examples/*/model.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, models)

	for _, m := range models {
		t.Run(filepath.Dir(m), func(t *testing.T) {
			s, err := model.LoadSystem(filepath.Dir(m))
			require.NoError(t, err)

			findings := model.Check(s)
			if s.Model.APIVersion == model.ArchitectureVersion {
				findings = model.CheckArchitecture(s)
			}

			assert.Empty(t, findings)
		})
	}
}

func TestCheck_ReportsNothing_OnAValidSystem(t *testing.T) {
	t.Parallel()
	s, err := model.LoadSystem("testdata/minimal")
	require.NoError(t, err)

	findings := model.Check(s)

	assert.Empty(t, findings)
}

const (
	modelHead  = "apiVersion: noodle/v1alpha1\nkind: Model\n"
	viewHead   = "apiVersion: noodle/v1alpha1\nkind: View\n"
	layoutHead = "apiVersion: noodle/v1alpha1\nkind: Layout\n"
	baseModel  = modelHead + `elements:
  - {id: z, kind: region, title: Z}
  - {id: a, kind: backend, parent: z, title: A}
  - {id: b, kind: database, parent: z, title: B, ports: [{name: pg, protocol: TCP, port: 5432}]}
connections:
  - {id: c-ab, from: a, to: b, port: pg, protocol: SQL, verb: query, kind: flow}
`
	baseView   = viewHead + "type: topology\nsteps: [c-ab]\ncards: [{id: card, title: C}]\n"
	baseLayout = layoutHead + `canvas: {width: 800, height: 400}
elements:
  z: {x: 0, y: 0, w: 600, h: 300}
  a: {x: 20, y: 40, w: 200, h: 80}
  b: {x: 300, y: 40, w: 200, h: 80}
cards:
  card: {x: 0, y: 320, w: 600, h: 60}
edges:
  c-ab: {from: a.right, to: b.left}
`
)

// system writes a model, one view "v" and, when layout is non-empty, its layout.
func system(t *testing.T, modelYAML, viewYAML, layoutYAML string) *model.System {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write("model.yaml", modelYAML)
	write("views/v.yaml", viewYAML)
	if layoutYAML != "" {
		write("layouts/v.yaml", layoutYAML)
	}
	s, err := model.LoadSystem(dir)
	require.NoError(t, err)
	return s
}

func TestCheck_ReportsFileIDAndReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		model, view, layout string
		wantFile, wantID    string
		wantMsg             string
	}{
		{"the base system is clean, so each case below fails for one reason only",
			baseModel, baseView, baseLayout, "", "", ""},
		{"duplicate id", baseModel + "  - {id: a, from: a, to: b, kind: flow}\n", baseView, baseLayout,
			"model.yaml", "a", "duplicate id"},
		{"unknown element kind", modelHead + "elements: [{id: x, kind: server}]\n", viewHead + "type: sequence\n", "",
			"model.yaml", "x", `unknown element kind "server"`},
		{"unknown status", modelHead + "elements: [{id: x, kind: backend, status: gone}]\n", viewHead + "type: sequence\n", "",
			"model.yaml", "x", `unknown status "gone"`},
		{"target on a live element", modelHead + "elements: [{id: x, kind: backend, target: SP1}]\n", viewHead + "type: sequence\n", "",
			"model.yaml", "x", "target is for planned elements only"},
		{"target on a child of a planned zone is valid", modelHead + "elements: [{id: z, kind: group, status: planned}, {id: x, kind: bus, parent: z, target: SP1}]\n", viewHead + "type: sequence\n", "",
			"", "", ""},
		{"parent that is not a zone", modelHead + "elements: [{id: x, kind: backend}, {id: y, kind: backend, parent: x}]\n", viewHead + "type: sequence\n", "",
			"model.yaml", "y", `parent "x" is not a zone`},
		{"connection to a missing element", baseModel + "  - {id: c-ax, from: a, to: x, kind: flow}\n", baseView, baseLayout,
			"model.yaml", "c-ax", `to "x" does not exist in the model`},
		{"port the target does not listen on", baseModel + "  - {id: c-ba, from: b, to: a, port: pg, kind: flow}\n", baseView, baseLayout,
			"model.yaml", "c-ba", `port "pg" is not a port of a`},
		{"blocked is no longer a connection kind", baseModel + "  - {id: c-ba, from: b, to: a, kind: blocked}\n", baseView, baseLayout,
			"model.yaml", "c-ba", `unknown connection kind "blocked"`},
		{"enforcement by a missing element", baseModel + "  - {id: c-ba, from: b, to: a, kind: flow, denied: true, enforced_by: [np]}\n", baseView, baseLayout,
			"model.yaml", "c-ba", `enforced_by "np" does not exist in the model`},
		{"enforcement on an allowed connection", baseModel + "  - {id: c-ba, from: b, to: a, kind: flow, enforced_by: [z]}\n", baseView, baseLayout,
			"model.yaml", "c-ba", "enforced_by is for denied connections only"},
		{"annotation target missing", baseModel + "annotations: [{id: G1, targets: [x]}]\n", baseView, baseLayout,
			"model.yaml", "G1", `target "x" does not exist in the model`},
		{"unknown view type", baseModel, viewHead + "type: context\n", baseLayout,
			filepath.Join("views", "v.yaml"), "v", `type "context", want topology, sequence, landscape, catalog or matrix`},
		{"landscape item missing from the model", baseModel, viewHead + "type: landscape\nbands: [{id: b1, title: B, sections: [{items: [x]}]}]\n", "",
			filepath.Join("views", "v.yaml"), "x", `item "x" does not exist in the model`},
		{"landscape item that is a zone", baseModel, viewHead + "type: landscape\nbands: [{id: b1, title: B, sections: [{items: [z]}]}]\n", "",
			filepath.Join("views", "v.yaml"), "z", "item is a zone, want a component"},
		{"landscape item shown twice", baseModel, viewHead + "type: landscape\nbands: [{id: b1, title: B, sections: [{items: [a]}]}]\n" + "side: [{title: S, items: [a]}]\n", "",
			filepath.Join("views", "v.yaml"), "a", "item shown twice"},
		{"landscape view with a layout", baseModel, viewHead + "type: landscape\nbands: [{id: b1, title: B, sections: [{items: [a]}]}]\n", baseLayout,
			filepath.Join("views", "v.yaml"), "v", "landscape views are computed and take no layout"},
		{"bands on a topology view", baseModel, baseView + "bands: [{id: b1, title: B, sections: [{items: [a]}]}]\n", baseLayout,
			filepath.Join("views", "v.yaml"), "v", "bands and side are for landscape views only"},
		{"landscape label for a missing element", baseModel, viewHead + "type: landscape\nbands: [{id: b1, title: B, sections: [{items: [a]}]}]\n" + "labels: {x: X}\n", "",
			filepath.Join("views", "v.yaml"), "x", "label for an unknown element"},
		{"a valid landscape with a tool", strings.Replace(baseModel, "connections:", "  - {id: tf, kind: tool, title: Terraform}\nconnections:", 1), viewHead + "type: landscape\nbands: [{id: b1, title: B, sections: [{items: [a, b, tf]}]}]\n", "",
			"", "", ""},
		{"offering backed by a missing element", strings.Replace(baseModel, "connections:", "offerings: [{id: db, title: DB, backed_by: [x]}]\nconnections:", 1), viewHead + "type: catalog\n", "",
			"model.yaml", "db", `backed_by "x" does not exist in the model`},
		{"grant on a missing resource", baseModel + "grants: [{id: g1, subject: a, resource: x, level: read}]\n", baseView, baseLayout,
			"model.yaml", "g1", `resource "x" does not exist in the model`},
		{"unknown grant level", baseModel + "grants: [{id: g1, subject: a, resource: b, level: owner}]\n", baseView, baseLayout,
			"model.yaml", "g1", `unknown level "owner"`},
		{"grant via a missing element", baseModel + "grants: [{id: g1, subject: a, resource: b, level: read, via: [p]}]\n", baseView, baseLayout,
			"model.yaml", "g1", `via "p" does not exist in the model`},
		{"grant target on a live grant", baseModel + "grants: [{id: g1, subject: a, resource: b, level: read, target: P2}]\n", baseView, baseLayout,
			"model.yaml", "g1", "target is for planned or deprecated grants and memberships"},
		{"grant target on a removed grant is its phase", baseModel + "grants: [{id: g1, subject: a, resource: b, level: read, status: deprecated, target: P3}]\n", baseView, baseLayout,
			"", "", ""},
		{"unknown auth method", baseModel + "grants: [{id: g1, subject: a, resource: b, level: read, auth: {method: magic}}]\n", baseView, baseLayout,
			"model.yaml", "g1", `unknown auth method "magic"`},
		{"grant on itself", baseModel + "grants: [{id: g1, subject: a, resource: a, level: read}]\n", baseView, baseLayout,
			"model.yaml", "g1", "subject and resource are the same element"},
		{"membership cycle", baseModel + "memberships: [{id: m1, subject: a, group: b}, {id: m2, subject: b, group: a}]\n", baseView, baseLayout,
			"model.yaml", "m1", "membership cycle through a"},
		{"matrix with a layout", baseModel, viewHead + "type: matrix\nidentities: [{items: [a]}]\nresources: [{items: [b]}]\n", baseLayout,
			filepath.Join("views", "v.yaml"), "v", "matrix views are computed and take no layout"},
		{"matrix without resources", baseModel, viewHead + "type: matrix\nidentities: [{items: [a]}]\n", "",
			filepath.Join("views", "v.yaml"), "v", "a matrix needs identities and resources"},
		{"matrix item missing from the model", baseModel, viewHead + "type: matrix\nidentities: [{items: [a]}]\nresources: [{items: [x]}]\n", "",
			filepath.Join("views", "v.yaml"), "x", `item "x" does not exist in the model`},
		{"matrix item shown twice", baseModel, viewHead + "type: matrix\nidentities: [{items: [a]}, {items: [a]}]\nresources: [{items: [b]}]\n", "",
			filepath.Join("views", "v.yaml"), "a", "shown twice in the same axis"},
		{"matrix state unknown", baseModel, viewHead + "type: matrix\nidentities: [{items: [a]}]\nresources: [{items: [b]}]\nstate: later\n", "",
			filepath.Join("views", "v.yaml"), "v", `state "later", want current, target or diff`},
		{"state on a topology view", baseModel, baseView + "state: diff\n", baseLayout,
			filepath.Join("views", "v.yaml"), "v", "identities, resources and state are for matrix views only"},
		{"a valid access model and matrix", baseModel + "memberships: [{id: m1, subject: a, group: z, auth: {method: oidc, mfa: true}}]\n" +
			"grants: [{id: g1, subject: z, resource: b, level: write, scope: \"project-*\", via: [a], status: planned, target: P2}]\n",
			viewHead + "type: matrix\nstate: diff\nidentities: [{title: People, items: [a]}]\nresources: [{title: Data, sub: plane, color: violet, items: [b]}]\nlabels: {b: B}\n", "",
			"", "", ""},
		{"offering target without planned", strings.Replace(baseModel, "connections:", "offerings: [{id: db, title: DB, backed_by: [b], target: SP1}]\nconnections:", 1), viewHead + "type: catalog\n", "",
			"model.yaml", "db", "target is for planned offerings only"},
		{"catalog include that is not an offering", strings.Replace(baseModel, "connections:", "offerings: [{id: db, title: DB, backed_by: [b]}]\nconnections:", 1), viewHead + "type: catalog\ninclude: [a]\n", "",
			filepath.Join("views", "v.yaml"), "a", "include is not an offering of the model"},
		{"columns on a topology view", baseModel, baseView + "columns: 2\n", baseLayout,
			filepath.Join("views", "v.yaml"), "v", "columns is for catalog views, from 1 to 4"},
		{"a valid catalog", strings.Replace(baseModel, "connections:", "offerings: [{id: db, title: DB, backed_by: [b], status: planned, target: SP1}]\nconnections:", 1), viewHead + "type: catalog\ninclude: [db]\ncolumns: 2\n", "",
			"", "", ""},
		{"matches that is not a discovered id", strings.Replace(baseModel, "title: A}", "title: A, matches: [api]}", 1), baseView, baseLayout,
			"model.yaml", "a", `matches "api" is not a discovered id <adapter>:<path>`},
		{"the same discovered id twice on one element", strings.Replace(baseModel, "title: A}", `title: A, matches: ["k8s:p/deployment/a", "k8s:p/deployment/a"]}`, 1), baseView, baseLayout,
			"model.yaml", "a", `matches "k8s:p/deployment/a" twice`},
		{"one discovered id matched by two elements", strings.Replace(strings.Replace(baseModel, "title: A}", `title: A, matches: ["k8s:p/deployment/a"]}`, 1), "title: B,", `title: B, matches: ["k8s:p/deployment/a"],`, 1), baseView, baseLayout,
			"model.yaml", "b", `matches "k8s:p/deployment/a", already matched by a`},
		{"matches on elements and zones, absent from any fragment, are valid", strings.Replace(strings.Replace(baseModel, "title: Z}", `title: Z, matches: ["k8s:p/namespace/p"]}`, 1), "title: A}", `title: A, matches: ["k8s:p/deployment/a", "k8s:p/service/a"]}`, 1), baseView, baseLayout,
			"", "", ""},
		{"step that is not a connection", baseModel, baseView + "background: [a]\n", baseLayout,
			filepath.Join("views", "v.yaml"), "a", "step is not a connection of the model"},
		{"include of a missing element", baseModel, baseView + "include: [x/**]\n", baseLayout,
			filepath.Join("views", "v.yaml"), "v", `include "x" does not exist in the model`},
		{"participants on a topology view", baseModel, baseView + "participants: [a]\n", baseLayout,
			filepath.Join("views", "v.yaml"), "v", "participants are for sequence views only"},
		{"topology view without layout", baseModel, baseView, "",
			filepath.Join("views", "v.yaml"), "v", "topology view has no layouts/v.yaml"},
		{"sequence view with a layout", baseModel, viewHead + "type: sequence\n", baseLayout,
			filepath.Join("views", "v.yaml"), "v", "sequence views are computed and take no layout"},
		{"included element without position", baseModel, baseView, layoutHead + "elements:\n  z: {x: 0, y: 0, w: 600, h: 300}\n  a: {x: 20, y: 40, w: 200, h: 80}\n",
			filepath.Join("layouts", "v.yaml"), "b", "included element has no position"},
		{"position relative to an unplaced zone", baseModel, baseView + "include: [a, b]\n", layoutHead + "elements:\n  a: {x: 20, y: 40, w: 200, h: 80}\n  b: {x: 300, y: 40, w: 200, h: 80}\n",
			filepath.Join("layouts", "v.yaml"), "a", `position is relative to zone "z", which has no position`},
		{"edge that is not in the model", baseModel, baseView, baseLayout + "  c-zz: {from: a.right, to: b.left}\n",
			filepath.Join("layouts", "v.yaml"), "c-zz", "edge is not a connection or reference of the model"},
		{"endpoint with an unknown side", baseModel, baseView, baseLayout + "  c-ab2: {from: a.east, to: b.left}\n",
			filepath.Join("layouts", "v.yaml"), "c-ab2", `endpoint "a.east": side "east"`},
		{"undefined lane", baseModel, baseView, baseLayout + "  c-ab3: {from: a.right, to: b.left, waypoints: [lane:bus]}\n",
			filepath.Join("layouts", "v.yaml"), "c-ab3", `lane "bus" is not defined`},
		{"lane with both x and y", baseModel, baseView, baseLayout + "lanes: {bus: {x: 1, y: 2}}\n",
			filepath.Join("layouts", "v.yaml"), "bus", "lane needs exactly one of x or y"},
		{"route drawn backwards", baseModel, baseView, layoutHead + "canvas: {width: 800, height: 400}\nelements:\n  z: {x: 0, y: 0, w: 600, h: 300}\n  a: {x: 20, y: 40, w: 200, h: 80}\n  b: {x: 300, y: 40, w: 200, h: 80}\nedges:\n  c-ab: {from: b.left, to: a.right}\n",
			filepath.Join("layouts", "v.yaml"), "c-ab", `endpoint "b.left" must be on a, the from of the edge`},
		{"edge between shown elements without route", baseModel, baseView, layoutHead + "elements:\n  z: {x: 0, y: 0, w: 600, h: 300}\n  a: {x: 20, y: 40, w: 200, h: 80}\n  b: {x: 300, y: 40, w: 200, h: 80}\n",
			filepath.Join("layouts", "v.yaml"), "c-ab", "edge between shown elements has no route"},
		{"card without position", baseModel, strings.Replace(baseView, "{id: card, title: C}", "{id: card, title: C}, {id: card2, title: D}", 1), baseLayout,
			filepath.Join("layouts", "v.yaml"), "card2", "card has no position"},
		{"a message against its connection is fine, a message off it is not", baseModel, viewHead + "type: sequence\nsteps: [{id: m, from: b, to: a, over: c-ab, text: back}, {from: a, to: z, over: c-ab}]\n", "",
			filepath.Join("views", "v.yaml"), "step 2", "a → z does not run over c-ab, which joins a and b"},
		{"a message against its connection without text", baseModel, viewHead + "type: sequence\nsteps: [{from: b, to: a, over: c-ab}]\n", "",
			filepath.Join("views", "v.yaml"), "step 1", `a message against c-ab needs a text: its verb "query" describes the other direction`},
		{"a message to itself", baseModel, viewHead + "type: sequence\nsteps: [{from: a, to: a, over: c-ab}]\n", "",
			filepath.Join("views", "v.yaml"), "step 1", "a message to itself is a note"},
		{"a reply before its message", baseModel, viewHead + "type: sequence\nsteps: [{reply: m}, {id: m, from: a, to: b, over: c-ab}]\n", "",
			filepath.Join("views", "v.yaml"), "step 1", `reply to "m", which is not an earlier message`},
		{"a step that is two things", baseModel, viewHead + "type: sequence\nsteps: [{id: m, from: a, to: b, over: c-ab, note: a}]\n", "",
			filepath.Join("views", "v.yaml"), "m", "a step is exactly one of"},
		{"a participant left out of participants", baseModel, viewHead + "type: sequence\nparticipants: [a]\nsteps: [{from: a, to: b, over: c-ab}]\n", "",
			filepath.Join("views", "v.yaml"), "step 1", "b is not in participants"},
		{"a bare connection id in a sequence", baseModel, viewHead + "type: sequence\nsteps: [c-ab]\n", "",
			filepath.Join("views", "v.yaml"), "step 1", "not a bare connection id"},
		{"card the view does not define", baseModel, baseView, strings.Replace(baseLayout, "cards:\n", "cards:\n  other: {x: 0, y: 0, w: 1, h: 1}\n", 1),
			filepath.Join("layouts", "v.yaml"), "other", "card is not defined in the view"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := system(t, tt.model, tt.view, tt.layout)

			findings := model.Check(s)

			if tt.wantMsg == "" {
				assert.Empty(t, findings)
				return
			}
			var match []model.Finding
			for _, f := range findings {
				if f.File == tt.wantFile && f.ID == tt.wantID && strings.Contains(f.Msg, tt.wantMsg) {
					match = append(match, f)
				}
			}
			assert.NotEmpty(t, match, "want %s: %s: %s…, got %v", tt.wantFile, tt.wantID, tt.wantMsg, findings)
		})
	}
}

func TestStatus_InheritsFromTheNearestZoneThatSetsIt(t *testing.T) {
	t.Parallel()
	s := system(t, modelHead+`elements:
  - {id: obs, kind: region, status: planned, target: SP2}
  - {id: loki, kind: backend, parent: obs}
  - {id: mq, kind: bus, parent: obs, target: SP1}
  - {id: old, kind: backend, parent: obs, status: deprecated}
  - {id: live, kind: backend}
`, viewHead+"type: sequence\n", "")

	tests := []struct{ id, wantStatus, wantTarget string }{
		{"loki", "planned", "SP2"},
		{"mq", "planned", "SP1"},
		{"old", "deprecated", ""},
		{"live", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			t.Parallel()

			status, target := s.Status(tt.id)

			assert.Equal(t, [2]string{tt.wantStatus, tt.wantTarget}, [2]string{status, target})
		})
	}
}
