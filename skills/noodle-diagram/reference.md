# Reference (versioned noodle contracts)

For layout-free architecture docs, start at [v1alpha2](#v1alpha2-docs-first-layout-free-mermaid)
below. The tables first describe the original draw.io presentation contract.

A system is one directory. Unknown fields are rejected; every file carries
`apiVersion: noodle/v1alpha1` and its `kind`. JSON Schemas are in the noodle repository
under `schemas/v1alpha1/`.

```
<system>/
  model.yaml          kind: Model   what exists: elements, connections, references
  views/<id>.yaml     kind: View    what one diagram shows; the file stem is the view id
  layouts/<id>.yaml   kind: Layout  where a topology view draws it; same stem as its view
```

One model, several views: a runtime topology, a login sequence, a tech stack and a
service catalogue can all come from the same `model.yaml`. Only topology views take a
layout; sequence, landscape and catalog views are computed.

```bash
noodle render <system> -view <id>                               # check and lint only
noodle render <system> -view <id> -theme dark -o out.drawio     # then render
noodle render <system> -view <id> -icons .noodle/icons -slide   # project icons; drawing only
noodle migrate old-spec.yaml <system>                            # a v0 single file, once
```

## Model

### Element

Components and zones are both elements. A zone has kind `region` or `group`.

| Field | Notes |
|---|---|
| `id` | semantic and stable, never renamed for cosmetics; unique across the model |
| `kind` | `frontend` `backend` `database` `cloud` `security` `bus` `external` · `tool` (built or run with, receives no traffic: IaC, scanners, bots) · `region` (infra or trust perimeter) · `group` (functional category) |
| `parent` | the zone it sits in; its layout box is relative to that zone |
| `title` | bold name; zones: uppercase, role first (`GATEWAY API`) |
| `tech` | `[Technology · variant]`, shown in italics |
| `desc` | one line: the responsibility |
| `sub` | zones only: small grey text after the title, e.g. the namespace |
| `icon` | see `noodle -list-icons`; project icons via `-icons DIR` |
| `shape` | `box` (default) · `cylinder` for a datastore · `actor` for a human |
| `color` | zones: `cyan` `emerald` `violet` `amber` `rose` `orange` `slate` `indigo` `sky`; siblings differ |
| `ports` | `[{name: https, protocol: TCP, port: 443}]`: listening ports, drawn as badges where connections land |
| `status` | `planned` (hatched) or `deprecated` (dotted, struck title); a zone's applies to its children |
| `target` | `planned` only: when it is expected, e.g. `SP2`, `Q1 2027`; drawn as a pill |
| `multiplicity` | the box stands for many alike, e.g. `one per application`: drawn as a stack, the text in the legend |
| `tags` | free labels, e.g. `risk:G4` |

### Connection: traffic

Opened by `from`, listened to by `to` (ADR-0003). Requests only, never responses.

| Field | Notes |
|---|---|
| `id` | `c-<from>-<to>` or `e-<step>-…`; stable |
| `from`, `to` | element ids; `from` opens the connection |
| `kind` | `flow` · `auth` · `tunnel` (outbound, set up in advance) · `async` (background sync) |
| `port` | a port **name** of `to`; its protocol and number become the badge |
| `protocol`, `verb` | the label reads `verb · protocol`, e.g. `proxy · HTTP` |
| `denied` | `true`: this must not happen, drawn as a blocked edge |
| `enforced_by` | with `denied`: the elements that block it, e.g. a NetworkPolicy (ADR-0010) |

### Reference: configuration, not traffic

`{id, from, to, kind}` with `kind` the relation: `parentRef`, `targetRef`, `envFrom`,
`secretKeyRef`. Drawn dotted with an open arrow; never numbered; the side rule does not
apply.

### Annotation: a gap between docs and code

`{id: G1, severity: high, title: …, text: …, targets: [element or edge ids]}`. Each
target shows the id as a badge; list the gaps in a card.

### Membership and grant: who may act on what (ADR-0019)

Facts, so they live in the model. Neither is traffic: they never draw on a topology.

```yaml
memberships:
  - {id: m-dev, subject: dev-a, group: grp-a-members, auth: {method: oidc, mfa: true}}
grants:
  - {id: g-a, subject: grp-a-members, resource: vault-a, level: read, via: [policy-a-read], status: planned, target: P3}
  - {id: g-cmp-root, subject: cmp, resource: vault-sys, level: admin, via: [root-token],
     auth: {method: static-token}, status: deprecated, target: P2}
```

| Field | Notes |
|---|---|
| `subject`, `group`, `resource` | element ids; identities, groups and mechanisms are ordinary elements |
| `level` | `read` · `write` · `admin` · `breakglass` |
| `scope` | free text, e.g. `project-*` |
| `via` | the elements that give the grant (policy, RoleBinding, IAM role), checked like `enforced_by` |
| `auth` | `{method, lifetime, mfa}`: `static-token` `access-key` `password` `oidc` `k8s-sa` `federated` |
| `status`, `target` | a plan: `deprecated` is what it removes, `planned` what it adds; `target` is the phase, on both |

### Offering: what a platform provides (ADR-0013)

`{id, title, icon, summary, provides: [..], request, backed_by: [element ids], owner,
status, target}`. Read by catalog views.

## View

Common fields: `type` (`topology` · `sequence` · `landscape` · `catalog` · `matrix`), `title`
(`Project · Topic · Question`), `subtitle` (the one question), `meta` (scope, out of
scope, version and source, one line each).

### Topology

| Field | Notes |
|---|---|
| `include` | element ids to show; `z-cluster/**` includes a zone and all it holds; empty shows everything |
| `steps` | connection ids of the nominal path, in time order: they become `[1]…[n]`, each used once |
| `background` | control-plane connection ids: `[A]`, `[B]`… |
| `labels` | `{edge-id: text}` to override a label in this view only |
| `notes` | `[{id, text}]`: a response that changes the path, next to who sends it |
| `cards` | `[{id, title, color, legend, lines}]`: `legend: true` generates the legend |

### Sequence (ADR-0004, ADR-0011)

For flows that reuse connections or go back and forth: login, token exchange, retries.
Each step is exactly one of:

```yaml
steps:
  - {id: get, from: browser, to: edge, over: c-browser-edge, text: "GET /"}   # message over a model connection
  - {note: app, text: "render page"}                                        # note on a participant
  - {reply: get, text: "200"}                                               # reply, drawn dashed
```

A message running against its connection (a relay back through a tunnel) needs a `text`.
`participants: [..]` fixes the column order; otherwise first appearance.

### Landscape: the tech stack (ADR-0012)

Rows of sections of logos, computed; no layout file.

```yaml
type: landscape
bands:
  - {id: delivery, title: Delivery, sub: from code to running platform, color: emerald, flow: true,
     sections: [{title: Provisioning, items: [terraform]}, {title: GitOps, items: [argocd]}]}
side:
  - {title: Observability, color: orange, items: [grafana, loki]}
labels: {cnpg: CloudNativePG}
```

Items are model elements, each shown once; `flow: true` draws arrows between sections.

### Catalog: the service catalogue (ADR-0013)

```yaml
type: catalog
include: [dbaas, app-hosting, sso]   # offering ids
columns: 2                           # 1 to 4
```

### Matrix: who may act on what (ADR-0019)

Computed, no layout. Rows are identities, columns resources, both in titled groups;
`labels` set header text (`<br>` for a second line). In `diff`, a cell's fill is the
level after the plan and its frame and tag the change and its phase. The legend is
generated under the grid.

```yaml
type: matrix
state: diff                  # current (default) · target · diff
identities: [{title: People, sub: "OIDC + MFA", items: [admin, dev-a]}]
resources: [{title: Control plane, color: indigo, items: [keycloak, vault-sys]}]
labels: {vault-sys: "Vault<br>config"}
```

## Layout (topology views)

```yaml
apiVersion: noodle/v1alpha1
kind: Layout
canvas: {width: 2340, height: 1800}
lanes:
  connectors: {x: 1100}              # a shared vertical corridor; {y: …} for a horizontal one
elements:
  z-cluster: {x: 720, y: 200, w: 1580, h: 1080}   # top level: canvas coordinates
  g-gateway: {x: 400, y: 60, w: 400, h: 320}      # child: relative to its parent zone
edges:
  e-1-browser-edge: {from: user.right, to: cf-edge.left}
  e-3-connector-envoy: {from: cloudflared-app.right, to: envoy.left@105px, waypoints: [lane:connectors]}
  e-4-envoy-service: {from: envoy.bottom@280px, to: app-service.top, waypoints: [[1440, 700], [1590, 700]], label_at: [1515, 700]}
notes:
  n-302: {x: 52, y: 590, w: 196, h: 90}
cards:
  card-legend: {x: 40, y: 1340, w: 540, h: 420}
```

- **Endpoints** are `id.side`, `id.side@NN%` or `id.side@NNpx` (offset from the top or
  left of that side). A bare side slides to line up with the next waypoint, so the first
  and last segments stay orthogonal.
- **Waypoints** are canvas points `[x, y]` or `lane:<name>`. Moving a lane moves every
  edge on it: give parallel edges their own lanes 15 to 20 px apart.
- `label_at` `[x, y]` on the drawn path, `label_offset` `[dx, dy]`, `against_flow: true`
  for outbound connections that run against the reading direction.
- Side rule, except references and `against_flow`: leave from the right or bottom of
  `from`, enter on the left or top of `to`.
- Typical box 220–320 × 72–120; actor 56 × 56. Text width is estimated at 0.6 em per
  character: 7.2 px at 12 px (title), 6 px at 10 px (tech, desc); available width is
  `w − 58` with an icon. The lint reports any overflow.

## Text markup

Everywhere text is shown: `[1]` filled step badge, `[A]` outlined step badge, `**bold**`,
`!!warning!!` in orange, `<br>` for a second line in a label.

## Minimal system

`model.yaml`:

```yaml
apiVersion: noodle/v1alpha1
kind: Model
elements:
  - {id: z-internet, kind: region, color: slate, icon: globe, title: INTERNET}
  - {id: z-cluster, kind: region, color: cyan, icon: kubernetes, title: CLUSTER}
  - {id: user, kind: frontend, parent: z-internet, shape: actor, icon: user, title: User, desc: browser}
  - {id: api, kind: backend, parent: z-cluster, icon: k8s-deploy, title: API, tech: "[Deployment]", desc: Serves requests,
     ports: [{name: https, protocol: TCP, port: 443}]}
  - {id: db, kind: database, parent: z-cluster, shape: cylinder, icon: cnpg, title: Postgres, tech: "[CloudNativePG]",
     desc: App data, ports: [{name: pg, protocol: TCP, port: 5432}]}
connections:
  - {id: c-user-api, from: user, to: api, kind: flow, port: https, protocol: HTTPS, verb: GET}
  - {id: c-api-db, from: api, to: db, kind: flow, port: pg, protocol: SQL, verb: query}
```

`views/request.yaml`:

```yaml
apiVersion: noodle/v1alpha1
kind: View
type: topology
title: Demo · Web app · Request path
subtitle: How a request reaches the database.
meta: ["Scope: demo", "v0.1 · source: architecture/demo"]
steps: [c-user-api, c-api-db]
cards:
  - {id: card-legend, title: Legend, color: slate, legend: true}
```

`layouts/request.yaml`:

```yaml
apiVersion: noodle/v1alpha1
kind: Layout
canvas: {width: 1100, height: 820}
elements:
  z-internet: {x: 40, y: 220, w: 220, h: 300}
  z-cluster: {x: 320, y: 220, w: 740, h: 300}
  user: {x: 82, y: 130, w: 56, h: 56}
  api: {x: 60, y: 100, w: 240, h: 80}
  db: {x: 440, y: 90, w: 240, h: 100}
edges:
  c-user-api: {from: user.right, to: api.left}
  c-api-db: {from: api.right, to: db.left}
cards:
  card-legend: {x: 40, y: 560, w: 1020, h: 220}
```

The noodle repository has one example per view type in `examples/`: `cnp-runtime`
(topology and sequence at scale), `sequence-basics`, `landscape`, `catalog`, and
`platform-regression` for large topologies. `examples/project-architecture` provides
a source-backed mixed-stack model, four Mermaid views and an architecture guide.

## v1alpha2 (docs-first, layout-free Mermaid)

Use `apiVersion: noodle/v1alpha2` consistently across model/view/layout; discovered
fragments stay v1alpha1. The Model/View/Layout schemas live in `schemas/v1alpha2/`.
Mermaid supports topology and landscape; topology takes no coordinates. draw.io still
requires a valid topology layout. Its original geometric lint is unchanged.

Mermaid uses quoted component labels, actual selected containment, visible technology
and responsibility, and known target transport/port in edge labels. It preserves
connection kinds and denial intent/enforcement. References are dotted, not traffic.
Landscape reading-order arrows are explicitly labelled, never runtime connections.
Automatic layout does not guarantee draw.io's orthogonal routing, endpoint sides,
border badges, official icons or collision checks: render and inspect the result.
Sequence/catalog/matrix views and view cards/notes return explicit unsupported errors
for Mermaid. Use the existing draw.io output for those views.

### Evidence

The model has an `evidence` map keyed by every element, connection and reference ID:

```yaml
apiVersion: noodle/v1alpha2
kind: Model
elements:
  - {id: api, kind: backend, title: API, tech: Go}
evidence:
  api:
    how: code
    sources: [{file: src/api/main.go, line: 12}]
  # An unknown fact instead uses: {how: unknown, note: Explain what is missing.}
```

`code` read in manifests/source, `config` resolved from values/env, `inferred` a guess
(such as a URL in an env var, edges marked uncertain), `docs` only in documentation,
`unknown` when not determinable. Non-unknown entries need one or more sources. A source
has either `file` (relative to the system directory) and optional nonnegative `line`,
or `object`, never both. Unknown entries require a nonempty `note`. No orphan evidence
keys are allowed. Uncertain relationships should normally be questions/annotations on
known components rather than fabricated connections. Evidence classes appear on rendered
nodes/edges; detailed sources are retained in safe Mermaid comments and the guide.

### CLI

```bash
noodle render DIR --view ID --format mermaid -o view.mmd   # -o omitted = validation only
noodle upgrade SRC DEST                                  # v1alpha1 directory -> new v1alpha2 directory
```

Upgrade preserves facts, views, layouts and discovered fragments. The destination must
not exist; missing legacy evidence becomes unknown, never a manufactured citation.
For a v0 single file, first use `noodle migrate SPEC.yaml DIR`.

### Whole-project analysis and updates

Read relevant manifests, entrypoints, runtime clients, service URLs, proxies, Compose,
IaC and delivery configuration. Manifests identify technologies; clients/config establish
intended relationships. Code/config wins over docs. Use existing k8s fragments as evidence
for the subset they cover, not as proof that every connection was discovered. Keep local,
staging and production endpoints distinct. A logical service can link to discovered
resource IDs through `matches`; deploys/manages relationships are references, not traffic.

Keep one source model and cheap question-specific views. Changing a shared component's
technology updates every generated perspective after regeneration. Keep source-relative
citations current and explain assumptions/unknowns in the adjacent guide.

### Presentation rules retained for draw.io

- Use official product logos (`noodle -list-icons`); add project SVGs under `.noodle/icons`
  and pass `-icons DIR`. Use Tailwind house palettes and different sibling zone colours.
- Nodes use title, technology and one-line responsibility. Region = infrastructure/trust
  perimeter; group = capability. Never mix one exploded instance with aggregate siblings.
- Connections leave right/bottom and enter left/top; use `against_flow` for explicit
  outbound tunnels/hairpins. The listening port is on the server border, never the client.
- Topology: one nominal step per connection; background work uses letters. Responses
  that change a path are notes. Repeated exchanges belong in sequence views.
- `denied` without `enforced_by` is intent, not proof of enforcement. Describe it honestly.
- Keep 60–80 px zone corridors and at least 24 px between nodes. Use orthogonal routes,
  reusable named lanes and zone-relative boxes. Labels belong on clear straight segments.
- Include title/question, scope, source and version metadata; add the legend, request path,
  background work and any gaps cards. Planned/deprecated status and target stay in the model.
- Build both themes, export PNG and look at it. draw.io is required for its image export;
  deliver `.drawio` and report the missing exporter if unavailable. SVG export:
  `drawio -x -f svg --embed-diagram -o out.drawio.svg out.drawio`.
