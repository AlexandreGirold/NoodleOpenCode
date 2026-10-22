package inventory

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/TheGostsniperfr/Noodle/internal/model"
)

var requirementsName = regexp.MustCompile(`^requirements([._-][a-z0-9_-]+)?\.(txt|in)$`)
var dependencyName = regexp.MustCompile(`^(@[a-zA-Z0-9_.-]+/)?[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
var requirement = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)(\[[A-Za-z0-9,._ -]+\])?\s*([<>=!~;@].*)?$`)
var imageName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]*(@sha256:[a-fA-F0-9]{64})?$`)
var serviceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// Only declaration fields are checked; runtime command/env interpolation is not
// interpreted as a listening port or copied into the report.
func dynamicKubernetes(raw []byte) *yaml.Node {
	keys := map[string]bool{"apiVersion": true, "kind": true, "image": true, "name": true, "namespace": true, "port": true, "containerPort": true, "targetPort": true}
	seen := map[*yaml.Node]bool{}
	var find func(*yaml.Node) *yaml.Node
	find = func(n *yaml.Node) *yaml.Node {
		if n == nil || seen[n] {
			return nil
		}
		seen[n] = true
		if n.Kind == yaml.AliasNode {
			return find(n.Alias)
		}
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				value := n.Content[i+1]
				if value.Kind == yaml.AliasNode {
					value = value.Alias
				}
				if n.Content[i].Value == "image" && value != nil && (!imageName.MatchString(value.Value) || strings.Contains(value.Value, "://")) {
					return value
				}
				if keys[n.Content[i].Value] && value != nil && (strings.Contains(value.Value, "${") || strings.Contains(value.Value, "{{") || strings.Contains(value.Value, "$(")) {
					return value
				}
			}
		}
		for _, child := range n.Content {
			if result := find(child); result != nil {
				return result
			}
		}
		return nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return nil
		} // document validation already reported invalid YAML
		if result := find(&doc); result != nil {
			return result
		}
	}
}

// field follows YAML aliases and merge mappings with a depth bound for cycles.
func field(n *yaml.Node, key string) *yaml.Node { return lookup(n, key, 0) }

func lookup(n *yaml.Node, key string, depth int) *yaml.Node {
	if n == nil || depth > 32 {
		return nil
	}
	if n.Kind == yaml.AliasNode {
		return lookup(n.Alias, key, depth+1)
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value != "<<" {
			continue
		}
		merge := n.Content[i+1]
		if merge.Kind == yaml.SequenceNode {
			for _, child := range merge.Content {
				if found := lookup(child, key, depth+1); found != nil {
					return found
				}
			}
		} else if found := lookup(merge, key, depth+1); found != nil {
			return found
		}
	}
	return nil
}

func at(src model.Src, n *yaml.Node) model.Src {
	if n != nil && n.Line > 0 {
		src.Line = n.Line
	}
	return src
}

func (r *Report) compose(doc *yaml.Node, src model.Src) {
	services := field(doc, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		r.issue("compose", "services-mapping-required", src)
		return
	}
	if field(doc, "include") != nil {
		r.issue("compose", "includes-not-expanded", at(src, field(doc, "include")))
	}
	var names []string
	byName := map[string]*yaml.Node{}
	for i := 0; i+1 < len(services.Content); i += 2 {
		name := services.Content[i].Value
		if !serviceName.MatchString(name) {
			r.issue("compose", "service-name-not-literal", at(src, services.Content[i]))
			continue
		}
		body := services.Content[i+1]
		if body.Kind != yaml.MappingNode && body.Kind != yaml.AliasNode {
			r.issue("compose", "service-mapping-required", at(src, body))
			continue
		}
		if _, duplicate := byName[name]; duplicate {
			r.issue("compose", "duplicate-service", at(src, services.Content[i]))
			continue
		}
		byName[name] = services.Content[i+1]
		names = append(names, name)
	}
	sort.Strings(names)
	id := func(name string) string {
		return "compose:" + url.PathEscape(src.File) + "/service/" + url.PathEscape(name)
	}
	for _, name := range names {
		service := byName[name]
		if service.Kind != yaml.MappingNode && service.Kind != yaml.AliasNode {
			r.issue("compose", "service-mapping-required", at(src, service))
			continue
		}
		e := model.Element{ID: id(name), Kind: "backend", Title: name, Tech: "Docker Compose"}
		if image := field(service, "image"); image != nil {
			if strings.Contains(image.Value, "$") || strings.Contains(image.Value, "://") || !imageName.MatchString(image.Value) {
				r.issue("compose", "image-not-literal", at(src, image))
			} else {
				e.Tech += " · " + image.Value
			}
		}
		if field(service, "extends") != nil {
			r.issue("compose", "extends-not-expanded", at(src, field(service, "extends")))
		}
		ports := map[model.Port]bool{}
		for _, key := range []string{"ports", "expose"} {
			list := field(service, key)
			if list == nil {
				continue
			}
			if list.Kind != yaml.SequenceNode {
				r.issue("compose", "port-list-required", at(src, list))
				continue
			}
			for _, port := range list.Content {
				target, protocol := port.Value, "TCP"
				if port.Kind == yaml.MappingNode {
					if key == "expose" {
						r.issue("compose", "port-not-literal", at(src, port))
						continue
					}
					n := field(port, "target")
					if n == nil {
						r.issue("compose", "port-target-required", at(src, port))
						continue
					}
					target = n.Value
					if p := field(port, "protocol"); p != nil {
						protocol = strings.ToUpper(p.Value)
					}
				} else if port.Kind == yaml.ScalarNode {
					var suffix string
					if before, after, ok := strings.Cut(target, "/"); ok {
						target, suffix = before, after
					}
					if suffix != "" {
						protocol = strings.ToUpper(suffix)
					}
					if strings.Contains(target, "$") {
						r.issue("compose", "port-not-literal", at(src, port))
						continue
					}
					if key == "ports" {
						target = target[strings.LastIndex(target, ":")+1:]
					}
				} else {
					r.issue("compose", "port-not-literal", at(src, port))
					continue
				}
				number, err := strconv.Atoi(target)
				if err != nil || number < 1 || number > 65535 || (protocol != "TCP" && protocol != "UDP") {
					r.issue("compose", "port-not-literal", at(src, port))
					continue
				}
				ports[model.Port{Name: strings.ToLower(protocol) + "-" + strconv.Itoa(number), Protocol: protocol, Number: number}] = true
			}
		}
		for port := range ports {
			e.Ports = append(e.Ports, port)
		}
		sort.Slice(e.Ports, func(i, j int) bool {
			if e.Ports[i].Number != e.Ports[j].Number {
				return e.Ports[i].Number < e.Ports[j].Number
			}
			return e.Ports[i].Protocol < e.Ports[j].Protocol
		})
		r.Elements = append(r.Elements, model.DiscoveredElement{Element: e, Src: at(src, service)})
		depends := field(service, "depends_on")
		if depends == nil {
			continue
		}
		var dependencies []*yaml.Node
		switch depends.Kind {
		case yaml.SequenceNode:
			dependencies = depends.Content
		case yaml.MappingNode:
			for i := 0; i+1 < len(depends.Content); i += 2 {
				dependencies = append(dependencies, depends.Content[i])
			}
		default:
			r.issue("compose", "dependency-not-literal", at(src, depends))
		}
		seen := map[string]bool{}
		for _, dependency := range dependencies {
			other := dependency.Value
			if _, exists := byName[other]; !exists || strings.Contains(other, "$") {
				r.issue("compose", "missing-or-dynamic-dependency", at(src, dependency))
				continue
			}
			if seen[other] {
				continue
			}
			seen[other] = true
			r.References = append(r.References, model.DiscoveredReference{Reference: model.Reference{
				ID: id(name) + "/depends_on/" + url.PathEscape(other), From: id(name), To: id(other), Kind: "depends_on"}, Src: at(src, dependency)})
		}
	}
}

func (r *Report) npm(doc *yaml.Node, raw []byte, src model.Src) {
	if !json.Valid(raw) || doc == nil || doc.Kind != yaml.MappingNode {
		r.issue("npm", "invalid-package-json", src)
		return
	}
	p := Package{Ecosystem: "npm", Src: src}
	if name := field(doc, "name"); name != nil && dependencyName.MatchString(name.Value) {
		p.Name, p.Src = name.Value, at(src, name)
	}
	for _, scope := range []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"} {
		dependencies := field(doc, scope)
		if dependencies == nil {
			continue
		}
		if dependencies.Kind != yaml.MappingNode {
			r.issue("npm", "dependency-mapping-required", at(src, dependencies))
			continue
		}
		for i := 0; i+1 < len(dependencies.Content); i += 2 {
			name := dependencies.Content[i]
			if !dependencyName.MatchString(name.Value) {
				r.issue("npm", "unsupported-dependency-name", at(src, name))
				continue
			}
			// Only dependency names/scopes; never versions, registry URLs or scripts.
			p.Dependencies = append(p.Dependencies, Dependency{Name: name.Value, Scope: scope, Src: at(src, name)})
		}
	}
	sort.Slice(p.Dependencies, func(i, j int) bool {
		if p.Dependencies[i].Scope != p.Dependencies[j].Scope {
			return p.Dependencies[i].Scope < p.Dependencies[j].Scope
		}
		return p.Dependencies[i].Name < p.Dependencies[j].Name
	})
	r.Packages = append(r.Packages, p)
}

func (r *Report) requirements(raw []byte, src model.Src) {
	p := Package{Ecosystem: "pypi", Src: src}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), maxFileBytes)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		where := src
		where.Line = line
		if before, _, ok := strings.Cut(text, " #"); ok {
			text = strings.TrimSpace(before)
		}
		match := requirement.FindStringSubmatch(text)
		if match == nil {
			r.issue("requirements", "directive-or-dynamic-requirement", where)
			continue
		}
		p.Dependencies = append(p.Dependencies, Dependency{Name: match[1], Scope: "requirements", Src: where})
	}
	if scanner.Err() != nil {
		r.issue("requirements", "invalid-requirements", src)
	}
	sort.SliceStable(p.Dependencies, func(i, j int) bool { return p.Dependencies[i].Name < p.Dependencies[j].Name })
	r.Packages = append(r.Packages, p)
}

func (r *Report) addKubernetes(root Root, scope string, fragment *model.Fragment) {
	dir, _ := filepath.Rel(root.Path, scope)
	qualify := func(id string) string {
		if id == "" {
			return ""
		}
		prefix, rest, _ := strings.Cut(id, ":")
		return prefix + ":" + url.PathEscape(root.ID) + "/" + url.PathEscape(filepath.ToSlash(dir)) + "/" + rest
	}
	source := func(src model.Src) model.Src {
		if rel, err := filepath.Rel(root.Path, src.File); err == nil {
			src.File = root.ID + "/" + filepath.ToSlash(rel)
		}
		return src
	}
	known := map[string]model.Element{}
	for _, e := range fragment.Elements {
		e.ID, e.Parent, e.Src = qualify(e.ID), qualify(e.Parent), source(e.Src)
		if !model.IsDiscoveredID(e.ID) || (e.Parent != "" && !model.IsDiscoveredID(e.Parent)) {
			r.issue("kubernetes", "invalid-resource-identity", e.Src)
			continue
		}
		ports := e.Ports[:0]
		for _, port := range e.Ports {
			if port.Number < 1 || port.Number > 65535 || (port.Protocol != "TCP" && port.Protocol != "UDP" && port.Protocol != "SCTP") {
				r.issue("kubernetes", "invalid-port-declaration", e.Src)
				continue
			}
			ports = append(ports, port)
		}
		e.Ports = ports
		known[e.ID] = e.Element
		r.Elements = append(r.Elements, e)
	}
	for _, c := range fragment.Connections {
		c.ID, c.From, c.To, c.Src = qualify(c.ID), qualify(c.From), qualify(c.To), source(c.Src)
		_, fromExists := known[c.From]
		target, toExists := known[c.To]
		portExists := c.Port == ""
		for _, port := range target.Ports {
			if port.Name == c.Port {
				portExists = true
			}
		}
		if !fromExists || !toExists || !portExists {
			r.issue("kubernetes", "unresolved-connection-target", c.Src)
			continue
		}
		for i := range c.EnforcedBy {
			c.EnforcedBy[i] = qualify(c.EnforcedBy[i])
		}
		r.Connections = append(r.Connections, c)
	}
	for _, ref := range fragment.References {
		ref.ID, ref.From, ref.To, ref.Src = qualify(ref.ID), qualify(ref.From), qualify(ref.To), source(ref.Src)
		_, fromExists := known[ref.From]
		_, toExists := known[ref.To]
		if !fromExists || !toExists {
			r.issue("kubernetes", "unresolved-reference-target", ref.Src)
			continue
		}
		r.References = append(r.References, ref)
	}
	for _, unresolved := range fragment.Unresolved {
		// Keep source/reason, not arbitrary unresolved input values.
		reason := unresolved.Kind
		if unresolved.Hint != "" {
			reason += ":" + unresolved.Hint
		}
		r.issue("kubernetes", reason, source(unresolved.Src))
	}
}
