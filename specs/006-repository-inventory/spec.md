# Spec 006 · Deterministic repository inventory and fact extraction

- Status: Implemented and verified locally
- Requested: 2026-10-22; reusable discovery/extraction in the repository and skill
- Decision: ADR-0022

## Goal

Reduce source-reading tokens before architecture curation. `noodle scan ROOT… [-o FILE]`
produces a deterministic, source-backed inventory without an LLM, network access,
dependency installation, project execution or edits to the curated model.

## Requirements

- Inventory recognized deployment, dependency, CI and infrastructure files across
  multiple roots. Include normalized paths, formats, processing status and SHA-256.
- Reuse Kubernetes discovery for standalone/rendered manifests. Report Helm/Kustomize
  inputs as requiring rendering; do not run them or treat raw templates as deployments.
  Keep manifest directories as separate extraction scopes to avoid merging unrelated
  environments with identical resource names; shared deployment context remains curated.
- Extract Compose services, literal image technology, container-side TCP/UDP ports and
  `depends_on` configuration references. Dependencies never imply runtime traffic.
- Extract dependency names/scopes from package.json and requirements files as metadata,
  not individual runtime services. Do not emit version/URL/script/environment values.
- Detect other known inputs (including Terraform, Python/Go/Rust manifests, Dockerfiles
  and CI); report parser gaps explicitly. No speculative arbitrary-code analysis.
- Every fact has a source file/line. Interpolation, unsupported port forms, includes,
  invalid inputs and parser gaps remain unresolved, without raw secret-bearing values.
- Ignore credential/state files, generated/dependency directories and symlinks. Bound
  input size with an explicit unresolved result rather than silently dropping it.
- Stable ordering, namespaced IDs and no implicit clock/environment inputs. Same roots
  and contents produce identical output; hashes support agent-side incremental comparison.
- Keep existing Model/View/Fragment contracts and discovery behavior unchanged. A
  separately versioned Inventory report is an investigation aid, not an architecture model.

## Non-goals

Automatic model curation, a persistent cache engine, rendering project inputs, package
installation, Terraform state reads, live reachability or modifications to the viewer.

## Acceptance

Focused Go/CLI tests cover extraction semantics, multiple roots, deterministic reruns,
source lines, ignored files/symlinks, malformed/oversized/dynamic inputs, secret omission,
Kubernetes reuse and failures without partial output. Existing Go and smoke checks pass.
The repository and installed skill run scan first and reserve source analysis for gaps.
