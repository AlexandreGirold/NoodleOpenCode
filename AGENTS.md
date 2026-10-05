# Working on noodle

## Session protocol

Every session starts without memory of the previous one. The repository carries the state;
Claude Code hooks add convenience checks, and OpenCode reads this file natively.

**Start.** In Claude Code, the `SessionStart` hook injects "Start here", the next
features and the git state. Run `./scripts/init.sh` and fix anything red before new work.

**Work, one feature at a time.** New work on a phase starts from a spec:
`specs/NNN-name/spec.md` (what and why), then `plan.md`, then `tasks.md`. Ask before choices
the spec leaves open, and record the answer in an ADR.

**After each verified feature, not only at the end:**
1. flip its `passes` to `true` in `docs/features.json`, only if you ran its `verify` step;
   never edit or delete an entry, `scripts/check-features.sh` and CI reject it;
2. add a line under today's entry in `docs/PROGRESS.md`;
3. commit with a conventional message.

The `Stop` hook blocks ending a turn when files changed but `docs/PROGRESS.md` did not.
A one-line "in progress" note satisfies it.

**End.** In Claude Code, the `Stop` hook blocks ending a turn when files changed but
`docs/PROGRESS.md` did not. In either host, when Brian stops ("je reprends demain",
"on s'arrête là"), run the `wrap-up` skill.

## Backlog

Planning lives on the [GitHub project board](https://github.com/users/TheGostsniperfr/projects/5)
(Idea, To Do, In Progress, Done), one issue per item. Read it with
`gh project item-list 5 --owner TheGostsniperfr`. The board says what to work on next;
`docs/features.json` says what is verified. A backlog id `B-NN` names the same item in
both: put it in the issue body. Close the issue in the PR that flips its `passes`
(`Closes #NN`), and open an issue for any new backlog item or spec.

## Model guidance

The main session model is Brian's choice (`/model`). Subagents pin theirs.

| Work | Model |
|---|---|
| Specs, ADRs, architecture, open questions, anything that sets direction | Opus, high effort |
| Implementing tasks from a `tasks.md` that is already clear | Sonnet |
| Collecting a diagram's facts from code (skill step 1) | `noodle:fact-finder` in Claude Code, `fact-finder` in OpenCode (Sonnet/default model) |
| Reviewing a rendered diagram as a tech lead | `noodle:diagram-reviewer` in Claude Code, `diagram-reviewer` in OpenCode (Opus/default model) |
| Mechanical work: fetching and recolouring logos, variants | `noodle:icon-curator` in Claude Code, `icon-curator` in OpenCode (Haiku/default model) |

## Read first

`docs/VISION.md`, `docs/ARCHITECTURE.md`, `docs/ROADMAP.md` and the ADRs in `docs/adr/`.
They are the source of truth for scope and design. The current phase is the first
unchecked one in the roadmap.

## Rules

- **Contracts are the boundary.** A module depends on schemas, never on another
  module's internals. Changing a contract needs a new ADR and a schema version bump.
- **Decisions go in ADRs.** A choice that a future reader would ask "why?" about gets an
  ADR. Never rewrite an accepted ADR; supersede it.
- **Stay in the current phase.** Do not build ahead of the roadmap without asking.
- **Semantics are fixed by ADR-0003 and ADR-0004.** Do not relax a lint rule to make a
  diagram pass; fix the diagram.
- Go: `gofmt`, `go vet`, errors wrapped with context, table-driven tests with testify.
- Everything in English: code, docs, diagrams.

## Layout

The repository root ships the Claude Code plugin (`.claude-plugin/`) and OpenCode project
configuration (`.opencode/`). `bin/noodle` is the wrapper users run; `assets/icons/` is
compiled into the binary. `.claude/skills/noodle-diagram` exposes the shared skill;
`agents/` serves Claude Code and `.opencode/agents/` serves OpenCode. `.claude/skills/wrap-up`
and the Claude hooks are for working on this repo only.
Dependencies are vendored: run `go mod vendor` after touching `go.mod`.

## Check before handing back

```bash
go vet ./... && go test ./...
go run ./cmd/noodle render examples/cnp-runtime --view runtime
go run ./cmd/noodle render examples/cnp-runtime --view oidc-login
claude plugin validate .
opencode debug skill
opencode debug agent fact-finder
```

For diagram work, render the PNG and look at it. The lint catches geometry, not taste.

## Show it in the PR

noodle is a drawing tool, so review is visual. A PR that changes what noodle draws puts
a before/after in its description (`.github/pull_request_template.md`): the PNG from
`main` next to the one from the branch, cropped on the change, linked by commit SHA
(`https://github.com/TheGostsniperfr/Noodle/blob/<sha>/<path>.png?raw=true`). Render
the example that exercises the change, or add one. A PR with no visual change says so.
A design decision with no rendering yet (an ADR, a plan) can be illustrated by a noodle
diagram in `docs/diagrams/`.
