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

Batches: 1 = phases 0-1 (T000-T012); 2 = phase 2 (T013-T026); 3 = phase 3 (T027-T036); 4 = phase 4 (T037-T045); next free ID: T046

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
| T027 | 3 | Full tokenizer (paragraph breaks, Sanitize) | T002 | opus | todo | |
| T028 | 3 | Unicode ORP table tests (TestPosition, TestIndex) | T003 | sonnet | todo | |
| T029 | 3 | Timing multipliers (TestDelay, TestDelayPara) | T001 | sonnet | todo | |
| T030 | 3 | Player live keys (R26) with pause and jump timing | T004 | sonnet | todo | rewrites T004's TestPlayerQuit (keys now change state) |
| T031 | 3 | Player effective wpm and drift-free schedule | T030 | sonnet | todo | |
| T032 | 3 | Resume store: directory, atomic save, delete, modes | T004 | opus | todo | |
| T033 | 3 | Resume store: load, corrupt entries, start index | T032 | opus | todo | |
| T034 | 3 | Resume wiring in cmd (TUI path, signals, raw/stdin never persisted) | T024, T033 | opus | todo | |
| T035 | 3 | Resume on the GUI exit path (finish callback) | T034 | opus | todo | |
| T036 | 3 | Phase 3 gate (core pipeline integration tests) | T026, T027, T028, T029, T031, T035 | sonnet | todo | also Phase 3 exit criterion |
| T037 | 4 | Block-glyph font tables from X11 misc-fixed (internal/tui/glyph) | T001 | sonnet | todo | Plan v1.1 row 26: misc-fixed replaces font8x8 (not available offline) |
| T038 | 4 | Block-glyph sizes 2-5 with size and rune fallback | T006, T037 | sonnet | todo | may set Size 1 in older level-1 tests (fix-up allowance in brief) |
| T039 | 4 | Guide ticks and the too-small screen | T038 | sonnet | todo | |
| T040 | 4 | Display widths (wide, combining, zero-width clusters) | T038 | sonnet | todo | |
| T041 | 4 | Long-word splitting and the focus-column invariant | T039, T040 | sonnet | todo | |
| T042 | 4 | Progress and help rows | T039 | sonnet | todo | |
| T043 | 4 | Encode hardening (control stripping) and colour modes | T027, T041, T042 | opus | todo | |
| T044 | 4 | Golden frames (sizes 1-5, ticks, toggles, fallbacks) | T041, T042, T043 | sonnet | todo | |
| T045 | 4 | Phase 4 gate (frame performance, render invariants) | T044 | sonnet | todo | also Phase 4 exit criterion |

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

- 2026-10-08, batch 3 (Plan.md v1, phase 3): added T027-T036 as `todo` (5 opus, 5 sonnet). Phase 3 tests map: TestTokenize, TestSanitize (+FuzzTokenize) T027; TestPosition,
  TestIndex T028; TestDelay, TestDelayPara T029; TestPlayerKeys T030; TestEffectiveWPM,
  TestScheduleNoDrift T031; TestResumeDirFallback/Modes/Atomic/DeleteAtEnd T032;
  TestResumeRoundTrip/HashMismatch/CorruptIgnored, TestStartIndex (+FuzzDecodeEntry) T033;
  TestRunRawNoState (+TestRunResumeFile, TestRunSignalSaves) T034; TestRunGUIFinishSaves
  T035; exit criterion in T036. Decisions: T030 deliberately rewrites T004's TestPlayerQuit
  (its "no other action changes state" clause is obsolete once R26 keys exist; name kept so
  T004.sh still passes). T029 counts letter/digit runes (not grapheme clusters) for the
  long-word bonus so `internal/timing` stays stdlib-only as Plan's architecture and T001.sh
  require; equals the cluster count except for conjoining-jamo/Indic spacing-mark text
  (Plan row 19 tension, no Plan edit). Resume `StartIndex` takes (start, startSet,
  noResume, saved, haveSaved, n), a refinement of Plan's 4-argument sketch. README coverage
  for phase 3: R8 T027; R9 T028; R10 T029; R26 T030; R27 effective wpm T031; R29 T032-T035;
  R32 signal save T034 (leak test stays in phase 5); R11 `--start`/`--no-resume` override
  T033, T034; R12 save failure exit 1 T034, T035; §5 control chars T027, T036; §5
  data/privacy T032-T034; §5 timing accuracy T031; AC7 T028; AC8 T029; AC21 T030; AC22
  (fake-clock wpm) T031; AC23 T032-T035; AC26 SIGTERM-saves part T034 (binary, < 1 s).

- 2026-10-08, batch 4 (Plan.md v1.1, phase 4): added T037-T045 as `todo` (1 opus, 8 sonnet).
  Plan.md edited for a real gap (now v1.1): row 26's font8x8 master cannot be obtained
  offline (downloads forbidden), so the glyphs come from the public-domain X11 misc-fixed
  ISO 8859-1 fonts installed here (4x6, 6x10, 7x14, 9x18 = exact 2R-pixel heights, no
  scaling, advance = box width) via a committed `//go:build ignore` generator whose output
  T037.sh regenerates byte for byte; row 27 gained the vertical rule y0 = h/2 - (R-1)/2 and
  a width condition in the size fallback (size 4 needs w >= 23, size 5 w >= 29) so R20
  splitting always progresses; rows 39, D14, Phase 7 deliverables and the Risks line now
  name `third_party/misc-fixed/LICENSE` (written in phase 7; T012.md's font8x8 remark is
  stale but left untouched). Decisions: whole focus glyph box is `StyleFocus` (NO_COLOR
  reverse video stays legible); zero-width clusters render as U+25CC + cluster; wide
  `Cont` cells are `StylePlain`; progress row = bar `━`/`─` + `word i/N  E wpm` (E = `—`
  until known); new API for phase 5: `EffectiveSize`, `Split`, `Model.Part`,
  `Frame.Size`, `Frame.Parts` (timing of split steps stays with the phase 5 loop). Hand
  numbers (size fallback table, split steps, glyph rows, progress rows, spot columns)
  verified with scratch Go programs against uniseg v0.4.7 and the real PCF files. Phase 4
  tests map: TestGlyphTables T037; TestSizeFallback, TestGlyphFallback T038; TestTicks,
  TestTooSmall T039; TestWideAndCombining T040; TestSplitLongWord, TestSplitLinear,
  TestFocusColumn T041; TestProgressHelpFocus, TestNoProgressFrame T042;
  TestEncodeStripsControls (+FuzzRenderEncode), TestColorModes, TestNoColor T043; TestGolden
  T044; TestRenderFast, BenchmarkRenderFrame, TestRenderInvariants + exit criterion T045.
  README coverage for phase 4: R14 T043; R15 T037, T038, T044; R16 T038, T041; R17 T038,
  T039 (live SIGWINCH in phase 5); R18 T040; R19 T039, T044; R20 T041 (per-step timing in
  phase 5); R27 T042; R28 T042; R33 golden frames T044, focus column T041; §5 control
  chars T043; §5 frame time T045; §5 legal font provenance T037 (notice in phase 7); AC11
  T043; AC12 T038, T044; AC13 unit part T041; AC14 unit part T038, T039; AC15 T040; AC16
  T039, T044; AC17 T041; AC22 frames T042, T044; AC28 frame part T045.
