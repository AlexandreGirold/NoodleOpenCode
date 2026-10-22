# Spec 005 · Project architecture understanding with Mermaid

- Status: Implemented and verified locally (tooling gaps recorded in tasks.md)
- Decision: ADR-0021; approved 2026-10-22
- Branch: `feat/project-architecture-mermaid`

## What and why

Map a project's technologies, major services, infrastructure, deployment and
communication. A shared source-backed model generates Mermaid documentation, with an
interactive explorer specified later. File paths are evidence, not the map's nodes.

## Requirements

- FR-001: v1alpha2 Model/View/Layout contracts, durable per-fact evidence and an explicit
  non-overwriting upgrade from v1alpha1. Read existing models and discovered fragments.
- FR-002: semantic validation independent of draw.io geometry for v1alpha2. Reject broken
  references, parent cycles, invalid ports, invalid evidence and inconsistent versions.
  Preserve v1alpha1 behavior and existing draw.io lint.
- FR-003: `noodle render DIR --view ID --format mermaid [-o FILE]` validates and generates
  deterministic topology or landscape Mermaid without manual coordinates. No `-o`
  validates only, matching existing render behavior. Default output remains draw.io.
- FR-004: preserve identity, real containment, responsibility, technology, communication
  direction, known listening ports, reference kinds, evidence classifications and
  applicable lifecycle/denial semantics. Never silently discard unsupported features.
- FR-005: a concise agent skill guides source investigation, discovery reuse, model
  curation, view generation and visual review. Details live in the reference. Sources
  and unknowns survive handoff. Do not expose secrets or infer traffic from packages.
- FR-006: a mixed-stack example includes application code/configuration, Compose,
  Kubernetes, a datastore, a broker/worker and an external dependency. Overview, stack,
  communication and deployment views share facts. Environment boundaries stay explicit.
- FR-007: an adjacent guide explains source locations, important interactions, uncertainty,
  generation and updating shared facts. Agent-assisted analysis is verified against input.

## Acceptance

Use the new P005 feature entries in `docs/features.json`. Verify schema/upgrade behavior,
semantic errors, renderer escaping and determinism, CLI regressions, mixed-stack source
accuracy, Mermaid parsing/rendering and visual readability. Existing smoke tests pass.

## Follow-on

The explorer will consume the authoritative model, not parse generated Mermaid. Browser
search, focus, filters and evidence navigation require their own spec and UI review.
