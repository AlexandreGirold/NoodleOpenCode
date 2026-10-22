---
name: noodle-diagram
description: "Explicit-call-only. Use ONLY when explicitly asked for noodle-diagram or Noodle diagramming. Map project architecture from deterministic inventory and sourced facts, with bounded LLM analysis. Use a dark-only interactive explorer when available; generate Mermaid/draw.io as requested."
---

# Project architecture with noodle

Maintain one model; generate diagrams from it. Map meaningful components and their
interactions, not every file or library. Load [reference.md](reference.md) only for needed
contract details. Use `./bin/noodle` here; installed plugins use `noodle`.
Mentioning or editing this skill does not invoke its workflow. Future visualizations are
dark-only: no white theme, light-mode toggle or automatic light exports. Reuse an existing
interactive viewer rather than building a new website for each project.

## Workflow

1. **Scope:** identify the project roots, environments and architectural questions.
   Cover applications, data, external services and infrastructure; Kubernetes is one source.
2. **Scan before LLM analysis:** run `noodle scan ROOT… -o inventory.yaml`; see
   [inventory.md](inventory.md). Reuse its extracted facts, source locations and file
   fingerprints. Read only relevant unresolved/unsupported source, not every manifest.
   No source reanalysis for a theme/view/open-only request. If substantial gaps warrant
   delegation, give one focused `fact-finder` the report and exact questions; do not
   duplicate its work. No default reviewer agent. Never copy secrets or invent traffic.
3. **Curate:** extend the shared `model.yaml`, keeping stable IDs. Put one question in
   each view's subtitle. For whole-project docs, create overview, stack, communication
   and deployment views plus an adjacent Markdown guide; omit irrelevant detail.
4. **Validate and generate:** use the appropriate recipe below. Fix semantic errors
   in the facts, not by weakening lint. Update source facts before regenerating outputs.
5. **Check and deliver:** inspect one representative render before batch export. Test
   affected behavior, fix actual defects and rerender affected output only. Open an
   existing ready explorer in Firefox once, in the background. Reuse the server; report
   its actual URL. Stop when checks pass; no automatic polish/review or export loop.

## Mermaid documentation

Use `noodle/v1alpha2` for the model and views. Every element, connection and reference
has an entry in the model's `evidence` map; see the exact example in the reference.
Topology and landscape are supported, without manual coordinates. Use model annotations
for visible caveats; view cards/notes and sequence/catalog/matrix are not supported here.

```bash
noodle render architecture/SYSTEM --view overview --format mermaid -o overview.mmd
```

Omit `-o` to validate only. Upgrade an existing **v1alpha1 directory** with
`noodle upgrade SOURCE_DIR NEW_DIR`; fill in its initially unknown evidence.
See the [mixed-stack example](../../examples/project-architecture/README.md).

## draw.io and presentation output

Existing v1alpha1 systems remain supported. Topology needs an authored layout;
sequence, landscape, catalog and matrix use computed geometry. A v1alpha2 system
also needs valid layouts when rendering topology to draw.io.

```bash
noodle render architecture/SYSTEM --view runtime -theme dark -o runtime.drawio
drawio -x -f png -s 2 --border 20 -o runtime.png runtime.drawio
```

Render the requested dark outputs and inspect them. Use official logos and house
style; the reference covers icons, port badges, layout and export details.

## Essential semantics

Connections point from client to listener, not in the direction data happens to move.
Keep configuration/provisioning references distinct from traffic. Packages, credentials
and missing policies alone do not prove a connection or its reachability. Separate
logical roles from environment-specific deployment resources. Preserve explicit
inference and unknowns; never invent ports. Keep repeated flows in a sequence view.
