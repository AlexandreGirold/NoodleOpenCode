// Package model holds the versioned noodle contracts: what exists (Model), what is
// shown (View) and where it is drawn (Layout), and loads them from a system directory.
package model

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const APIVersion = "noodle/v1alpha1"

// ArchitectureVersion is the apiVersion of v1alpha2 architecture models, views and
// layouts (ADR-0021). v1alpha1 remains readable and fragments stay v1alpha1.
const ArchitectureVersion = "noodle/v1alpha2"

type Model struct {
	APIVersion  string       `yaml:"apiVersion"`
	Kind        string       `yaml:"kind"`
	Elements    []Element    `yaml:"elements"`
	Connections []Connection `yaml:"connections"`
	References  []Reference  `yaml:"references"`
	Annotations []Annotation `yaml:"annotations"`
	Offerings   []Offering   `yaml:"offerings"`
	Memberships []Membership `yaml:"memberships"`
	Grants      []Grant      `yaml:"grants"`
	// Evidence is keyed by element, connection or reference id (ADR-0021).
	Evidence map[string]Evidence `yaml:"evidence,omitempty"`
}

// Evidence records how a fact was established.
type Evidence struct {
	How     string           `yaml:"how"`
	Sources []EvidenceSource `yaml:"sources,omitempty"`
	Note    string           `yaml:"note,omitempty"`
}

func (e *Evidence) UnmarshalYAML(node *yaml.Node) error {
	var fields map[string]yaml.Node
	if err := node.Decode(&fields); err != nil {
		return err
	}
	for key, value := range fields {
		value = evidenceValue(value)
		switch key {
		case "how", "note":
			if value.Tag != "!!str" {
				return fmt.Errorf("evidence %s must be a string", key)
			}
		case "sources":
			if value.Kind != yaml.SequenceNode {
				return fmt.Errorf("evidence sources must be an array")
			}
		default:
			return fmt.Errorf("unknown evidence field %q", key)
		}
	}
	type plain Evidence
	var out plain
	if err := node.Decode(&out); err != nil {
		return err
	}
	*e = Evidence(out)
	return nil
}

// EvidenceSource distinguishes an omitted line from an explicit line: 0 is valid for
// source-backed evidence, but lines cannot be attached to API object sources.
type EvidenceSource struct {
	File   string `yaml:"file,omitempty"`
	Line   *int   `yaml:"line,omitempty"`
	Object string `yaml:"object,omitempty"`
}

// UnmarshalYAML preserves the source contract's field-presence rules: an empty
// object key alongside file is still two source alternatives, not an absent key.
func (s *EvidenceSource) UnmarshalYAML(node *yaml.Node) error {
	var fields map[string]yaml.Node
	if err := node.Decode(&fields); err != nil {
		return err
	}
	_, file := fields["file"]
	_, object := fields["object"]
	if file == object {
		return fmt.Errorf("evidence source needs exactly one of file or object")
	}
	for key, value := range fields {
		value = evidenceValue(value)
		switch key {
		case "file", "object":
			if value.Tag != "!!str" || strings.TrimSpace(value.Value) == "" {
				return fmt.Errorf("evidence source %s must be a nonblank string", key)
			}
		case "line":
			if !file || value.Tag != "!!int" {
				return fmt.Errorf("evidence line must be an integer accompanying file")
			}
		default:
			return fmt.Errorf("unknown evidence source field %q", key)
		}
	}
	type plain EvidenceSource
	var out plain
	if err := node.Decode(&out); err != nil {
		return err
	}
	*s = EvidenceSource(out)
	return nil
}

// YAML aliases have the same contract meaning as their resolved value.
func evidenceValue(node yaml.Node) yaml.Node {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = *node.Alias
	}
	return node
}

// Membership says Subject belongs to Group or assumes it: a user in a Keycloak group, a
// ServiceAccount bound to a Vault auth role (ADR-0019).
type Membership struct {
	ID      string `yaml:"id"`
	Subject string `yaml:"subject"`
	Group   string `yaml:"group"`
	Auth    *Auth  `yaml:"auth,omitempty"`
	Status  string `yaml:"status,omitempty"`
	Target  string `yaml:"target,omitempty"`
}

// Grant says Subject may act on Resource at Level, limited to Scope, through the
// elements in Via (a policy, a RoleBinding, an IAM role), checked like enforced_by
// (ADR-0010, ADR-0019).
type Grant struct {
	ID       string   `yaml:"id"`
	Subject  string   `yaml:"subject"`
	Resource string   `yaml:"resource"`
	Level    string   `yaml:"level"`
	Scope    string   `yaml:"scope,omitempty"`
	Via      []string `yaml:"via,omitempty"`
	Auth     *Auth    `yaml:"auth,omitempty"`
	Status   string   `yaml:"status,omitempty"`
	Target   string   `yaml:"target,omitempty"`
}

// Auth is how an identity proves itself on one hop. Lifetime is free text ("1h");
// empty means the credential does not expire.
type Auth struct {
	Method   string `yaml:"method"`
	Lifetime string `yaml:"lifetime,omitempty"`
	MFA      bool   `yaml:"mfa,omitempty"`
}

// Offering is what a platform promises its users and how to get it (ADR-0013).
// BackedBy lists the elements that deliver it.
type Offering struct {
	ID       string   `yaml:"id"`
	Title    string   `yaml:"title"`
	Icon     string   `yaml:"icon"`
	Summary  string   `yaml:"summary"`
	Provides []string `yaml:"provides"`
	Request  string   `yaml:"request"`
	BackedBy []string `yaml:"backed_by"`
	Owner    string   `yaml:"owner"`
	Status   string   `yaml:"status"`
	Target   string   `yaml:"target"`
}

// Element is a component or a zone. Zones have kind region or group and may carry
// color and sub.
type Element struct {
	ID           string   `yaml:"id"`
	Kind         string   `yaml:"kind,omitempty"`
	Parent       string   `yaml:"parent,omitempty"`
	Title        string   `yaml:"title,omitempty"`
	Tech         string   `yaml:"tech,omitempty"`
	Desc         string   `yaml:"desc,omitempty"`
	Icon         string   `yaml:"icon,omitempty"`
	Shape        string   `yaml:"shape,omitempty"`
	Color        string   `yaml:"color,omitempty"`
	Sub          string   `yaml:"sub,omitempty"`
	Ports        []Port   `yaml:"ports,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	Status       string   `yaml:"status,omitempty"`
	Target       string   `yaml:"target,omitempty"`
	Multiplicity string   `yaml:"multiplicity,omitempty"`
	// Matches lists the discovered ids this element stands for (ADR-0015).
	Matches []string `yaml:"matches,omitempty"`
}

func (e Element) IsZone() bool { return e.Kind == "region" || e.Kind == "group" }

type Port struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	Number   int    `yaml:"port"`
}

// Connection is opened by From and listened to by To (ADR-0003). Port names a port of To.
type Connection struct {
	ID       string `yaml:"id"`
	From     string `yaml:"from,omitempty"`
	To       string `yaml:"to,omitempty"`
	Port     string `yaml:"port,omitempty"`
	Protocol string `yaml:"protocol,omitempty"`
	Verb     string `yaml:"verb,omitempty"`
	Kind     string `yaml:"kind,omitempty"`
	// Denied is the intent that this connection must not happen; EnforcedBy names the
	// elements that make it so (ADR-0010). Denied without EnforcedBy is intent only.
	Denied     bool     `yaml:"denied,omitempty"`
	EnforcedBy []string `yaml:"enforced_by,omitempty"`
}

type Reference struct {
	ID   string `yaml:"id"`
	From string `yaml:"from,omitempty"`
	To   string `yaml:"to,omitempty"`
	Kind string `yaml:"kind,omitempty"`
}

type Annotation struct {
	ID       string   `yaml:"id"`
	Severity string   `yaml:"severity"`
	Title    string   `yaml:"title"`
	Text     string   `yaml:"text"`
	Targets  []string `yaml:"targets"`
}

// Fragment is what an adapter discovers from one source (ADR-0015). Its ids are
// discovered ids, never curated ones; the curated model links to them with matches.
type Fragment struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	// ID is the file stem, <adapter>-<source>.
	ID          string                 `yaml:"-"`
	Provenance  Provenance             `yaml:"provenance"`
	Elements    []DiscoveredElement    `yaml:"elements"`
	Connections []DiscoveredConnection `yaml:"connections"`
	References  []DiscoveredReference  `yaml:"references"`
	Unresolved  []Unresolved           `yaml:"unresolved"`
}

// Unresolved is what an adapter saw but could not settle, kept for an agent or a person
// to answer (ADR-0017). Count is set when one entry stands for several objects.
type Unresolved struct {
	About []string `yaml:"about,omitempty"`
	Kind  string   `yaml:"kind"`
	Value string   `yaml:"value,omitempty"`
	Hint  string   `yaml:"hint,omitempty"`
	Count int      `yaml:"count,omitempty"`
	Src   Src      `yaml:"src"`
}

type Provenance struct {
	Adapter    string `yaml:"adapter,omitempty"`
	Source     string `yaml:"source,omitempty"`
	Ref        string `yaml:"ref,omitempty"`
	ObservedAt string `yaml:"observedAt,omitempty"`
}

// Src is where one discovered item comes from: a file and line, or an API object.
type Src struct {
	File   string `yaml:"file,omitempty"`
	Line   int    `yaml:"line,omitempty"`
	Object string `yaml:"object,omitempty"`
}

type DiscoveredElement struct {
	Element `yaml:",inline"`
	Src     Src `yaml:"src"`
}

// DiscoveredConnection is Inferred when a heuristic found it, such as a Service DNS
// name in an env value, rather than a manifest that declares it.
type DiscoveredConnection struct {
	Connection `yaml:",inline"`
	Inferred   bool `yaml:"inferred,omitempty"`
	Src        Src  `yaml:"src,omitempty"`
}

type DiscoveredReference struct {
	Reference `yaml:",inline"`
	Src       Src `yaml:"src"`
}

// discoveredID is <adapter>:<path>, e.g. k8s:prod/deployment/api (ADR-0015).
var discoveredID = regexp.MustCompile(`^[a-z][a-z0-9]*:\S+$`)

func IsDiscoveredID(id string) bool { return discoveredID.MatchString(id) }

type View struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	// ID is the file stem, not a field: renaming the file renames the view.
	ID           string            `yaml:"-"`
	Type         string            `yaml:"type"`
	Title        string            `yaml:"title"`
	Subtitle     string            `yaml:"subtitle"`
	Meta         []string          `yaml:"meta"`
	Include      []string          `yaml:"include"`
	Steps        []Step            `yaml:"steps"`
	Background   []string          `yaml:"background"`
	Labels       map[string]string `yaml:"labels"`
	Participants []string          `yaml:"participants"`
	Cards        []Card            `yaml:"cards"`
	Notes        []Note            `yaml:"notes"`
	// Landscape views (ADR-0012): rows of sections, an optional side column, and the
	// canvas width the grid wraps to.
	Bands []Band    `yaml:"bands"`
	Side  []Section `yaml:"side"`
	Width float64   `yaml:"width"`
	// Catalog views (ADR-0013): cards per row; Include lists offering ids.
	Columns int `yaml:"columns"`
	// Matrix views (ADR-0019): identities as row groups, resources as column groups,
	// and which grants count: current, target or diff.
	Identities []Section `yaml:"identities"`
	Resources  []Section `yaml:"resources"`
	State      string    `yaml:"state"`
}

// Band is one row of a landscape. Flow draws an arrow between consecutive sections.
type Band struct {
	ID       string    `yaml:"id"`
	Title    string    `yaml:"title"`
	Sub      string    `yaml:"sub"`
	Color    string    `yaml:"color"`
	Icon     string    `yaml:"icon"`
	Flow     bool      `yaml:"flow"`
	Sections []Section `yaml:"sections"`
}

// Section groups landscape items, or a matrix's rows or columns; Items are element ids.
type Section struct {
	Title string   `yaml:"title"`
	Sub   string   `yaml:"sub"`
	Color string   `yaml:"color"`
	Items []string `yaml:"items"`
}

// Step is a connection id in a topology view, and a message, reply or note in a
// sequence view (ADR-0011).
type Step struct {
	Connection string
	ID         string
	From       string
	To         string
	Over       string
	Text       string
	Reply      string
	Note       string
}

func (s Step) IsMessage() bool { return s.From != "" || s.To != "" || s.Over != "" }

var stepKeys = map[string]bool{"id": true, "from": true, "to": true, "over": true, "text": true, "reply": true, "note": true}

func (s *Step) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		s.Connection = n.Value
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a step is a connection id or a mapping", n.Line)
	}
	fields := map[string]*string{"id": &s.ID, "from": &s.From, "to": &s.To, "over": &s.Over, "text": &s.Text, "reply": &s.Reply, "note": &s.Note}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if !stepKeys[k.Value] {
			return fmt.Errorf("line %d: field %s not found in type model.Step", k.Line, k.Value)
		}
		if v.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: step field %s must be a string", v.Line, k.Value)
		}
		*fields[k.Value] = v.Value
	}
	return nil
}

type Card struct {
	ID     string   `yaml:"id"`
	Title  string   `yaml:"title"`
	Color  string   `yaml:"color"`
	Legend bool     `yaml:"legend"`
	Lines  []string `yaml:"lines"`
}

type Note struct {
	ID   string `yaml:"id"`
	Text string `yaml:"text"`
}

type Layout struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	// ID is the file stem and names the view this layout draws.
	ID       string               `yaml:"-"`
	Canvas   Canvas               `yaml:"canvas"`
	Lanes    map[string]Lane      `yaml:"lanes"`
	Elements map[string]Box       `yaml:"elements"`
	Edges    map[string]EdgeRoute `yaml:"edges"`
	Cards    map[string]Box       `yaml:"cards"`
	Notes    map[string]Box       `yaml:"notes"`
}

type Canvas struct {
	Width  float64 `yaml:"width"`
	Height float64 `yaml:"height"`
}

// Lane is a named vertical (X) or horizontal (Y) line that edge routes can share.
type Lane struct {
	X *float64 `yaml:"x"`
	Y *float64 `yaml:"y"`
}

// Box is relative to the parent zone's top-left corner, or to the canvas at top level.
type Box struct {
	X float64 `yaml:"x"`
	Y float64 `yaml:"y"`
	W float64 `yaml:"w"`
	H float64 `yaml:"h"`
}

type Point [2]float64

// EdgeRoute draws one connection or reference. From and To are "id.side",
// "id.side@NN%" or "id.side@NNpx". Waypoints and LabelAt are canvas coordinates.
type EdgeRoute struct {
	From        string     `yaml:"from"`
	To          string     `yaml:"to"`
	Waypoints   []Waypoint `yaml:"waypoints"`
	LabelAt     *Point     `yaml:"label_at"`
	LabelOffset Point      `yaml:"label_offset"`
	AgainstFlow bool       `yaml:"against_flow"`
}

// Waypoint is either an absolute point [x, y] or "lane:<name>".
type Waypoint struct {
	Point *Point
	Lane  string
}

func (w *Waypoint) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		lane, ok := strings.CutPrefix(n.Value, "lane:")
		if !ok || lane == "" {
			return fmt.Errorf("line %d: waypoint %q is neither [x, y] nor lane:<name>", n.Line, n.Value)
		}
		w.Lane = lane
		return nil
	}
	var p Point
	if err := n.Decode(&p); err != nil {
		return fmt.Errorf("line %d: waypoint: %w", n.Line, err)
	}
	w.Point = &p
	return nil
}

// Endpoint splits "id.side@NN%" into its element id and the rest.
func Endpoint(s string) (id, anchor string) {
	id, anchor, _ = strings.Cut(s, ".")
	return id, anchor
}
