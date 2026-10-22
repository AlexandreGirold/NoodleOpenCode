// Package mermaid generates architecture views directly from model contracts.
// Mermaid owns geometry; connection meaning never depends on automatic placement.
package mermaid

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/TheGostsniperfr/Noodle/internal/model"
)

func Render(s *model.System, viewID string) (string, error) {
	if s == nil || s.Model == nil {
		return "", fmt.Errorf("mermaid: missing model")
	}
	v := s.Views[viewID]
	if v == nil {
		return "", fmt.Errorf("mermaid: unknown view %q", viewID)
	}
	if v.Type != "topology" && v.Type != "landscape" {
		return "", fmt.Errorf("mermaid: view type %q is not supported; use topology or landscape", v.Type)
	}
	if len(v.Cards)+len(v.Notes) > 0 {
		return "", fmt.Errorf("mermaid: view %q has unsupported cards or notes; use model annotations or the adjacent guide", viewID)
	}
	if findings := model.CheckArchitecture(s); len(findings) > 0 {
		return "", fmt.Errorf("mermaid: invalid architecture: %s", findings[0])
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%%%% %s\n%%%% %s\n", comment(v.Title), comment(v.Subtitle))
	for _, meta := range v.Meta {
		fmt.Fprintf(&b, "%%%% %s\n", comment(meta))
	}
	if v.Type == "landscape" {
		// Capability groups read as columns; LR would stack disconnected bands in
		// a single narrow column and shrink the exported technology map excessively.
		b.WriteString("flowchart TB\n")
	} else {
		b.WriteString("flowchart LR\n")
	}
	context := v.Title
	if v.Subtitle != "" {
		context += "\n" + v.Subtitle
	}
	for _, meta := range v.Meta {
		context += "\n" + meta
	}
	if strings.TrimSpace(context) != "" {
		fmt.Fprintf(&b, "  context[%s]\n", label(wrap(context, 65)))
	}
	elements := map[string]model.Element{}
	for _, e := range s.Model.Elements {
		e.Status, e.Target = s.Status(e.ID)
		elements[e.ID] = e
	}
	shown := map[string]bool{}
	if v.Type == "topology" {
		renderTopology(&b, s, v, elements, shown)
	} else {
		if len(v.Include)+len(v.Steps)+len(v.Background) > 0 {
			return "", fmt.Errorf("mermaid: landscape include, steps and background are not supported; select band/section items")
		}
		renderLandscape(&b, s, v, elements, shown)
	}
	notes := renderAnnotations(&b, s.Model, shown)
	b.WriteString("  legend[\"Solid: client opens connection to listener<br/>Dotted: reference or reading order, not traffic<br/>[inferred], [docs], [unknown], [unverified]: evidence status<br/>No port label: listening port not recorded\"]\n")
	b.WriteString("  classDef note fill:#fef3c7,stroke:#b45309,color:#451a03\n")
	b.WriteString("  class legend,context note\n")
	if v.Type == "landscape" {
		// Invisible links constrain presentation order only. They never represent
		// model relationships: bands remain rows instead of one unreadably wide strip.
		ranks := []string{}
		if strings.TrimSpace(context) != "" {
			ranks = append(ranks, "context")
		}
		for i := range v.Bands {
			ranks = append(ranks, fmt.Sprintf("band_%d", i))
		}
		for i := range v.Side {
			ranks = append(ranks, fmt.Sprintf("side_%d", i))
		}
		ranks = append(ranks, notes...)
		ranks = append(ranks, "legend")
		b.WriteString("  %% Invisible links below arrange rows; they are not architecture relationships.\n")
		for i := 1; i < len(ranks); i++ {
			fmt.Fprintf(&b, "  %s ~~~ %s\n", ranks[i-1], ranks[i])
		}
	}
	return b.String(), nil
}

// Hex encoding is injective, avoids reserved words, and cannot collide with the
// separate namespaces used by generated groups, annotations and the legend.
func idOf(id string) string { return "n_" + hex.EncodeToString([]byte(id)) }

func label(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '\n':
			b.WriteString("<br/>")
		case r == '\r':
			// CRLF is one line break, not two.
		case strings.ContainsRune("&<>\"#|`\\", r) || r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, "#%d;", r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func comment(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ", "%", "percent").Replace(s)
}

func evidenceTag(m *model.Model, id string) string {
	how := m.Evidence[id].How
	switch how {
	case "code", "config":
		return ""
	case "inferred", "docs", "unknown":
		return " [" + how + "]"
	default:
		return " [unverified]"
	}
}

func nodeLabel(m *model.Model, e model.Element, override string) string {
	name := e.Title
	if name == "" {
		name = e.ID
	}
	if override != "" {
		name = override
	}
	name += evidenceTag(m, e.ID)
	for _, detail := range []string{e.Tech, e.Desc, e.Sub, e.Multiplicity} {
		if detail != "" {
			name += "\n" + detail
		}
	}
	if e.Status != "" {
		name += "\n" + e.Status
		if e.Target != "" {
			name += " · " + e.Target
		}
	}
	return name
}

func writeNode(b *strings.Builder, m *model.Model, e model.Element, override, indent string) {
	open, close := "[", "]"
	switch e.Shape {
	case "cylinder":
		open, close = "[(", ")]"
	case "actor":
		open, close = "((", "))"
	}
	fmt.Fprintf(b, "%s%s%s%s%s\n", indent, idOf(e.ID), open, label(nodeLabel(m, e, override)), close)
	writeEvidence(b, m, e.ID, indent)
}

func renderTopology(b *strings.Builder, s *model.System, v *model.View, elements map[string]model.Element, shown map[string]bool) {
	for _, id := range s.Included(v) {
		shown[id] = true
	}
	children := map[string][]string{}
	for _, id := range sortedKeys(shown) {
		parent := elements[id].Parent
		for parent != "" && !shown[parent] {
			parent = elements[parent].Parent
		}
		children[parent] = append(children[parent], id)
	}
	var emit func(string, int)
	emit = func(parent string, depth int) {
		indent := strings.Repeat("  ", depth)
		for _, id := range children[parent] {
			e := elements[id]
			if e.IsZone() {
				fmt.Fprintf(b, "%ssubgraph %s[%s]\n", indent, idOf(id), label(nodeLabel(s.Model, e, "")))
				writeEvidence(b, s.Model, id, indent+"  ")
				emit(id, depth+1)
				fmt.Fprintf(b, "%send\n", indent)
			} else {
				writeNode(b, s.Model, e, "", indent)
			}
		}
	}
	emit("", 1)
	steps := map[string]string{}
	for i, step := range v.Steps {
		steps[step.Connection] = fmt.Sprintf("[%d] ", i+1)
	}
	for i, id := range v.Background {
		steps[id] = fmt.Sprintf("[%s] ", letters(i))
	}
	connections := append([]model.Connection(nil), s.Model.Connections...)
	sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })
	for _, c := range connections {
		if !shown[c.From] || !shown[c.To] {
			continue
		}
		parts := []string{}
		verb := c.Verb
		if override := v.Labels[c.ID]; override != "" {
			verb = override
		}
		for _, p := range []string{verb, c.Protocol, c.Kind} {
			if p != "" {
				parts = append(parts, p)
			}
		}
		if c.Port != "" {
			for _, port := range elements[c.To].Ports {
				if port.Name == c.Port {
					parts = append(parts, fmt.Sprintf("%s %d", port.Protocol, port.Number))
				}
			}
		}
		if c.Denied {
			denial := "DENIED intent only"
			if len(c.EnforcedBy) > 0 {
				by := append([]string(nil), c.EnforcedBy...)
				sort.Strings(by)
				denial = "DENIED by " + strings.Join(by, ", ")
			}
			parts = append(parts, denial)
		}
		text := steps[c.ID] + strings.Join(parts, " · ") + evidenceTag(s.Model, c.ID)
		fmt.Fprintf(b, "  %s -->|%s| %s\n", idOf(c.From), label(text), idOf(c.To))
		writeEvidence(b, s.Model, c.ID, "  ")
		shown[c.ID] = true
	}
	references := append([]model.Reference(nil), s.Model.References...)
	sort.Slice(references, func(i, j int) bool { return references[i].ID < references[j].ID })
	edgeIndex := 0
	for _, c := range connections {
		if shown[c.ID] {
			edgeIndex++
		}
	}
	for _, r := range references {
		if !shown[r.From] || !shown[r.To] {
			continue
		}
		text := "reference: " + r.Kind
		if override := v.Labels[r.ID]; override != "" {
			text += " · " + override
		}
		fmt.Fprintf(b, "  %s -.->|%s| %s\n", idOf(r.From), label(text+evidenceTag(s.Model, r.ID)), idOf(r.To))
		fmt.Fprintf(b, "  linkStyle %d stroke-dasharray:3 3\n", edgeIndex)
		edgeIndex++
		writeEvidence(b, s.Model, r.ID, "  ")
		shown[r.ID] = true
	}
}

func letters(index int) string {
	result := ""
	for index++; index > 0; index = (index - 1) / 26 {
		result = string(rune('A'+(index-1)%26)) + result
	}
	return result
}

func renderLandscape(b *strings.Builder, s *model.System, v *model.View, elements map[string]model.Element, shown map[string]bool) {
	section := func(sec model.Section, id string, depth int) {
		indent := strings.Repeat("  ", depth)
		text := sec.Title
		if sec.Sub != "" {
			text += "\n" + sec.Sub
		}
		fmt.Fprintf(b, "%ssubgraph %s[%s]\n", indent, id, label(text))
		items := append([]string(nil), sec.Items...)
		sort.Strings(items)
		for _, item := range items {
			shown[item] = true
			writeNode(b, s.Model, elements[item], v.Labels[item], indent+"  ")
		}
		fmt.Fprintf(b, "%send\n", indent)
	}
	for bi, band := range v.Bands {
		id := fmt.Sprintf("band_%d", bi)
		text := band.Title
		if band.Sub != "" {
			text += "\n" + band.Sub
		}
		fmt.Fprintf(b, "  subgraph %s[%s]\n", id, label(text))
		for si, sec := range band.Sections {
			section(sec, fmt.Sprintf("%s_section_%d", id, si), 2)
		}
		if band.Flow {
			for si := 0; si+1 < len(band.Sections); si++ {
				fmt.Fprintf(b, "    %s_section_%d -.->|\"reading order, not traffic\"| %s_section_%d\n", id, si, id, si+1)
			}
		}
		b.WriteString("  end\n")
	}
	for si, sec := range v.Side {
		section(sec, fmt.Sprintf("side_%d", si), 1)
	}
}

func writeEvidence(b *strings.Builder, m *model.Model, id, indent string) {
	e := m.Evidence[id]
	if e.How == "" {
		e.How = "unverified"
	}
	note := comment(e.Note)
	if note != "" {
		note = "; " + note
	}
	fmt.Fprintf(b, "%s%%%% evidence %s: %s%s\n", indent, comment(id), comment(e.How), note)
	sources := make([]string, 0, len(e.Sources))
	for _, source := range e.Sources {
		text := source.File
		if source.Line != nil {
			text += fmt.Sprintf(":%d", *source.Line)
		}
		if source.Object != "" {
			text = source.Object
		}
		sources = append(sources, text)
	}
	sort.Strings(sources)
	for _, source := range sources {
		fmt.Fprintf(b, "%s%%%% source %s: %s\n", indent, comment(id), comment(source))
	}
}

func renderAnnotations(b *strings.Builder, m *model.Model, shown map[string]bool) []string {
	annotations := append([]model.Annotation(nil), m.Annotations...)
	sort.Slice(annotations, func(i, j int) bool { return annotations[i].ID < annotations[j].ID })
	var ids []string
	for _, annotation := range annotations {
		visible := false
		for _, target := range annotation.Targets {
			visible = visible || shown[target]
		}
		if visible {
			id := "note_" + hex.EncodeToString([]byte(annotation.ID))
			targets := append([]string(nil), annotation.Targets...)
			sort.Strings(targets)
			text := annotation.Title + " [" + annotation.Severity + "]\n" + annotation.Text + "\nTargets: " + strings.Join(targets, ", ")
			fmt.Fprintf(b, "  %s[%s]\n  class %s note\n", id, label(wrap(text, 65)), id)
			ids = append(ids, id)
		}
	}
	return ids
}

func wrap(text string, width int) string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if len(line)+len(word)+1 > width && line != "" {
				lines = append(lines, line)
				line = ""
			}
			if line != "" {
				line += " "
			}
			line += word
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
