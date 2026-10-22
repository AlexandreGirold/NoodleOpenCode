# Tasks

- [x] T01: Branch, baseline smoke check, ADR and spec/plan/tasks.
- [x] T02: Versioned model, evidence, semantic validation and upgrade tests.
- [x] T03: Mermaid topology/landscape renderer and semantic regression tests.
- [x] T04: CLI format dispatch and non-overwriting upgrade command.
- [x] T05: Mixed-stack sources, authoritative model, four views and guide.
- [x] T06: Concise skill, detailed reference, fact-finder and README.
- [x] T07: Automated regression, Mermaid rendering and visual review.
- [x] T08: Independent review, acceptance flags and progress handoff.

## Verification evidence (2026-10-22)

- `go test ./...`, `go vet ./...`, `./scripts/init.sh` and generated-fixture
  reproducibility checks passed during integration and in the final post-review rerun.
- `opencode debug skill` resolves the shortened project skill; `opencode debug agent
  fact-finder` resolves the native read-only profile. Claude CLI is absent, so Claude
  plugin validation could not run. GitHub CLI is absent; issue/board entry is pending.
- Free Beam subagents supplied implementation drafts and an independent source-only
  inventory following the short skill/fact-finder workflow. The hub reconciled the
  inventory with the authoritative model, corrected fixture/schema errors, and generated
  the four views through the CLI. The inventory independently confirmed browser→nginx,
  nginx→API:8000, API→PostgreSQL:5432, API/worker→Redis:6379 and worker→external HTTPS.
  Missing production gateway/database/Redis provisioning and external reachability
  remain explicit limitations rather than invented facts.
- GPT-6.1 Sol **high** independently reviewed code and opened all four rendered PNGs.
  Initial findings led to regression tests for version aliases, evidence-schema/runtime
  parity and visible view context. Follow-up caught aliased evidence values and Mermaid
  line-hop styling, both corrected. Final SOL high source/PNG inspection confirmed
  both remaining findings resolved with no unresolved actionable findings in that check.
- Rendering used an isolated pnpm installation under `/tmp/opencode/mermaid-tools`,
  Mermaid CLI 12.0.0 / Mermaid 12.2.0. All four `.mmd` files parsed and rendered to PNG.
  The hub inspected all four plus a light-theme communication render. Wider labels,
  ordered landscape rows and disabled line hops preserve readability/reference styles.
- Example application dependencies were not installed or deployed: this verifies the
  source-backed documentation workflow, not production runtime readiness.
