# Implementation plan

1. Freeze ADR-0021 and spec 005; preserve existing contracts and user work.
2. Extend `internal/model` and versioned schemas with evidence, semantic validation,
   compatibility and a non-overwriting system upgrade.
3. Implement `internal/render/mermaid` against model contracts. Add explicit format
   selection and an upgrade command at the CLI boundary.
4. Add a source-backed mixed-stack example and four generated documentation views.
5. Shorten the shared skill, extend its reference and fact-finder for broad projects,
   and document initial generation and updates.
6. Run focused tests, all repository checks, native Mermaid rendering and visual
   review. Record exact acceptance/progress; no unverified feature flips.

Subagents receive bounded, non-overlapping assignments and compact context. Availability
probe deadlines never become time limits on agents actively implementing work.
