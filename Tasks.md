# Tasks: fastread

_Generated from Plan.md v1. This file is the loop's persistent state (status and notes
only): update it in place, never regenerate it during a build. Briefs live in `tasks/<ID>.md`,
verifications in `scripts/verify/<ID>.sh`._

## Standing constraints

The supervisor prepends these, verbatim, to every worker brief:

- Repository root: `/home/sa/workspace/code/fastread`, Go module `github.com/0x6c6d/fastread`.
  The check command must pass for the whole repo when you finish (it is also defined as the
  shell function `check` in `scripts/verify/_lib.sh`):
  `test -z "$(gofmt -l .)" && go vet ./... && go vet -tags nogui ./... && go vet -tags e2e ./... && go test ./... && go test -race ./... && go test -tags nogui ./... && go build ./... && go build -tags nogui ./... && CGO_ENABLED=0 go build -tags nogui ./...`
- Protected (never modify, create, delete or stage): `README.md` (the spec), `Plan.md`,
  `Tasks.md`, `tasks/`, `scripts/verify/`, `prompts/`, `.claude/`, `Improvements.md`,
  `scripts/check-protected.sh`, `scripts/protected.sha256`, all of `CLAUDE.md` (its
  "Delegation workflow" section is hash-protected; the rest is frozen for v1), and the user's
  editor swap file `.README.md.swp`. `scripts/check-protected.sh` must keep exiting 0.
- Write only the files your brief names (plus `go.mod`/`go.sum` when it adds a dependency).
  Usage documentation goes under `docs/`, never into `README.md`.
- No commits, no `git add`/`stash`/`reset`/`checkout`; the supervisor commits after verifying.
- Toolchain: run every `go` command with `GOTOOLCHAIN=local` (installed Go 1.24.4). `go.mod`
  keeps `go 1.24.x` and no toolchain newer than go1.24. Add dependencies only with
  `GOTOOLCHAIN=local go get <module>@<pinned version>`, never `@latest`. Pins: `gioui.org v0.10.3`,
  `golang.org/x/term v0.40.0`, `golang.org/x/sys v0.41.0`, `golang.org/x/net v0.50.0`,
  `golang.org/x/image v0.36.0`, `golang.org/x/text v0.34.0` (indirect),
  `github.com/rivo/uniseg v0.4.7`, `github.com/ledongthuc/pdf v0.0.0-20260907135840-6c8c28e0e8a0`.
  No other third-party module.
- Network: only the Go module proxy, and only via `go get`/`go mod download` of the pins above.
  No network code in the program or its tests (no `net`, no `net/http`); no downloads of
  fixtures; no web lookups needed (module sources are in `$(go env GOMODCACHE)`).
- No global or system installs (`sudo`, `apt`, `go install`, `pip`, `npm -g`, …); if a tool or
  library is missing, stop and report it.
- Live systems: only processes you start yourself and stop before finishing (local `tmux`
  servers on their own `-L` socket, `Xvfb` on a free display). Tests use a temporary
  `XDG_STATE_HOME` and never write to the real `~/.local/state`; never touch the user's tmux
  sessions or X display.
- No secrets, tokens or personal data in any file.
- Layering (R30): `internal/{input,tokenize,orp,timing,state}` import no UI package;
  `gioui.org` is imported only under `internal/gui/`. Tests use stdlib `testing` only and are
  table-driven where cases repeat.
- Every command that could hang runs under a timeout; leave no background process running.
- End your report with one line `Loop note: <friction>` if the process itself got in your way.

## Task List

Batches: 1 = phases 0-1 (T000-T012); 2 = phase 2 (T013-T026); next free ID: T027

| ID | Phase | Title | Depends-on | Model | Status | Notes |
|----|-------|-------|------------|-------|--------|-------|
| T000 | 0 | Preflight | — | supervisor | todo | at generation all env checks passed (x11-xcb now present); only the clean-tree check fails until batch 1 is committed |
| T001 | 1 | Go module and timing skeleton | T000 | opus | todo | |
| T002 | 1 | Tokenizer skeleton | T001 | haiku | todo | |
| T003 | 1 | ORP focus index skeleton (uniseg pin) | T001 | opus | todo | |
| T004 | 1 | Playback model skeleton (state.Player) | T001, T002 | sonnet | todo | |
| T005 | 1 | Input skeleton (selection, raw, stdin) | T001 | opus | todo | |
| T006 | 1 | TUI frame renderer and ANSI encoder skeleton | T003 | sonnet | todo | |
| T007 | 1 | TUI runtime skeleton (terminal, alt screen, loop) | T004, T006 | opus | todo | |
| T008 | 1 | GUI skeleton (Gio window, nogui stub) | T004 | opus | todo | |
| T009 | 1 | CLI flags, usage, help and version | T001 | opus | todo | |
| T010 | 1 | CLI wiring (input to TUI/GUI) | T002, T005, T007, T008, T009 | opus | todo | |
| T011 | 1 | E2E harness and TestE2EBasic (tmux) | T010 | sonnet | todo | also Phase 1 exit criterion |
| T012 | 1 | MIT LICENSE file | T000 | opus | todo | |
| T013 | 2 | Hardened input helpers (size caps, zip, XML, recover) | T005 | opus | todo | |
| T014 | 2 | Source selection with the path-like rule (A3) | T005, T010 | opus | todo | |
| T015 | 2 | File type detection, file loading and loader stubs | T010, T013 | opus | todo | |
| T016 | 2 | Markdown stripper | T015 | sonnet | todo | |
| T017 | 2 | XHTML to paragraphs (x/net/html pin) | T013 | opus | todo | |
| T018 | 2 | EPUB loader (container, OPF spine, zip-entry resolution) | T015, T017 | opus | todo | |
| T019 | 2 | EPUB DRM detection (encryption.xml) | T018 | opus | todo | |
| T020 | 2 | FB2 loader (body text only) | T015 | sonnet | todo | |
| T021 | 2 | PDF text-layer loader (ledongthuc/pdf pin) | T015 | opus | todo | |
| T022 | 2 | PDF hostile-input suite | T021 | opus | todo | |
| T023 | 2 | Flag validation table and help/version tests | T010 | opus | todo | |
| T024 | 2 | Typed-error chain and exit-code mapping | T014, T015, T018, T021 | opus | todo | |
| T025 | 2 | Performance test 5 MB load and tokenize | T015 | sonnet | todo | |
| T026 | 2 | Phase 2 gate (fixtures through the CLI flow) | T016, T019, T020, T022, T023, T024, T025 | sonnet | todo | also Phase 2 exit criterion |

Status values: `todo` · `done` · `blocked` · `superseded`. No pipe characters inside cells.

## Execution Protocol

Run with the `loop-coding` skill (`/loop-coding Tasks.md`). The authoritative procedure
(round budget, escalation, commit format, resume, Definition-of-Done repair) is in
`.claude/skills/loop-coding/SKILL.md`; in short, per iteration:
1. Select the first `todo` row whose dependencies are all `done`.
2. Read `tasks/<ID>.md`; dispatch one fresh worker with the row's `Model` and a brief made of the
   Standing constraints, the task's Brief and the path of its verification script.
3. The supervisor runs `scripts/verify/<ID>.sh` itself (incl. its negative checks); never trust
   the worker's claim.
4. Pass → one commit with the work and this file's update. Fail → feedback, then escalation
   one tier up, then `blocked`.
5. Nothing selectable → run Plan.md's Global Definition of Done; done when it passes.
6. Friction in the loop itself → one row in `Improvements.md` (see SKILL.md, *Improvement log*); no round, no block.

## Changelog

- 2026-10-08, batch 2 (Plan.md v1, phase 2): added T013-T026 as `todo`. Reviewed the orphaned
  drafts T013-T016 from an interrupted run (kept IDs): T015 now also depends on T010 (its
  script runs the wired binary), T013's negative-check label corrected, T013-T016 scripts made
  executable. Phase 2 tests map: TestSelectSource T014; TestDetectType, TestUnsupportedMismatch
  T015; TestMarkdownStrip T016; TestEPUBSpineOrder/Corrupt/Limits T018; TestEPUBEncrypted T019;
  TestFB2BodyOnly T020; TestPDFText/NoText T021; TestPDFCorrupt T022; TestFlags,
  TestHelpVersion T023; TestExitCodes, TestErrorsIsChain T024; TestPerfLoadTokenize5MB T025;
  exit criterion in T026.
