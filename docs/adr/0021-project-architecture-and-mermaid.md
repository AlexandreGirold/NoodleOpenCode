# ADR-0021 · A source-backed architecture model generates Mermaid

- Status: Accepted
- Date: 2026-10-22
- Amends: ADR-0007 (layout requirements), ADR-0016 (delivery order)

## Context

The user wants whole-project architecture understanding: technologies, services,
deployment and communication, rather than a file-level knowledge graph. Kubernetes
remains supported, but is one source among application code, Compose, IaC and service
configuration. They chose a shared model over authoritative Mermaid, agent-assisted
analysis, documentation first, and an explorer later.

## Decision

- Deliver spec 005 before continuing the existing discovery backlog. Retain that
  backlog and its verification requirements; this feature does not declare it complete.
- Keep the curated model authoritative. Generate `.mmd` views from facts, not from
  draw.io geometry. Start with topology and landscape output; overview, communication
  and deployment are separately selected topology perspectives, not new view types.
- Introduce `noodle/v1alpha2` for Model, View and Layout. Retain v1alpha1 reading and
  the v1alpha1 Fragment contract. Model/view/layout versions agree within a system.
- Add `evidence`, a map keyed by element, connection or reference ID. Each entry has
  `how: code | config | inferred | docs | unknown`, `sources: [{file, line} | {object}]`
  and an optional `note`. Every element, connection and reference in a v1alpha2 model
  has evidence. Non-unknown entries need sources; unknown entries need a note.
  Unknown relationships should normally be questions on their known components,
  rather than invented connection facts.
- Allow coordinate-free v1alpha2 topology authoring. Architecture validation checks
  facts and views; draw.io still requires valid layouts and all existing geometry
  lint. v1alpha1 retains its layout requirement. An explicit directory upgrade keeps
  all facts and marks missing legacy evidence unknown; it never overwrites a system.
- Runtime arrows remain client-to-listener connections. References are distinct.
  Mermaid puts known target ports in labels and uses automatic layout; it does not
  promise draw.io's endpoint sides, border badges or collision guarantees. Rendered
  Mermaid must be reviewed visually. Unsupported meaningful features fail explicitly.
- Use the coding agent to investigate and curate across technologies. Deterministic
  Kubernetes discovery supplements it; a dependency declaration alone is not traffic.
  Evidence paths are relative to the system directory unless stated otherwise.
- Keep the skill a short workflow with essential rules and links. Detailed contract
  syntax and recipes live in references and working examples.

## Consequences

One model can feed consistent documentation and a future explorer without manual
coordinates. Updating generated Mermaid requires changing its source model or view.
Versioned schemas, an explicit upgrade, format-specific tests and visual verification
protect existing diagrams. Automatic arbitrary-project analysis is an agent workflow,
not a new promise that the CLI can statically interpret every technology.
