# Tasks: fastread

_Generated from Plan.md v1.1 (batches 1-3 from v1). This file is the loop's persistent state (status and notes
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

Batches: 1 = phases 0-1 (T000-T012); 2 = phase 2 (T013-T026); 3 = phase 3 (T027-T036); 4 = phase 4 (T037-T045); 5 = phase 5 (T046-T056); 6 = phase 6 (T057-T065); 7 = phase 7 (T066-T069); next free ID: T070

| ID | Phase | Title | Depends-on | Model | Status | Notes |
|----|-------|-------|------------|-------|--------|-------|
| T000 | 0 | Preflight | — | supervisor | done | all 7 preflight checks pass |
| T001 | 1 | Go module and timing skeleton | T000 | opus | done | go.mod (go 1.24.1, toolchain go1.24.4), internal/timing constants, Delay/DelayPara base |
| T002 | 1 | Tokenizer skeleton | T001 | haiku | done | internal/tokenize: Token, Tokenize skeleton + test |
| T003 | 1 | ORP focus index skeleton (uniseg pin) | T001 | opus | done | internal/orp (Clusters, Position, Index via uniseg v0.4.7), go.mod/go.sum |
| T004 | 1 | Playback model skeleton (state.Player) | T001, T002 | sonnet | done | internal/state: Clock, Player skeleton + test |
| T005 | 1 | Input skeleton (selection, raw, stdin) | T001 | opus | done | internal/input: Source, Select, Load raw/stdin, typed errors + tests; orphan from interrupted run verified |
| T006 | 1 | TUI frame renderer and ANSI encoder skeleton | T003 | sonnet | done | internal/tui: color.go, render.go, encode.go + tests |
| T007 | 1 | TUI runtime skeleton (terminal, alt screen, loop) | T004, T006 | opus | done | internal/tui: terminal.go, run.go, run_test.go; x/term, x/sys pinned |
| T008 | 1 | GUI skeleton (Gio window, nogui stub) | T004 | opus | done | internal/gui: gui.go, stub.go, window.go + tests; gioui v0.10.3, x/image pinned; doc wording deviates (see Improvements) |
| T009 | 1 | CLI flags, usage, help and version | T001 | opus | done | cmd/fastread: main.go, flags.go, run.go + tests; run doc comment reworded (see Improvements) |
| T010 | 1 | CLI wiring (input to TUI/GUI) | T002, T005, T007, T008, T009 | opus | done | cmd/fastread/run.go wiring + run_test.go (seams, signals, exit codes) |
| T011 | 1 | E2E harness and TestE2EBasic (tmux) | T010 | sonnet | done | e2e/helpers_test.go, basic_test.go; Phase 1 exit criterion passes |
| T012 | 1 | MIT LICENSE file | T000 | opus | done | LICENSE (MIT, last line rewrapped to satisfy line-based check) |
| T013 | 2 | Hardened input helpers (size caps, zip, XML, recover) | T005 | opus | done | internal/input/safe.go + safe_test.go; Load delegates to load(lim) |
| T014 | 2 | Source selection with the path-like rule (A3) | T005, T010 | opus | done | internal/input/select.go + select_test.go; Select moved out of input.go |
| T015 | 2 | File type detection, file loading and loader stubs | T010, T013 | opus | done | detect.go, load.go, loader stubs, fixture_test.go + tests; TestLoadRawStdin stub case updated to ErrNotFound (superseded behaviour) |
| T016 | 2 | Markdown stripper | T015 | sonnet | done | markdown.go stripper, markdown_test.go, testdata/sample.md; load_test markdown stub case updated |
| T017 | 2 | XHTML to paragraphs (x/net/html pin) | T013 | opus | done | xhtml.go + tests, race const files; x/net v0.50.0 direct; td/th as word separators (brief gap, see Improvements) |
| T018 | 2 | EPUB loader (container, OPF spine, zip-entry resolution) | T015, T017 | opus | done | epub.go loader, epub_fixture_test.go, epub_test.go, testdata/sample.epub |
| T019 | 2 | EPUB DRM detection (encryption.xml) | T018 | opus | done | epub_drm.go + epub_drm_test.go; checkEncryption wired into loadEPUB |
| T020 | 2 | FB2 loader (body text only) | T015 | sonnet | done | fb2.go, fb2_test.go, testdata/sample.fb2 |
| T021 | 2 | PDF text-layer loader (ledongthuc/pdf pin) | T015 | opus | done | pdf.go loader, pdf_fixture_test.go, pdf_test.go, sample.pdf + notext.pdf; ledongthuc/pdf pinned |
| T022 | 2 | PDF hostile-input suite | T021 | opus | done | pdf_corrupt_test.go (20 rows); pdf.go root-Pages validation + one-line errors |
| T023 | 2 | Flag validation table and help/version tests | T010 | opus | done | cmd/fastread/cli_test.go (TestFlags, TestHelpVersion); no production change |
| T024 | 2 | Typed-error chain and exit-code mapping | T014, T015, T018, T021 | opus | done | run.go: usageError, prepare, exitCode, runErr; errors_test.go; empty-text error now carries file path |
| T025 | 2 | Performance test 5 MB load and tokenize | T015 | sonnet | done | cmd/fastread perf_test.go + race const files; measured ~70 ms for 5 MiB (bound 2 s) |
| T026 | 2 | Phase 2 gate (fixtures through the CLI flow) | T016, T019, T020, T022, T023, T024, T025 | sonnet | done | fixtures_test.go, testdata/sample.txt; Phase 2 exit criterion passes |
| T027 | 3 | Full tokenizer (paragraph breaks, Sanitize) | T002 | opus | done | tokenize.go: Sanitize + single-pass Tokenize; tests + FuzzTokenize |
| T028 | 3 | Unicode ORP table tests (TestPosition, TestIndex) | T003 | sonnet | done | internal/orp/orp_table_test.go (37 rows); orp.go unchanged |
| T029 | 3 | Timing multipliers (TestDelay, TestDelayPara) | T001 | sonnet | done | timing/delay.go multipliers; TestDelay, TestDelayPara |
| T030 | 3 | Player live keys (R26) with pause and jump timing | T004 | sonnet | done | player.go R26 Apply + remaining; TestPlayerKeys; TestPlayerQuit rewritten as planned |
| T031 | 3 | Player effective wpm and drift-free schedule | T030 | sonnet | done | player.go EffectiveWPM; TestEffectiveWPM, TestScheduleNoDrift |
| T032 | 3 | Resume store: directory, atomic save, delete, modes | T004 | opus | done | state/store.go write half (Dir, Store, Save, Delete) + store_test.go |
| T033 | 3 | Resume store: load, corrupt entries, start index | T032 | opus | done | store.go Load, decodeEntry, StartIndex, ErrCorruptState; store_load_test.go; directory-entry row asserts Save error (brief gap) |
| T034 | 3 | Resume wiring in cmd (TUI path, signals, raw/stdin never persisted) | T024, T033 | opus | done | resume.go, run.go (runTUI, notifyContext seams), resume_test.go; GUI TODO comment reworded (see Improvements) |
| T035 | 3 | Resume on the GUI exit path (finish callback) | T034 | opus | done | run.go runGUI seam + once-guarded finish saving resume; resume_gui_test.go |
| T036 | 3 | Phase 3 gate (core pipeline integration tests) | T026, T027, T028, T029, T031, T035 | sonnet | done | core_test.go (TestPrepareSanitized, TestCorePipeline); Phase 3 exit criterion passes |
| T037 | 4 | Block-glyph font tables from X11 misc-fixed (internal/tui/glyph) | T001 | sonnet | done | internal/tui/glyph: gen.go, data.go (byte-identical regen), glyph.go, glyph_test.go; Plan v1.1 row 26: misc-fixed replaces font8x8 (not available offline) |
| T038 | 4 | Block-glyph sizes 2-5 with size and rune fallback | T006, T037 | sonnet | done | layout.go EffectiveSize, render.go blocks, glyph_render_test.go; fix-ups: TestEncodeColors h 3-5, cmd TestRunSignalSaves --size 1 |
| T039 | 4 | Guide ticks and the too-small screen | T038 | sonnet | done | render.go ticks + TooSmallText; ticks_test.go (TestTicks, TestTooSmall, TestTicksSpot) |
| T040 | 4 | Display widths (wide, combining, zero-width clusters) | T038 | sonnet | done | render.go level-1 width placement (U+25CC for zero-width); widths_test.go |
| T041 | 4 | Long-word splitting and the focus-column invariant | T039, T040 | sonnet | done | layout.go Split (linear), render.go Part/Parts, split_test.go, race const files |
| T042 | 4 | Progress and help rows | T039 | sonnet | done | render.go HelpText + addRows (help row 0, progress row h-1); rows_test.go |
| T043 | 4 | Encode hardening (control stripping) and colour modes | T027, T041, T042 | opus | done | encode.go cleanCell + tick default colour + clipping; encode_test.go (+FuzzRenderEncode) |
| T044 | 4 | Golden frames (sizes 1-5, ticks, toggles, fallbacks) | T041, T042, T043 | sonnet | done | golden_test.go + 28 testdata/golden files |
| T045 | 4 | Phase 4 gate (frame performance, render invariants) | T044 | sonnet | done | perf_test.go, invariants_test.go (3000 cases); Phase 4 exit criterion passes |
| T046 | 5 | Player split-step timing (SetParts) | T031 | sonnet | done | player.go SetParts/Part/Parts; steps_test.go |
| T047 | 5 | Key decoder with escape sequences (TestKeyDecode) | T007 | sonnet | done | internal/tui keys.go KeyDecoder + keys_test.go (TestKeyDecode, FuzzKeyDecode) |
| T048 | 5 | TUI loop on Player deadlines (keys, split steps, drift) | T041, T042, T046, T047 | opus | done | loop.go (Run + loop, seams), run.go only Options, loop_test.go; 50 wpm start asserts 75/50 (brief gap) |
| T049 | 5 | SIGWINCH re-layout and the real /dev/tty (TestLoopResize) | T048 | opus | done | terminal.go Resizer, newTTY, idempotent Close; loop.go resize case; resize_test.go, terminal_test.go (pty); go.mod x/sys still marked indirect |
| T050 | 5 | Restore on every exit path and the leak test | T049 | opus | done | loop.go guarded single cleanup, reader error reporting; exit_test.go (12 exit paths, 6 panic rows, 72 leak runs) |
| T051 | 5 | E2E stdin, not-found and no-argument TTY (tmux) | T011, T024, T034 | sonnet | done | e2e/cli_test.go (TestE2EStdin, TestE2ENotFound, TestE2ENoArgTTY) |
| T052 | 5 | E2E terminal restore (TestE2ERestore) | T011, T050 | sonnet | done | e2e/proc_test.go (paneState, appPID), restore_test.go (5 exit ways) |
| T053 | 5 | E2E focus column with a pane-screen parser | T011, T048 | sonnet | done | e2e/screen_test.go (parseScreen, focusCols, tickCols), focus_test.go (4 subtests) |
| T054 | 5 | E2E live resize (TestE2EResize) | T049, T053 | sonnet | done | e2e/resize_test.go (TestE2EResize, 7 steps, -count=2 stable) |
| T055 | 5 | E2E resume and SIGTERM state save | T034, T052 | sonnet | done | e2e/resume_test.go (TestE2ESIGTERMSavesState, TestE2EResume) |
| T056 | 5 | Phase 5 gate (live keys e2e, exit criterion) | T050, T051, T052, T053, T054, T055 | sonnet | done | e2e/keys_test.go (TestE2EKeys); Phase 5 exit criterion passes |
| T057 | 6 | GUI pure layout: focus x, baseline, ticks | T008, T028 | sonnet | done | gui/layout.go (pure Layout, PxOf, Measurer), measure_test.go, layout_test.go |
| T058 | 6 | GUI shrink and split (R23), all-sizes focus invariant | T057 | sonnet | done | layout.go shrinkSplit (R23), shrink_test.go (31 words, 1395 cases), race const files |
| T059 | 6 | GUI help and progress geometry (pure) | T057, T042 | sonnet | done | gui/chrome.go (LayoutChrome, ProgressText, HelpText), chrome_test.go |
| T060 | 6 | Gio key map (TestGUIKeyMap) | T008, T030 | sonnet | done | internal/gui/keys.go, keys_test.go: one key table drives keyFilters and keyAction; 1 round |
| T061 | 6 | Gio window: layout drawing, Player deadlines, keys, resize | T035, T046, T058, T059, T060 | opus | done | window.go, measure_gio.go, measure_gio_test.go: Gio drawing on Player deadlines; 1 round; verify uses _gui.sh |
| T062 | 6 | GUI exit path and display errors (finish once, R24) | T061 | opus | done | session.go, session_test.go, window.go; row fixed: unescaped newSession regex in verify; 1 round |
| T063 | 6 | E2E Xvfb harness and TestE2EGUIXvfb | T011, T062 | sonnet | done | e2e/xvfb_test.go, gui_test.go: adds helpers newXvfb, startGUI, readIndex; 1 round |
| T064 | 6 | E2E GUI error paths and the nogui binary | T011, T062 | sonnet | done | e2e/guierr_test.go; 1 round |
| T065 | 6 | Phase 6 gate (live GUI keys e2e, exit criterion) | T063, T064 | sonnet | todo | also Phase 6 exit criterion |
| T066 | 7 | Third-party font licence texts (gofont, misc-fixed) | T008, T037 | opus | todo | opus per Consequential row 39 although Plan's routing example says haiku; content fixed byte for byte by the script |
| T067 | 7 | E2E every input type and error case (TestE2EInputTypes) | T026, T051 | sonnet | todo | its CASE lines are the D16 report material |
| T068 | 7 | Usage documentation (docs/usage.md) | T036, T056, T065, T066 | opus | todo | opus: Risk CLI contract and row 39; script cross-checks --help, HelpText, LevelSp, MinSp, go.mod |
| T069 | 7 | Global Definition of Done gate (D1-D17) | T012, T045, T067, T068 | supervisor | todo | also Phase 7 exit criterion; D5 uses scripts/verify/_dodcheck.go |

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

- 2026-10-08, batch 5 (Plan.md v1.1, phase 5): added T046-T056 as `todo` (3 opus, 8 sonnet).
  No Plan.md edit. Decisions: split-step timing (Plan row 28, left open by phase 4) lives in
  `state.Player` as `SetParts`/`Part`/`Parts` (T046; UI-independent, so phase 6 reuses it;
  non-final step = `timing.Delay(part minus "-")`, final = `DelayPara(part, ParaEnd)`, equal
  parts are a no-op so Up/Down keep the current deadline, a changed split keeps the time
  already spent in the step, so deadlines stay absolute); the loop only computes
  `Split` and calls `SetParts` (T048). `Run` moves from `run.go` into `loop.go` with test
  seams `renderFn`, `encodeFn`, `frameHook`. Lone Esc quits after `EscTimeout` = 50 ms;
  unknown CSI/SS3 sequences are swallowed, Ctrl+C always quits (T047). SIGWINCH via an
  optional `Resizer` interface on `Terminal` (real tty: `signal.Notify`/`signal.Stop` in
  `terminal.go`), so the existing fake terminals in cmd tests stay valid; real tty tested on
  a pty pair via `x/sys/unix` (`TestTTYPty`, T049). A hung-up tty (Read error) ends Run with
  an error, exit 1, position still saved by cmd (T050). No cmd change in phase 5: SIGINT/
  SIGTERM save and resume wiring stay with T034; T055 only adds the Plan's e2e tests on top.
  Extra tests beyond the Plan list: TestPlayerSteps, TestPlayerStepsNoDrift, FuzzKeyDecode,
  TestLoopKeys, TestLoopSplitSteps, TestLoopExitPaths, TestTTYPty, TestParseScreen,
  TestE2EKeys (live R26-R28 keys through real tmux sequences, in the gate). tmux facts
  checked here (3.5a): send-keys Right/Home/PPage/F1/Escape/C-c → `ESC[C`, `ESC[1~`,
  `ESC[5~`, `ESCOP`, `ESC`, 0x03; `#{alternate_on}`, `#{cursor_flag}` exist; `capture-pane
  -e` reports `91`, `38;5;196`, `38;2;255;0;0`, `1;7` unchanged; `resize-window` to 1×1 on a
  detached session works. Phase 5 tests map: TestPlayerSteps, TestPlayerStepsNoDrift T046;
  TestKeyDecode (+FuzzKeyDecode) T047; TestLoopKeys, TestLoopSplitSteps,
  TestLoopDriftRealClock T048; TestLoopResize, TestTTYPty T049; TestRestoreOnPanic,
  TestLoopExitPaths, TestLoopNoLeak T050; TestE2EStdin, TestE2ENotFound, TestE2ENoArgTTY
  T051; TestE2ERestore T052; TestParseScreen, TestE2EFocusColumn T053; TestE2EResize T054;
  TestE2ESIGTERMSavesState, TestE2EResume T055; TestE2EKeys + exit criterion T056. README
  coverage for phase 5: R1 stdin/TTY rule live T051 (+T011); R7 not-found live T051; R13
  T050, T052; R14 live NO_COLOR/truecolor T053; R16 live T053; R17 SIGWINCH T049, T054;
  R20 per-step timing T046, T048; R26 keys T047, T048, T056; R27/R28 live toggles T056;
  R29 live T055; R32 leak T050, SIGTERM/SIGINT T052, T055; §3 flow step 6 (keys and resize
  during delays) T048, T049; §5 timing accuracy real clock T048 (fake clock stays T031,
  T046); AC1 T051 (+T011); AC10 T050, T052; AC13 live T053; AC14 live T049, T054; AC21
  terminal-key part T047; AC23 live T055; AC26 T050, T055; AC28 drift T048. R21-R25 and
  AC18-AC20 are GUI (phase 6), not phase 5.

- 2026-10-08, batch 6 (Plan.md v1.1, phase 6): added T057-T065 as `todo` (2 opus, 7
  sonnet). No Plan.md edit. New supervisor-owned verify helpers: `scripts/verify/_gui.sh`
  (own Xvfb via `-displayfd`, app start/keys/wait, cleanup trap) and
  `scripts/verify/_xwdcheck.go` (XWD dump facts: size, black share, red box, tick rows,
  top/bottom ink; leading `_` keeps it out of every package; gofmt-clean). Facts checked
  here with scratch programs against gioui.org v0.10.3, x/image v0.36.0, uniseg v0.4.7 (module
  cache, no network): Gio's shaper and `x/image/font/opentype` (unhinted, 72 DPI) give
  identical advances and ascent/descent for all 2068 printable Latin-1 rune/size pairs, so
  display-free tests measure with x/image; every literal value in T057-T059 (origins,
  baselines, tick rects, shrink sizes, split steps, chrome rows) comes from a reference
  implementation of the briefs' rules; under Xvfb Gio opens 600x300 px (PxPerDp 1), xwd
  dumps show the drawn pixels (red box centred exactly on W/2, 2 px ticks at W/2-1..W/2,
  also after `xdotool windowsize`), keys arrive only after `xdotool windowfocus --sync`
  (names: q `Q`, `?` with Shift, Ctrl+C `C`+Ctrl, Release events too), `xdotool
  windowclose` does not end a Gio window (close button tested only via `windowErr(nil)`),
  a bad `DISPLAY` or `WAYLAND_DISPLAY` yields `DestroyEvent.Err` (`wayland:
  wl_display_connect failed: ...`) within ~10 ms (Risk "Gio v0.10.3 newness" path seen
  working). Decisions: pure `Layout` takes a `Measurer` (cluster advances, metrics) and
  never Gio types; baseline and ticks come from the level size so shrinking never moves
  them; tick length max(2, ceil(ascent)/3), gap 4 px, white; shrink checks MinSp first then
  walks down from the level size; the split rule mirrors `tui.Split` at 10 sp; help and
  progress live in a separate pure `LayoutChrome` (14 sp) that takes no word, so toggles
  cannot move the focus; the window measures and draws each grapheme cluster separately
  (`gioMeasurer`, `op.Affine` sub-pixel offsets); the exit path is a Gio-free `session`
  (`end` once, `guard`, `windowErr`), testable in every build. Extra tests beyond the Plan
  list: TestLayoutChrome, TestGioMeasurer, TestGUIExitPath, TestE2EGUIKeys (live keys and
  resume through the GUI, in the gate). The optional AC19 pixel check is run by the
  T061/T065 scripts (xwd present, `import` absent), so D16 can report it as run. Phase 6
  tests map: TestLayoutFocusX, TestLayoutBaseline, TestLayoutTicks T057;
  TestLayoutShrinkSplit, TestLayoutFocusXAllSizes T058; TestLayoutChrome T059;
  TestGUIKeyMap T060; TestGioMeasurer T061; TestGUIExitPath T062; TestE2EGUIXvfb T063;
  TestE2EGUINoDisplay, TestE2EGUIBadDisplay, TestE2ENoguiBinary T064; TestE2EGUIKeys +
  exit criterion, D8, D10-D13 T065. README coverage for phase 6: R21 T061, T063 (600x300,
  resizable, black, goregular, no system fonts); R22 T057, T061 (pixels), T065; R23 T058,
  T060, T061, T063; R24 T062, T064 (Wayland live "not run", error path via bogus
  `WAYLAND_DISPLAY`); R25 T057-T059; R26 GUI T060, T061, T065; R27/R28 GUI T059, T061,
  T065; R29 GUI T063, T065 (save wiring stays T035); R30 T057, T065; R31 T064, T065; R32
  GUI signals T062, T063; R33 GUI focus invariant T058; AC18 T057, T058; AC19 T063 (+
  pixels T061, T065); AC20 T062, T064; AC21 GUI part T060; AC25 T064, T065. Phase 7 keeps
  docs, licences (incl. `third_party/gofont/LICENSE`) and TestE2EInputTypes.

- 2026-10-08, batch 7 (Plan.md v1.1, phase 7): added T066-T069 as `todo` (2 opus, 1 sonnet,
  1 supervisor). No Plan.md edit. Tiers: T066 (licence texts) and T068 (docs) are opus
  although the Plan's routing examples say haiku/sonnet: both touch Consequential row 39 and
  T068 also the Risk "CLI contract" (prompt rule: never below opus); T066's content is fixed
  byte for byte (`third_party/gofont/LICENSE` = `font/gofont/ttfs/README` of the pinned
  `golang.org/x/image v0.36.0`, the Bigelow & Holmes licence; `third_party/misc-fixed/LICENSE`
  = a fixed notice quoting the four PCF `COPYRIGHT` properties, checked live). Root `LICENSE`
  stays T012 (complete; its font8x8 remark is stale). T067 `TestE2EInputTypes` logs one
  `CASE name=<n> exit=<c> output=<q>` line per case (raw, stdin, txt, md, epub, fb2, pdf via
  tmux; missing-file, empty-text, bad-flag, scanned-pdf, wrong-extension via exec), which D16
  quotes. T068 makes docs accurate to the build by cross-checking against code: every
  `--help` flag, `tui.HelpText` verbatim, every `go.mod` require module, `gui.LevelSp`/`MinSp`
  values; docs must name no flag the binary lacks (negative). T069 (final DoD row, run by the
  supervisor) runs D0 (all other rows done) and D1-D17 as labelled checks, D4 also on the
  binary, D5 also via the new supervisor-owned `scripts/verify/_dodcheck.go` (AC7/AC8 numbers
  on `orp`/`timing` directly; `//go:build ignore`, copied to a temporary `./.dodcheck.*` dir
  and run with `go run`, so `./...` never sees it); D16 always reports Wayland "not run" (no
  headless compositor; the user's session is off limits, slightly stricter than Plan D16's
  "unless WAYLAND_DISPLAY is set") and re-runs the optional xwd pixel check; D17 = D1 passed
  with HEAD unchanged and no uncommitted code. Whole-plan coverage check (prompt step 6, run
  mechanically): index 9 columns on all 70 rows, every phase 0-7 has rows, all dependency IDs
  exist, no forward references, no cycles (`tsort`), every row has `tasks/<ID>.md` and an
  executable `scripts/verify/<ID>.sh` passing `bash -n` that sources `_lib.sh` and ends in
  `finish`, no orphan files, every Plan-named test appears in a task file, every brief under
  6000 characters; R1-R33 and AC1-AC32 all in Plan's coverage table and all but AC15 also in a
  task file (AC15 = TestWideAndCombining, T040); every Risk item maps to an opus task (or the
  supervisor rows T000/T069); opus scripts T010, T014, T015, T022, T023, T035 carry their
  negative cases as `expect_ok "negative: …"` exact-exit checks rather than `expect_fail`
  (accepted). Fixed: T036.md header now lists T026 like its row.
