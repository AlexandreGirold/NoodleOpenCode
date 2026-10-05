---
name: noodle-diagram
description: Create or update an architecture diagram with noodle, in the house style (Tailwind dark/light palettes, official logos, port badges, lint-enforced readability, draw.io + PNG output). Use when asked for an architecture, flow, deployment or security diagram of a system, or when a change to a system should be reflected in an existing diagram.
---

# Architecture diagrams with noodle

Diagrams are generated, not drawn. The source of truth is a system directory: one
`model.yaml` (what exists), one file per view in `views/` (what a diagram shows) and, for
topology views, one layout in `layouts/` (where it is drawn). The format is in
[reference.md](reference.md): read it before writing one. The `noodle` command turns a
view into a `.drawio` file and refuses to emit it
while the lint finds a collision. Hand edits in draw.io are for throwaway variants only;
a canonical change goes through the files so the palette, the IDs and the PR diff stay
consistent. A project still on the old single-file format converts each file once with
`noodle migrate <old>.yaml <system-dir>`, which refuses to write a system that would draw
anything different.

Claude Code puts `noodle` on PATH. In this source checkout with OpenCode, use
`./bin/noodle` instead; a separately installed `noodle` on PATH also works.

## Workflow

1. **Facts first, in a subagent.** Hand the question and the code paths to the
   `fact-finder` agent (`noodle:fact-finder` in Claude Code's installed plugin). It reads
   the code that implements what the diagram shows
   (Helm templates, Terraform, K8s manifests), not only `docs/`, and returns one YAML
   inventory with a source on every fact. Work from that inventory: reading the raw
   files yourself fills this session with YAML that every later turn pays for again.
   Open a file only to settle an `unknowns` entry or an `inferred` fact the diagram
   depends on. When code and docs disagree, draw the code, add an annotation `Gx`
   targeting the element and an entry in the "Gaps" card.
2. **One question per diagram.** Write it as the subtitle. Anything that does not help
   answer it goes to "Out of scope" in `meta`.
3. **Write the system in English** (proper nouns aside), next to the project's docs,
   for example `architecture/<system>/`. Extend the existing `model.yaml` rather than
   starting a second one: a new diagram is usually a new view of the same model. Check
   the icons first with `noodle -list-icons`. A missing logo goes in the project, e.g.
   `.noodle/icons/<name>.svg`, passed with `-icons .noodle/icons`.
4. **Check and lint, then build both themes:**
   ```bash
   S=architecture/<system>; V=<view-id>
   noodle render $S -view $V -icons .noodle/icons       # check and lint only; fix every finding
   B=architecture/diagrams/$V
   for t in dark light; do
     noodle render $S -view $V -icons .noodle/icons -theme $t -o $B.$t.drawio
     drawio -x -f png -s 2 --border 20 -o $B.$t.png $B.$t.drawio
   done
   ```
   `-slide` renders the drawing only, cropped to its content, for a slide deck.
   `drawio` must be installed for PNG and SVG exports. If it is missing, deliver the
   `.drawio` files and say so.
5. **Look at the PNG.** The lint catches geometry, not taste. Expect two or three rounds.
6. For docs, export the editable SVG:
   `drawio -x -f svg --embed-diagram -o $B.$t.drawio.svg $B.$t.drawio`.
   The PNG is the one to paste in chats.

## Semantics

- **An arrow is a connection, drawn from the side that opens it (client) to the side
  that listens (server).** Traffic carried back over a connection opened the other way,
  such as a tunnel, is said in the label ("[2] relayed via tunnel ◀").
- **Draw requests only, never responses.** When a specific response changes the path
  (302, 401, 500, retry), write it as a note next to the component that sends it
  ("Without a session, Envoy replies 302 to auth…"), not as a return arrow.
- **Topology and sequence are separate diagrams.** A structural diagram numbers only the
  nominal path, with one step per connection and each step used once; the lint rejects
  anything else. A flow that reuses connections or goes back and forth (OIDC login,
  token exchange, retries, sagas) gets its own `type: sequence` view over the same
  model connections. They stay on the topology, unnumbered, with a label pointing to
  that sequence.
- **Egress leaves from the right or bottom border, ingress enters on the left or top.**
  The lint enforces it. `against_flow: true` is the only exception, for outbound
  connections that run against the reading direction: tunnels, agents calling home, a
  hairpin to a public endpoint. Use it knowingly.
- **`port:` names a listening port of the server** (`ports:` on the element), drawn as
  a badge straddling its border, like `TCP 8080` or `UDP 7844`. Client ports are
  ephemeral and are not drawn.
- **Arrow label: `[step] verb · protocol`**, like `[6] proxy · HTTP`, built from the
  connection's `verb` and `protocol` and the view's `steps`. The port is on the badge,
  so it is not repeated in the label.
- **Numbers `[1]…[n]` are the request path (data plane), in time order: the view's
  `steps`. Letters `[A]…` are background work (control plane), its `background`:**
  tunnels, syncs, operators. Two cards list them.
- **A reference is an object reference, not traffic** (parentRef, targetRef, envFrom,
  secret name): `references:` in the model. It is dotted, has an open arrow, carries no
  step and ignores the side rule.
- **A connection that must not happen** is `denied: true`, with `enforced_by` naming
  what blocks it (a NetworkPolicy, a firewall). Without it, it is intent only: say so.
- **Node text is the C4 triptych:** `title` in bold, `tech` in brackets and italics,
  `desc` as one line about the component's responsibility. Technical specifics go in
  the step and gap cards, not in the box.

## Visual language

| Element | Rule |
|---|---|
| Node `kind` | `frontend` cyan · `backend` emerald (services, routing) · `database` violet (data and secrets) · `cloud` amber (edge, tunnels, providers) · `security` rose (identity, policies) · `bus` orange · `external` slate · `tool` slate (IaC, scanners, bots: no traffic) |
| Node `icon` | Official logo when the box is a product (`cloudflare`, `envoy`, `keycloak`, `vault`, `cnpg`, `argo`, `cilium`, `kubernetes`). Kubernetes resource icon (`k8s-svc`, `k8s-deploy`, `k8s-pod`, `k8s-secret`, `k8s-ns`, `k8s-sa`, `k8s-crd`) when the box is a K8s object. CRDs without an official icon use `k8s-crd`, as Argo CD does. |
| Node `shape` | `box` default · `cylinder` for a datastore · `actor` for a human |
| Zone `kind` | `region` = infra or trust perimeter. `group` = functional category, named by role first ("GATEWAY API", "IDENTITY", "SECRETS"), namespace in `sub`. |
| Zone `icon` | Always set. Environment on regions (`globe` internet, `cloudflare` SaaS, `rack` on-prem, a provider logo for a cloud), product or `k8s-ns` on groups. |
| Zone `color` | Sibling zones get different colours so boundaries read at a glance. |
| Edge `kind` | `flow` connection · `auth` authentication · `tunnel` outbound tunnel set up in advance · `async` background sync; `denied: true` draws it blocked; references draw as links |
| `status` | `planned` hatched, with `target` as a pill ("SP2"); `deprecated` dotted and struck. Set it on a zone to apply it to everything inside. |

Palettes are Tailwind v3: 400 strokes on slate-950 for dark, 600 strokes on 50 fills for
light, both checked for WCAG AA text contrast. Colour is never the only carrier: kind is
also shown by icon, dash pattern and legend. Built-in icons: `noodle -list-icons`.
Add a project icon as `<name>.svg` from the CNCF artwork repo, the Kubernetes community
icon set or simple-icons, recoloured with the brand colour. Add `<name>.dark.svg` /
`<name>.light.svg` when one variant disappears on a background. An icon useful beyond one
project belongs upstream in noodle's `assets/icons/`.

## Layout and cable management

- Leave **60–80 px corridors between zones**. Edges travel in corridors, never through
  a zone they do not serve. Inside a zone, 24 px minimum between nodes.
- Positions in the layout are relative to the parent zone: moving a zone moves what it
  holds. Every edge is orthogonal: endpoints `id.side` (or `id.side@NNpx` to pin the
  spot) and waypoints between them. No diagonals. Keep port badges clear of the node
  icon, top-left.
- **Backbones, not spaghetti.** When several edges go the same way, give them parallel
  named `lanes` 15–20 px apart in the same corridor and turn them together, like a PCB
  bus or cable management in a rack. Fan out only at the last bend before each target.
  A lane moved once moves every edge on it.
- Put a label on the longest straight segment, or set `label_at` on the drawn path in a
  clear spot. Never on a bend, a box, a zone title, a port badge or another line.
- A perpendicular crossing is fine: draw.io draws a bridge. Two edges running on top of
  each other are not: the lint rejects it.
- Flow reads left to right: actors → edge/SaaS → cluster → data.

## Grouping and naming

- **Never mix an exploded instance with an aggregate sibling in one group.** To show what
  is allowed or denied between instances, draw two generic instances (`<app-a>`,
  `<app-b>`) side by side, not one detailed app next to an "other apps" box.
- **Group by capability, not by "core" or "misc".** "Managed services" (DBaaS, MQaaS),
  "Observability", "Gateway API". A component that is not deployed yet sits in its
  capability group, marked as planned, never in a "planned" bucket.
- **One group, one role.** Docs and tooling are two groups, not "docs · tooling".
- **Generic names stay explicit:** `<app-a>-backend`, the public URL in the group's
  `sub`, injected variables in labels (`GET $APP_B_URL/api`).

## Check before drawing a flow

- **Client-side calls go through the browser.** A SPA's `fetch()` runs in the browser and
  comes back through the gateway. Before drawing frontend → backend, look for a reverse
  proxy (`proxy_pass`) and read the NetworkPolicy. If neither allows it, draw it as a
  denied flow with a note.
- **Verify live when read access exists** (`kubectl get`, pod spec, policies) and say
  what was verified in `meta`.

## Recipes

**Application runtime view** ("what does my app see?"). Callers left, gateway, the app's
namespace in the centre, reachable dependencies right, a "denied destinations" column
last. Add a `Runtime contract` card: injected env vars, how secrets arrive, sandbox
(user, filesystem, capabilities), endpoints the app must expose, sizing.

**Repository map** ("where do I commit?"). Teams left, the real SCM tree in the centre
(from the provisioning code, not the docs), what reads each repo on the right (GitOps
controller, registry, provisioning targets). One repo per app drawn as a stack. A card
"I want to change… → commit in" is the answer most readers came for.

**Tech stack** (`type: landscape`, "what is the platform built with?"). One band per
concern, top to bottom from what users touch to the foundation: delivery, platform
services, runtime; cross-cutting concerns (observability, identity) in `side`. Sections
are capabilities ("Provisioning", "GitOps"), items are model elements with their
official logo; tools that receive no traffic are `kind: tool`. `flow: true` on the
delivery band reads as a pipeline. Items not deployed yet stay in their section as
`status: planned` with a `target`. No layout to write: the grid is computed.

**Access matrix** (`type: matrix`, "who can touch what, before and after?"). Declare
`memberships` and `grants` in the model; mark what a plan removes `deprecated` and what
it adds `planned`, both with the phase as `target`. Rows by identity type (break-glass,
people, service accounts), columns by access plane (control, management, workloads).
A grant is never an arrow on a topology (ADR-0003).

**Service catalogue** (`type: catalog`, "what can I get, and how?"). One `offering` per
thing a team can ask for (a database, an app slot, SSO): a one-line summary, what it
`provides`, the exact `request` (the value to set, the PR to open), and `backed_by`
listing the elements that deliver it, so the catalogue cannot promise what the model
does not run. Two or three `columns`. No layout to write.

## Required parts

Title with a one-line question, scope and out-of-scope lines, version, source path.
Cards under a topology: `Legend` (`legend: true`, generated from the kinds used),
`Request path · data plane`, `Background · control plane`, and `Gaps between docs and
code` when there are any.

## Stable IDs

IDs are semantic (`envoy-shared-gateway`, `e-4-connector-envoy`) and never renamed for
cosmetic reasons. The PR diff matches elements across versions by ID.
