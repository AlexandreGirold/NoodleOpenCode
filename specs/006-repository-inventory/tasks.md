# Tasks 006

- [x] Inventory report, scanner and recognized-format detection
- [x] Kubernetes reuse, Compose and dependency metadata extraction
- [x] CLI and schema/behavior/determinism tests
- [x] Required regression checks and real-repository smoke scan
- [x] Repository/global skill and documentation integration

## Verification · 2026-10-22

- `go vet ./...` and `go test ./...`: passed, including schema, source evidence,
  repeatability, distinct root/folder scopes, dynamic/invalid ports, malformed and
  cyclic YAML, credential omission, ignored inputs and atomic CLI failure handling.
- `go test ./cmd/noodle/... -run TestScanCommand`: passed.
- `scripts/init.sh`: passed all Go, existing diagram and generated-Mermaid regressions;
  Claude CLI unavailable, so its plugin validation was skipped.
- `opencode debug skill` and `opencode debug agent fact-finder`: passed.
- Both `go run ./cmd/noodle scan` and `bin/noodle scan` on the mixed-stack example:
  identical output; 4 inputs, 9 elements, 1 connection, 2 dependency manifests.
- Noodle self-scan: 13 recognized inputs, 14 elements and 5 explicit gaps.
- Two-root CNP/GitOps scan and reversed-root repeat: byte-identical (`cmp`), 213 inputs,
  128 unresolved entries. Their raw Helm/Kustomize sources correctly require rendering;
  no speculative deployed components or connections are emitted.
- Byte-based estimates on that pair: ~365.3k input tokens vs ~14.8k report tokens.
  Small example: ~574 input vs ~1.1k report, so savings are not universal. These are
  payload estimates, not measured LLM usage or proof of architectural completeness.
- Repository and installed global skill now scan first and investigate only relevant gaps.
  No persistent cache, additional parser promises or live connectivity checks are claimed.
