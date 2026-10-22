# ADR-0022 · Inventory before architecture interpretation

- Status: Accepted
- Date: 2026-10-22
- Origin: user-requested repository inventory and declarative extraction, spec 006

## Decision

Add `noodle scan` as a read-only, deterministic preflight. Its independently versioned
`noodle.inventory/v1alpha1` Inventory report lists recognized inputs, fingerprints,
extracted architectural declarations, dependency metadata and unresolved source locations.
This is a new report contract; existing Model/View/Fragment schemas are unchanged.

Reuse the Kubernetes adapter rather than implementing another Kubernetes parser.
Manifest directories are separate extraction scopes, not asserted deployment environments;
the scanner must not combine identically named resources across unrelated folders.
Compose dependencies are references, not traffic. Library dependencies stay metadata,
not service nodes. Dynamic/unsupported values are unresolved; never evaluate project
code, environment files, templates or remote sources during scanning.

The coding agent curates the existing shared model from the compact report and reads
source only to resolve meaningful gaps. File fingerprints support reuse but this slice
does not implement a persistent extraction cache or claim arbitrary-project completeness.
