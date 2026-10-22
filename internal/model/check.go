package model

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Finding names the file and the id at fault, as FR-005 requires.
type Finding struct {
	File, ID, Msg string
}

func (f Finding) String() string { return fmt.Sprintf("%s: %s: %s", f.File, f.ID, f.Msg) }

var (
	elementKinds    = set("frontend", "backend", "database", "cloud", "security", "bus", "external", "tool", "region", "group")
	connectionKinds = set("flow", "auth", "tunnel", "async")
	statuses        = set("", "planned", "deprecated")
	shapes          = set("", "box", "cylinder", "actor")
	viewTypes       = set("topology", "sequence", "landscape", "catalog", "matrix")
	// computedViews take no layout file: their geometry follows from the view alone.
	computedViews = set("sequence", "landscape", "catalog", "matrix")
	levels        = set("read", "write", "admin", "breakglass")
	authMethods   = set("static-token", "access-key", "password", "oidc", "k8s-sa", "federated")
	matrixStates  = set("", "current", "target", "diff")
	sides         = set("left", "right", "top", "bottom")
)

func set(vs ...string) map[string]bool {
	m := map[string]bool{}
	for _, v := range vs {
		m[v] = true
	}
	return m
}

type checker struct {
	s            *System
	architecture bool
	findings     []Finding
	elements     map[string]Element
	conns        map[string]Connection
	edgeIDs      map[string]bool
	offerings    map[string]bool
	ends         map[string][2]string
}

func (c *checker) errf(file, id, format string, args ...any) {
	c.findings = append(c.findings, Finding{file, id, fmt.Sprintf(format, args...)})
}

// Check reports every broken reference and invalid value across the model, its views
// and their layouts. An empty result means the system can be resolved.
func Check(s *System) []Finding { return checkWith(s, false) }

// CheckArchitecture performs semantic checks for v1alpha2 architecture systems
// (ADR-0021): coordinate-free facts, evidence and version consistency.
func CheckArchitecture(s *System) []Finding { return checkWith(s, true) }

func checkWith(s *System, arch bool) []Finding {
	c := &checker{s: s, architecture: arch, elements: map[string]Element{}, conns: map[string]Connection{}, edgeIDs: map[string]bool{}, offerings: map[string]bool{}, ends: map[string][2]string{}}
	if s == nil || s.Model == nil {
		return []Finding{{File: "model.yaml", ID: "(empty)", Msg: "missing model"}}
	}
	if s.Model.APIVersion != APIVersion && s.Model.APIVersion != ArchitectureVersion {
		c.errf("model.yaml", "(version)", "unsupported apiVersion %q", s.Model.APIVersion)
	}
	for _, v := range s.Views {
		if v.APIVersion != s.Model.APIVersion {
			c.errf(filepath.Join("views", v.ID+".yaml"), v.ID, "apiVersion differs from model")
		}
	}
	for _, l := range s.Layouts {
		if l.APIVersion != s.Model.APIVersion {
			c.errf(filepath.Join("layouts", l.ID+".yaml"), l.ID, "apiVersion differs from model")
		}
	}
	c.checkModel()
	for _, id := range s.ViewIDs() {
		c.checkView(s.Views[id])
	}
	for _, id := range sortedKeys(s.Layouts) {
		if !(arch && s.Model.APIVersion == ArchitectureVersion) {
			c.checkLayout(s.Layouts[id])
		}
	}
	sort.SliceStable(c.findings, func(i, j int) bool {
		if c.findings[i].File != c.findings[j].File {
			return c.findings[i].File < c.findings[j].File
		}
		return c.findings[i].ID < c.findings[j].ID
	})
	return c.findings
}

func (c *checker) checkModel() {
	const file = "model.yaml"
	seen := map[string]bool{}
	unique := func(id string) {
		if id == "" {
			c.errf(file, "(empty)", "missing id")
		} else if seen[id] {
			c.errf(file, id, "duplicate id")
		}
		seen[id] = true
	}
	for _, e := range c.s.Model.Elements {
		unique(e.ID)
		c.elements[e.ID] = e
	}
	for _, e := range c.s.Model.Elements {
		if !elementKinds[e.Kind] {
			c.errf(file, e.ID, "unknown element kind %q", e.Kind)
		}
		if !statuses[e.Status] {
			c.errf(file, e.ID, "unknown status %q, want planned or deprecated", e.Status)
		}
		if status, _ := c.s.Status(e.ID); e.Target != "" && status != "planned" {
			c.errf(file, e.ID, "target is for planned elements only (ADR-0014)")
		}
		if !shapes[e.Shape] {
			c.errf(file, e.ID, "unknown shape %q", e.Shape)
		}
		if e.Parent != "" {
			if p, ok := c.elements[e.Parent]; !ok {
				c.errf(file, e.ID, "parent %q does not exist", e.Parent)
			} else if !p.IsZone() {
				c.errf(file, e.ID, "parent %q is not a zone", e.Parent)
			}
		}
		ports := map[string]bool{}
		for _, p := range e.Ports {
			if p.Name == "" || ports[p.Name] {
				c.errf(file, e.ID, "duplicate or empty port name %q", p.Name)
			}
			ports[p.Name] = true
			if p.Number < 1 || p.Number > 65535 {
				c.errf(file, e.ID, "port %q number %d is outside 1..65535", p.Name, p.Number)
			}
			if p.Protocol != "TCP" && p.Protocol != "UDP" && p.Protocol != "SCTP" {
				c.errf(file, e.ID, "port %q has invalid transport %q", p.Name, p.Protocol)
			}
		}
	}
	// Parent pointers must form a forest, not merely point at existing zones.
	parents := map[string]string{}
	for _, e := range c.s.Model.Elements {
		parents[e.ID] = e.Parent
	}
	state := map[string]int{}
	var visitParent func(string)
	visitParent = func(id string) {
		if state[id] == 1 {
			c.errf(file, id, "parent cycle")
			return
		}
		if state[id] == 2 {
			return
		}
		state[id] = 1
		if p := parents[id]; p != "" {
			visitParent(p)
		}
		state[id] = 2
	}
	for id := range parents {
		visitParent(id)
	}
	c.checkMatches(file)
	for _, cn := range c.s.Model.Connections {
		unique(cn.ID)
		c.conns[cn.ID] = cn
		c.edgeIDs[cn.ID] = true
		c.ends[cn.ID] = [2]string{cn.From, cn.To}
		if !connectionKinds[cn.Kind] {
			c.errf(file, cn.ID, "unknown connection kind %q", cn.Kind)
		}
		if len(cn.EnforcedBy) > 0 && !cn.Denied {
			c.errf(file, cn.ID, "enforced_by is for denied connections only")
		}
		for _, id := range cn.EnforcedBy {
			c.mustElement(file, cn.ID, "enforced_by", id)
		}
		c.mustElement(file, cn.ID, "from", cn.From)
		if to, ok := c.mustElement(file, cn.ID, "to", cn.To); ok && cn.Port != "" && !hasPort(to, cn.Port) {
			c.errf(file, cn.ID, "port %q is not a port of %s", cn.Port, cn.To)
		}
	}
	for _, r := range c.s.Model.References {
		unique(r.ID)
		c.edgeIDs[r.ID] = true
		c.ends[r.ID] = [2]string{r.From, r.To}
		c.mustElement(file, r.ID, "from", r.From)
		c.mustElement(file, r.ID, "to", r.To)
	}
	for _, o := range c.s.Model.Offerings {
		unique(o.ID)
		c.offerings[o.ID] = true
		if !statuses[o.Status] {
			c.errf(file, o.ID, "unknown status %q, want planned or deprecated", o.Status)
		}
		if o.Target != "" && o.Status != "planned" {
			c.errf(file, o.ID, "target is for planned offerings only (ADR-0014)")
		}
		for _, id := range o.BackedBy {
			c.mustElement(file, o.ID, "backed_by", id)
		}
	}
	for _, m := range c.s.Model.Memberships {
		unique(m.ID)
	}
	for _, g := range c.s.Model.Grants {
		unique(g.ID)
	}
	c.checkAccess(file)
	for _, a := range c.s.Model.Annotations {
		unique(a.ID)
		for _, t := range a.Targets {
			if _, ok := c.elements[t]; !ok && !c.edgeIDs[t] {
				c.errf(file, a.ID, "target %q does not exist in the model", t)
			}
		}
	}
	if c.s.Model.APIVersion == ArchitectureVersion {
		c.checkEvidence(file)
	}
}

func (c *checker) checkEvidence(file string) {
	want := map[string]bool{}
	for _, e := range c.s.Model.Elements {
		want[e.ID] = true
	}
	for _, e := range c.s.Model.Connections {
		want[e.ID] = true
	}
	for _, e := range c.s.Model.References {
		want[e.ID] = true
	}
	for id := range want {
		ev, ok := c.s.Model.Evidence[id]
		if !ok {
			c.errf(file, id, "missing evidence")
			continue
		}
		switch ev.How {
		case "code", "config", "inferred", "docs":
			if len(ev.Sources) == 0 {
				c.errf(file, id, "evidence %q needs at least one source", ev.How)
			}
		case "unknown":
			if strings.TrimSpace(ev.Note) == "" {
				c.errf(file, id, "unknown evidence needs a note")
			}
		default:
			c.errf(file, id, "unknown evidence how %q", ev.How)
		}
		for _, src := range ev.Sources {
			if (src.File == "") == (src.Object == "") {
				c.errf(file, id, "evidence source needs exactly one of file or object")
			}
			if src.Line != nil && (*src.Line < 0 || src.File == "") {
				c.errf(file, id, "evidence line must be nonnegative and only accompany file")
			}
		}
	}
	for id := range c.s.Model.Evidence {
		if !want[id] {
			c.errf(file, id, "orphan evidence: no element, connection or reference")
		}
	}
}

// checkMatches keeps each discovered id on one curated element at most, so a merge
// never has to choose between two.
func (c *checker) checkMatches(file string) {
	matchedBy := map[string]string{}
	for _, e := range c.s.Model.Elements {
		for _, id := range e.Matches {
			switch owner, taken := matchedBy[id]; {
			case !IsDiscoveredID(id):
				c.errf(file, e.ID, "matches %q is not a discovered id <adapter>:<path>", id)
			case taken && owner == e.ID:
				c.errf(file, e.ID, "matches %q twice", id)
			case taken:
				c.errf(file, e.ID, "matches %q, already matched by %s", id, owner)
			default:
				matchedBy[id] = e.ID
			}
		}
	}
}

func (c *checker) mustElement(file, id, role, ref string) (Element, bool) {
	e, ok := c.elements[ref]
	if !ok {
		c.errf(file, id, "%s %q does not exist in the model", role, ref)
	}
	return e, ok
}

func hasPort(e Element, name string) bool {
	for _, p := range e.Ports {
		if p.Name == name {
			return true
		}
	}
	return false
}

func (c *checker) checkView(v *View) {
	file := filepath.Join("views", v.ID+".yaml")
	if !viewTypes[v.Type] {
		c.errf(file, v.ID, "type %q, want topology, sequence, landscape, catalog or matrix", v.Type)
	}
	for _, inc := range v.Include {
		if v.Type == "catalog" {
			if !c.offerings[inc] {
				c.errf(file, inc, "include is not an offering of the model")
			}
			continue
		}
		c.mustElement(file, v.ID, "include", strings.TrimSuffix(inc, "/**"))
	}
	if v.Columns != 0 && (v.Type != "catalog" || v.Columns < 1 || v.Columns > 4) {
		c.errf(file, v.ID, "columns is for catalog views, from 1 to 4")
	}
	if v.Type == "sequence" {
		c.checkSequence(file, v)
	} else {
		included := map[string]bool{}
		for _, id := range c.s.Included(v) {
			included[id] = true
		}
		seen := map[string]bool{}
		for _, st := range v.Steps {
			if st.Connection == "" {
				c.errf(file, v.ID, "a topology step is a connection id, not a message (ADR-0011)")
			} else if _, ok := c.conns[st.Connection]; !ok {
				c.errf(file, st.Connection, "step is not a connection of the model")
			} else {
				if seen[st.Connection] {
					c.errf(file, st.Connection, "repeated topology step")
				}
				seen[st.Connection] = true
				cn := c.conns[st.Connection]
				if !included[cn.From] || !included[cn.To] {
					c.errf(file, st.Connection, "step is hidden because an endpoint is not included")
				}
			}
		}
	}
	backgroundSeen := map[string]bool{}
	for _, id := range v.Background {
		cn, ok := c.conns[id]
		if !ok {
			c.errf(file, id, "step is not a connection of the model")
		} else if v.Type != "sequence" {
			for _, step := range v.Steps {
				if step.Connection == id {
					c.errf(file, id, "connection cannot be both a numbered step and background work")
				}
			}
			if backgroundSeen[id] {
				c.errf(file, id, "repeated background connection")
			}
			backgroundSeen[id] = true
			included := map[string]bool{}
			for _, element := range c.s.Included(v) {
				included[element] = true
			}
			if !included[cn.From] || !included[cn.To] {
				c.errf(file, id, "background connection is hidden because an endpoint is not included")
			}
		}
	}
	for _, id := range sortedKeys(v.Labels) {
		if v.Type == "landscape" || v.Type == "matrix" {
			if _, ok := c.elements[id]; !ok {
				c.errf(file, id, "label for an unknown element")
			}
		} else if !c.edgeIDs[id] {
			c.errf(file, id, "label for an unknown connection or reference")
		}
	}
	c.checkLandscape(file, v)
	c.checkMatrix(file, v)
	if len(v.Participants) > 0 && v.Type != "sequence" {
		c.errf(file, v.ID, "participants are for sequence views only")
	}
	for _, p := range v.Participants {
		c.mustElement(file, v.ID, "participant", p)
	}
	_, hasLayout := c.s.Layouts[v.ID]
	if v.Type == "topology" && !hasLayout && !(c.architecture && c.s.Model.APIVersion == ArchitectureVersion) {
		c.errf(file, v.ID, "topology view has no layouts/%s.yaml", v.ID)
	}
	if computedViews[v.Type] && hasLayout {
		c.errf(file, v.ID, "%s views are computed and take no layout (ADR-0007, ADR-0012)", v.Type)
	}
}

// checkSequence enforces ADR-0011: each step is exactly one of a message over a model
// connection, a reply to an earlier message, or a note.
func (c *checker) checkSequence(file string, v *View) {
	if len(v.Background) > 0 {
		c.errf(file, v.ID, "background steps are for topology views; a sequence numbers every message")
	}
	if len(v.Cards) > 0 || len(v.Notes) > 0 {
		c.errf(file, v.ID, "cards and notes need a layout; a sequence has none yet")
	}
	participants := map[string]bool{}
	for _, p := range v.Participants {
		participants[p] = true
	}
	messages := map[string]bool{}
	for i, st := range v.Steps {
		where := st.ID
		if where == "" {
			where = fmt.Sprintf("step %d", i+1)
		}
		kinds := 0
		for _, set := range []bool{st.IsMessage(), st.Reply != "", st.Note != "", st.Connection != ""} {
			if set {
				kinds++
			}
		}
		if kinds != 1 {
			c.errf(file, where, "a step is exactly one of a message (from, to, over), a reply or a note")
			continue
		}
		if st.ID != "" {
			if messages[st.ID] {
				c.errf(file, st.ID, "duplicate step id")
			}
			if !st.IsMessage() {
				c.errf(file, st.ID, "only messages take an id")
			}
		}
		used := []string{}
		switch {
		case st.Connection != "":
			c.errf(file, where, "a sequence step is a message, reply or note, not a bare connection id (ADR-0011)")
		case st.Note != "":
			c.mustElement(file, where, "note", st.Note)
			used = append(used, st.Note)
		case st.Reply != "":
			if !messages[st.Reply] {
				c.errf(file, where, "reply to %q, which is not an earlier message", st.Reply)
			}
		default:
			if st.From == "" || st.To == "" || st.Over == "" {
				c.errf(file, where, "a message needs from, to and over")
				continue
			}
			if st.From == st.To {
				c.errf(file, where, "a message to itself is a note")
			}
			cn, ok := c.conns[st.Over]
			if !ok {
				c.errf(file, where, "over %q is not a connection of the model", st.Over)
			} else if !(cn.From == st.From && cn.To == st.To) && !(cn.From == st.To && cn.To == st.From) {
				c.errf(file, where, "%s → %s does not run over %s, which joins %s and %s", st.From, st.To, st.Over, cn.From, cn.To)
			} else if cn.From != st.From && st.Text == "" {
				c.errf(file, where, "a message against %s needs a text: its verb %q describes the other direction", st.Over, cn.Verb)
			}
			c.mustElement(file, where, "from", st.From)
			c.mustElement(file, where, "to", st.To)
			used = append(used, st.From, st.To)
			if st.ID != "" {
				messages[st.ID] = true
			}
		}
		for _, p := range used {
			if len(participants) > 0 && !participants[p] {
				c.errf(file, where, "%s is not in participants", p)
			}
		}
	}
}

// checkLandscape: every item is a component of the model, shown once (ADR-0012).
func (c *checker) checkLandscape(file string, v *View) {
	if v.Type != "landscape" {
		if len(v.Bands) > 0 || len(v.Side) > 0 {
			c.errf(file, v.ID, "bands and side are for landscape views only")
		}
		return
	}
	if len(v.Bands) == 0 {
		c.errf(file, v.ID, "landscape view has no bands")
	}
	seen := map[string]bool{}
	items := func(sections []Section) {
		for _, s := range sections {
			for _, id := range s.Items {
				if e, ok := c.mustElement(file, id, "item", id); ok && e.IsZone() {
					c.errf(file, id, "item is a zone, want a component")
				}
				if seen[id] {
					c.errf(file, id, "item shown twice")
				}
				seen[id] = true
			}
		}
	}
	bandIDs := map[string]bool{}
	for _, b := range v.Bands {
		if b.ID == "" || bandIDs[b.ID] {
			c.errf(file, v.ID, "band %q: missing or duplicate id", b.ID)
		}
		bandIDs[b.ID] = true
		items(b.Sections)
	}
	items(v.Side)
}

func (c *checker) checkLayout(l *Layout) {
	file := filepath.Join("layouts", l.ID+".yaml")
	v, ok := c.s.Views[l.ID]
	if !ok {
		c.errf(file, l.ID, "no view views/%s.yaml for this layout", l.ID)
		return
	}
	for _, id := range sortedKeys(l.Elements) {
		e, ok := c.mustElement(file, id, "element", id)
		if _, placed := l.Elements[e.Parent]; ok && e.Parent != "" && !placed {
			c.errf(file, id, "position is relative to zone %q, which has no position", e.Parent)
		}
	}
	shown := map[string]bool{}
	for _, id := range c.s.Included(v) {
		shown[id] = true
		if _, ok := l.Elements[id]; !ok {
			c.errf(file, id, "included element has no position")
		}
	}
	for _, id := range sortedKeys(c.ends) {
		if e := c.ends[id]; shown[e[0]] && shown[e[1]] {
			if _, ok := l.Edges[id]; !ok {
				c.errf(file, id, "edge between shown elements has no route")
			}
		}
	}
	for _, id := range sortedKeys(l.Edges) {
		r := l.Edges[id]
		if !c.edgeIDs[id] {
			c.errf(file, id, "edge is not a connection or reference of the model")
		}
		for i, end := range []string{r.From, r.To} {
			c.checkEndpoint(file, id, end)
			if want, ok := c.ends[id]; ok {
				if got, _ := Endpoint(end); got != want[i] {
					c.errf(file, id, "endpoint %q must be on %s, the %s of the edge", end, want[i], [2]string{"from", "to"}[i])
				}
			}
		}
		for _, w := range r.Waypoints {
			if w.Lane != "" {
				if _, ok := l.Lanes[w.Lane]; !ok {
					c.errf(file, id, "lane %q is not defined", w.Lane)
				}
			}
		}
	}
	for _, name := range sortedKeys(l.Lanes) {
		if lane := l.Lanes[name]; (lane.X == nil) == (lane.Y == nil) {
			c.errf(file, name, "lane needs exactly one of x or y")
		}
	}
	cards, notes := map[string]bool{}, map[string]bool{}
	for _, cd := range v.Cards {
		cards[cd.ID] = true
	}
	for _, n := range v.Notes {
		notes[n.ID] = true
	}
	for _, id := range sortedKeys(l.Cards) {
		if !cards[id] {
			c.errf(file, id, "card is not defined in the view")
		}
	}
	for _, id := range sortedKeys(l.Notes) {
		if !notes[id] {
			c.errf(file, id, "note is not defined in the view")
		}
	}
	for _, cd := range v.Cards {
		if _, ok := l.Cards[cd.ID]; !ok {
			c.errf(file, cd.ID, "card has no position")
		}
	}
	for _, n := range v.Notes {
		if _, ok := l.Notes[n.ID]; !ok {
			c.errf(file, n.ID, "note has no position")
		}
	}
}

// checkEndpoint validates the element and side of "id.side[@NN%]"; the ratio's range
// is the resolver's concern.
func (c *checker) checkEndpoint(file, edgeID, end string) {
	id, anchor := Endpoint(end)
	c.mustElement(file, edgeID, "endpoint", id)
	side, _, _ := strings.Cut(anchor, "@")
	if !sides[side] {
		c.errf(file, edgeID, "endpoint %q: side %q, want left, right, top or bottom", end, side)
	}
}

// Status returns an element's status and target, inherited from the nearest enclosing
// zone that sets them (ADR-0008, ADR-0014). Parents must exist; Check reports those
// that do not.
func (s *System) Status(id string) (status, target string) {
	byID := make(map[string]Element, len(s.Model.Elements))
	for _, e := range s.Model.Elements {
		byID[e.ID] = e
	}
	for seen := map[string]bool{}; id != "" && !seen[id]; id = byID[id].Parent {
		seen[id] = true
		e := byID[id]
		if status == "" {
			status = e.Status
		}
		if target == "" {
			target = e.Target
		}
		if status != "" && (status != "planned" || target != "") {
			break
		}
	}
	return status, target
}

// Included returns the element ids a view shows, sorted. No include means all.
func (s *System) Included(v *View) []string {
	elements := map[string]bool{}
	children := map[string][]string{}
	for _, e := range s.Model.Elements {
		elements[e.ID] = true
		if e.Parent != "" {
			children[e.Parent] = append(children[e.Parent], e.ID)
		}
	}
	if len(v.Include) == 0 {
		return sortedKeys(elements)
	}
	out := map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		if out[id] {
			return
		}
		out[id] = true
		for _, ch := range children[id] {
			walk(ch)
		}
	}
	for _, inc := range v.Include {
		if root, ok := strings.CutSuffix(inc, "/**"); ok {
			walk(root)
		} else if elements[inc] {
			out[inc] = true
		}
	}
	return sortedKeys(out)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// checkAccess enforces ADR-0019 on memberships and grants: ids exist, levels and auth
// methods are known, a target names the phase of a planned or deprecated item, and no
// membership chain loops.
func (c *checker) checkAccess(file string) {
	status := func(id, st, target string) {
		if !statuses[st] {
			c.errf(file, id, "unknown status %q, want planned or deprecated", st)
		}
		if target != "" && st == "" {
			c.errf(file, id, "target is for planned or deprecated grants and memberships (ADR-0019)")
		}
	}
	auth := func(id string, a *Auth) {
		if a != nil && !authMethods[a.Method] {
			c.errf(file, id, "unknown auth method %q, want static-token, access-key, password, oidc, k8s-sa or federated", a.Method)
		}
	}
	groups := map[string][]string{}
	for _, m := range c.s.Model.Memberships {
		c.mustElement(file, m.ID, "subject", m.Subject)
		c.mustElement(file, m.ID, "group", m.Group)
		if m.Subject == m.Group {
			c.errf(file, m.ID, "subject and group are the same element")
		}
		status(m.ID, m.Status, m.Target)
		auth(m.ID, m.Auth)
		groups[m.Subject] = append(groups[m.Subject], m.Group)
	}
	for _, g := range c.s.Model.Grants {
		c.mustElement(file, g.ID, "subject", g.Subject)
		c.mustElement(file, g.ID, "resource", g.Resource)
		if g.Subject == g.Resource {
			c.errf(file, g.ID, "subject and resource are the same element")
		}
		if !levels[g.Level] {
			c.errf(file, g.ID, "unknown level %q, want read, write, admin or breakglass", g.Level)
		}
		for _, id := range g.Via {
			c.mustElement(file, g.ID, "via", id)
		}
		status(g.ID, g.Status, g.Target)
		auth(g.ID, g.Auth)
	}
	// A cycle is reported once, on the membership that closes it.
	state := map[string]int{}
	var visit func(string) bool
	visit = func(id string) bool {
		state[id] = 1
		for _, g := range groups[id] {
			if state[g] == 1 || (state[g] == 0 && visit(g)) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for _, m := range c.s.Model.Memberships {
		if state[m.Subject] == 0 && visit(m.Subject) {
			c.errf(file, m.ID, "membership cycle through %s", m.Subject)
			return
		}
	}
}

// checkMatrix keeps identities, resources and state on matrix views, every item an
// element of the model, shown once (ADR-0019).
func (c *checker) checkMatrix(file string, v *View) {
	if v.Type != "matrix" {
		if len(v.Identities) > 0 || len(v.Resources) > 0 || v.State != "" {
			c.errf(file, v.ID, "identities, resources and state are for matrix views only")
		}
		return
	}
	if len(v.Identities) == 0 || len(v.Resources) == 0 {
		c.errf(file, v.ID, "a matrix needs identities and resources")
	}
	if !matrixStates[v.State] {
		c.errf(file, v.ID, "state %q, want current, target or diff", v.State)
	}
	for _, groups := range [][]Section{v.Identities, v.Resources} {
		seen := map[string]bool{}
		for _, g := range groups {
			for _, id := range g.Items {
				if seen[id] {
					c.errf(file, id, "shown twice in the same axis")
				}
				seen[id] = true
				c.mustElement(file, id, "item", id)
			}
		}
	}
}
