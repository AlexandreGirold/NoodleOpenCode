# Noodle · Vision

> Take the spaghetti out of a large architecture and keep it clear: a map authored as
> code, checked against the real infrastructure, explored and presented interactively,
> and eventually operated from.

## The problem

- **Nobody has the detailed map.** Platforms grow faster than their diagrams. What
  exists is a 30-second overview slide, or nothing. A tech lead cannot review a change
  without reading the code, and a new engineer spends days on onboarding for each
  cluster.
- **Diagrams lie.** They are drawn by hand, drift from the code, and nobody notices.
  Writing the first real diagram of the CNP platform surfaced six places where the docs
  and the code disagree, two of them security holes.
- **A static image forces one story.** To stay readable, a picture shows one path. The
  login flow, the failure path and the background jobs get dropped or turned into
  spaghetti.
- **AI has no context.** An assistant asked to add a feature or investigate an incident
  re-explores the whole project every time, because there is no structured description
  of the architecture it can read.
- **Incidents need a map.** When something breaks, the question is "where on the path
  is it blocked?". Today that path lives in someone's head.

## What noodle is

One **architecture model**, versioned in Git, from which everything else is derived:
static diagrams, interactive views, guided presentations, visual PR diffs, discovered
drafts, live overlays, and the context an AI needs.

## Principles

1. **Model first.** What exists (elements, connections, ports, zones) is described once.
   Views, layouts and presentations reference it; they never redefine it.
2. **Views are cheap.** Any number of perspectives, scenarios and highlights on one
   model. Showing one path is no longer a trade-off.
3. **Truth comes from code and infra; humans curate.** Discovery drafts the model from
   manifests, `kubectl` and network flows. People decide what matters. The gap between
   the two is reported, not hidden.
4. **Readable by construction.** Layout rules are enforced by a linter: no line through a
   box, no label over another, one step per connection, requests only. A diagram that
   breaks them is not emitted.
5. **Honest semantics.** An arrow is a connection, from the side that opens it to the
   side that listens, with the listening port on the server. Object references are not
   traffic. Responses are notes, not arrows.
6. **Git is the source of truth.** Model, views and layouts are reviewed in pull
   requests. Rendered artifacts are generated, never hand-maintained.
7. **Modules talk through contracts.** Versioned schemas between modules, so a new
   adapter or renderer plugs in without touching the others, and without an AI
   rewriting code nobody planned to change.
8. **Read-only by default.** Anything that touches live infrastructure starts read-only.
   Write actions come last, behind an explicit authorization model.
9. **AI-native.** The model is the context. It is exposed to agents so they reason on the
   architecture instead of rediscovering it.
10. **Presentation grade.** Output good enough for a tech lead review, a landing page or
    a portfolio: consistent house style, official logos, dark and light themes.

## Capabilities, by level

Each level builds on the previous one and stays usable on its own. Level 0 must always
work: mapping and producing diagrams is the original goal and remains the core.

### L0 · Author

Diagrams as code in the house style. Lint for readability and semantics. Exports:
draw.io, PNG, SVG, dark and light. *Exists as v0 for one diagram.*

Mermaid architecture documentation also derives from the same authoritative model:
overview, technologies, communication and deployment, with source evidence and explicit
uncertainty. Project-wide analysis is agent-assisted and covers application code and
configuration as well as Kubernetes. Documentation comes before the interactive explorer
in this slice (ADR-0021); it does not introduce a file-level knowledge graph.

### L1 · Explore

An interactive viewer on the same model:

- scenario selector to show one path or another;
- **focus**: select an element, its relations light up, the rest dims;
- perspectives and tags as overlays (risk, technology, owner, cost, environment);
- animated flow on connections, search, per-element documentation in Markdown;
- keyboard and screen-reader navigation;
- **export the current view** with its filters to SVG or PNG.

### L2 · Present

- **Flows** played step by step, with IcePanel-like step types: introduction, message,
  process, alternate paths, parallel paths, go-to-another-flow, information, conclusion.
- **Auto-play**: the same interactions a person would do by hand (scenario changes,
  focus, zoom, scope changes) played on a timer, for a landing page or a portfolio.
- Self-contained HTML export, and a public profile that redacts sensitive details.

### L3 · Review

- A model change in a PR produces a visual diff: added, removed and changed elements in
  dedicated colours, against the base branch.
- A static preview of the interactive viewer is linked from the PR.
- Future states (drafts) are simply branches.

### L4 · Discover

- Draft a model from Kubernetes manifests (Helm and Kustomize rendered), from `kubectl`
  with read-only access, from Cilium Hubble flows (real connections and ports), and
  from Terraform state.
- Every discovered fact carries its provenance.
- A **drift report** compares the curated model with what was discovered. A cluster
  nobody knows gets a usable map in minutes.

### L5 · Operate

- Live overlays on the map: Argo CD health, firing alerts, dropped flows. The map
  becomes a control-room board, like the mimic diagrams of power grids and railways.
- Click an element to see its facts and jump to logs, metrics, Argo CD.
- Test connectivity along a modelled path to find where it breaks.
- Later, guarded actions such as a restart, only once an authorization model exists.

### L6 · Assist

- An MCP server exposes the model, paths and live state to AI agents (HolmesGPT, Claude).
- An investigation starts from the topology instead of from scratch.
- Improvement proposals come back as patches to the model, reviewed like any PR.

## Audiences

| Who | Needs |
|---|---|
| Tech lead or reviewer | the right granularity, the real semantics, what a change adds or removes |
| New engineer | a guided tour of an unknown platform or cluster |
| Presenter, portfolio visitor | a clear, animated story in a few minutes |
| On-call engineer | where the request path breaks, and the links to dig in |
| AI agent | a structured, queryable description instead of re-exploring the repo |

## Non-goals, for now

- Replacing Argo CD, Grafana or Hubble. Noodle links to them and overlays their signals.
- A general-purpose whiteboard or a free-form drawing tool.
- A hosted multi-tenant SaaS.
- Write actions on clusters before an authorization model exists.

## How we will know it works

- **L0/L1:** a tech lead understands a platform from its diagrams without opening the code.
- **L2:** a portfolio visitor follows the project story end to end with no narrator.
- **L3:** every architecture PR carries a visual diff link.
- **L4:** onboarding on an unknown cluster goes from days to an afternoon.
- **L5/L6:** during an incident, the first question ("where is it blocked?") is answered
  from the map.

## Open questions

- Storage for live data and history beyond Git: needed from L5 on.
- Authentication and authorization for the viewer and the cockpit, and redaction rules
  for public exports.
- Layout at scale: when manual coordinates stop being viable, and how pinned and
  automatic layouts coexist.
- How far sequence diagrams and topology views share one rendering engine.
