# Noodle · Architecture overview

Status: **draft**. The original model/view/layout contract is `v1alpha1` (ADR-0007);
`v1alpha2` adds evidence and coordinate-free Mermaid authoring (ADR-0021). Both
versions are readable, and discovery fragments remain v1alpha1. The sketches below also show parts not built yet (tours,
redaction, overlays, adapters). The v0 single-file format is gone: `noodle migrate`
converts old files.

## Shape

```
            ┌──────────────── adapters (in) ─────────────────┐
            │ manual YAML · k8s manifests · kubectl (RO)     │
            │ Hubble flows · Terraform state                 │
            └───────────────────────┬────────────────────────┘
                                    ▼  model fragments + provenance
 ┌───────────────────────────────────────────────────────────────┐
 │ MODEL            elements · connections · references · ports  │◀── lint rules
 │ (noodle/v1alpha1) zones · annotations (gaps) · links          │    (plugins)
 └──────────┬──────────────────────────────────┬─────────────────┘
            ▼                                  ▼
 VIEWS: perspectives · scenarios/flows    MCP server: query, path-find,
        · tours · tags · redaction        validate, propose patch
            ▼                                  ▲
 LAYOUT: pinned coordinates or auto       │  OVERLAY feed
            ▼                                  │  {element, state, since, links}
 RENDERERS: drawio · SVG (animated) · PNG · web viewer · HTML bundle
```

## Modules

| Module | Input | Output | Notes |
|---|---|---|---|
| `model` | fragments | merged, validated model | owns the schema and the merge rules |
| `lint` | model, view, layout | findings | rules are independent plugins; v0 rules move here |
| `adapter-*` | a source (files, API) | model fragment | never edits the curated model directly |
| `layout` | model + view | layout | pinned coordinates win; auto-layout fills the rest |
| `render-*` | model + view + layout + theme | artifact | drawio, svg, png, html |
| `viewer` | the same four files | interactive page | static build; no server needed below L5 |
| `diff` | two model versions | change set | drives the PR visual diff |
| `overlay-*` | a live source | overlay events | read-only; L5 |
| `mcp` | model (+ overlays) | MCP tools | L6, but a read-only version is cheap early |

**Rule: a module depends on contracts, never on another module's internals.**

### Project architecture documentation (implemented)

`internal/inventory` and `noodle scan` provide a deterministic repository preflight
(ADR-0022). A separately versioned Inventory report lists recognized inputs/hashes,
reuses the Kubernetes adapter, extracts conservative Compose/dependency declarations,
and records unsupported/dynamic source locations. This never changes the curated model,
executes projects or substitutes dependency declarations for runtime connections.

The coding agent investigates relevant application clients, endpoint configuration,
deployment inputs and manifests, then curates one model. The CLI validates and generates;
it is not a general-purpose autonomous source analyzer. Kubernetes discovery provides
deterministic fragments for the subset it supports.

`internal/render/mermaid` consumes model/view contracts directly. `CheckArchitecture`
validates facts, evidence and view semantics; v1alpha2 topology does not require a
coordinate layout. The existing draw.io path still runs `Check`, resolves explicit
geometry and applies all geometric lint. Mermaid calculates its own geometry and puts
target ports in edge labels; rendered output needs visual review.

v1alpha2's `evidence` map records every element, connection and reference's classification,
source locations and explanation of unknowns. The explicit `upgrade` command preserves
legacy facts in a new directory and marks their evidence unknown. The same model produces
overview, technology, communication and deployment perspectives and can later feed an
explorer without reconstructing facts from Mermaid labels.

## Contracts (sketches)

Phase 1 pipeline and file split (ADR-0007):

![Contract pipeline](diagrams/contract-pipeline/contract-pipeline.dark.png)

### Model: what exists

```yaml
apiVersion: noodle/v1alpha1
kind: Model
elements:
  - id: envoy-shared-gateway
    kind: backend            # frontend | backend | database | cloud | security | bus | external
    title: Envoy proxy
    tech: Gateway API · shared-gateway
    desc: Routes *.3istor.com, runs OIDC
    icon: envoy
    parent: g-gateway        # zones are elements too, with kind: region | group
    ports: [{name: http, protocol: TCP, port: 80}]
    tags: [risk:G4]
    links: [{title: Argo CD, url: "..."}]
connections:                 # network: client opens, server listens
  - id: c-connector-envoy
    from: cloudflared-app
    to: envoy-shared-gateway
    port: http               # references a port of the target
    protocol: HTTP
    verb: forward
references:                  # configuration, not traffic
  - {id: r-route-gateway, from: httproute, to: envoy-shared-gateway, kind: parentRef}
annotations:
  - {id: G1, severity: high, title: Network isolation, targets: [g-tenant-a], text: "..."}
```

### View: what is shown

```yaml
apiVersion: noodle/v1alpha1
kind: View
id: runtime-request-path
model: cnp
include: [g-cluster/**, cf-edge, user]
scenarios:
  - id: nominal
    title: Signed-in user
    steps:
      - {type: introduction, text: "..."}
      - {type: message, connection: c-browser-edge, text: "GET app"}
      - {type: alternate, paths: [{title: No session, flow: oidc-login}, {title: Session, steps: [...]}]}
      - {type: conclusion}
tours:
  - {id: portfolio, autoplay: 6s, frames: [{scenario: nominal, focus: envoy-shared-gateway, zoom: g-gateway}]}
redaction: public            # hides annotations and internal hostnames
```

### Layout: where things are

Per view: element positions and sizes, edge waypoints, label anchors, port badge sides.
Generated by `layout`, and editable. Pinned values survive regeneration.

### Overlay: what is happening

```json
{"element": "app-pods", "state": "degraded", "since": "2026-09-26T10:02:00Z",
 "source": "argocd", "links": [{"title": "Logs", "url": "..."}]}
```

### Adapter output: a model fragment with provenance

Same schema as the model, plus `provenance: {source, ref, observedAt}` on every item,
so the drift report can say where a fact comes from.

## Interfaces

| Interface | For | Role |
|---|---|---|
| CLI `noodle` | humans, CI | deterministic core: lint, render, diff, discover |
| Agent skill | AI authoring | the conventions to write good models and views |
| MCP server | AI agents | query the model, find paths, validate, propose patches |
| Web viewer | humans | explore, present, review, operate |
| CI action | PRs | lint, render, diff, publish the preview link |

These are not alternatives: the same model serves all of them.

## Storage and security

- **Git** holds models, views and layouts. Everything else is generated in CI.
- **Previews** are static builds per PR, behind access control such as Cloudflare
  Access or SSO. An architecture map is sensitive: it lists hostnames, ports and known
  gaps.
- **Public exports** go through a redaction profile. Nothing is published unredacted.
- **Live cockpit (L5)** runs next to the clusters with read-only service accounts and
  SSO. Credentials never reach the browser.
- **History of live data** needs a store beyond Git. To be decided in L5.

## Technology, tentative

- Core, CLI, adapters and MCP in **Go**.
- Viewer in **TypeScript**, rendering SVG.
- Auto-layout with **ELK** (layered algorithm, ports, orthogonal routing, hierarchy),
  which matches the port and side rules of ADR-0003. Where it runs, build time or in
  the viewer, is an open ADR.
- draw.io stays an export target, not the engine.

## Target repository layout

```
.claude-plugin/        Claude Code plugin manifest and marketplace
.opencode/             OpenCode subagent definitions
.claude/skills/        shared skill discovery for Claude Code and OpenCode
bin/noodle             wrapper: finds or builds the binary
cmd/noodle/            CLI
assets/icons/          icons compiled into the binary
internal/model/        schema, merge, validation
internal/lint/         rules
internal/render/...    drawio, svg, png
internal/adapter/...   k8s, kubectl, hubble, terraform
viewer/                TypeScript web viewer
schemas/               JSON Schemas of every contract, versioned
docs/                  vision, architecture, roadmap, ADRs
examples/              reference models
skills/                shared agent skills
```
