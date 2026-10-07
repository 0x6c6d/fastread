<role>
You are a senior Go engineer. You work precisely and thoroughly. You verify everything by
building and running it, and you never claim something works without having run it.
</role>

<goal>
Build "fastread", a Linux CLI in Go for RSVP speed reading. It shows one word at a time at
a fixed screen position. Each word has one red focus letter (ORP); all other letters are
white. The focus letter never moves, so the reader's eye never has to move.
Module path: github.com/<user>/fastread (ask only if I haven't given it). Go: current stable.
Target: Linux only. Licence: MIT.
</goal>

<priorities>
Must: tokenizer, ORP, timing, all input loaders, TUI, resume, tests, README.
Should: GUI mode (Gio), progress bar.
Non-goals: Windows/macOS, scanned/OCR PDFs, DRM'd EPUBs, RTL text shaping, config files.
If time or environment forces a cut, cut "Should" items and say so; never cut Must items silently.
</priorities>

<input>
Source is chosen in this order. Detect file type by extension AND magic bytes.
1. Positional argument that is not an existing file path -> raw text, e.g. `fastread "some text"`.
   Existing file path -> file loader.
2. No argument and stdin is a pipe/file (not a TTY) -> read stdin (UTF-8; invalid bytes replaced).
3. No argument and stdin is a TTY -> print usage, exit 2.
Supported files: .txt, .md (strip Markdown syntax: headings, emphasis, links keep their text,
code fences dropped), .epub (zip; spine order; XHTML -> text), .fb2 (XML, <body> text only),
.pdf (text layer only). A PDF without extractable text fails with a clear message, exit 1.
Dependencies: prefer pure Go. In the plan list every third-party dependency with a one-line
justification and, for PDF/EPUB, one alternative considered. No dependency without justification.
</input>

<display_modes>
`--ui tui|gui`, default tui.

TUI:
- Alternate screen, restored on exit, Ctrl+C, SIGTERM and panic (defer + recover).
- Truecolor red for focus (fallback: ANSI 256, then ANSI 16 bright red; honour NO_COLOR by
  using bold/reverse instead of colour).
- Size levels 1-5 (default 2): level 1 = normal text; 2-5 = block glyphs scaled
  (default: 3/5/7/9 rows high). The plan must describe the glyph source (embedded bitmap font
  covering ASCII + Latin-1; unknown runes fall back to level-1 text).
- Handle resize (SIGWINCH): re-centre, keep focus column at the centre column.
- Terminal smaller than the chosen size -> automatically drop to the largest size that fits;
  smaller than 20x5 -> show "terminal too small" instead of crashing.
- Width of a rune: use display width (wide/emoji = 2 cells, combining = 0).

GUI (Gio, gioui.org, pinned version in go.mod; no other GUI toolkit):
- Window default 600x300, resizable, black background. Text size is real font scaling.
- Draw the word yourself with Gio text shaping; measure glyph advances so the focus letter's
  horizontal centre is exactly on the window's vertical centre line (pixel-accurate, rounded
  consistently). Baseline is fixed so the word does not jump vertically.
- Two guide ticks (above and below the focus letter) drawn with clip/paint ops at the same x.
- Embedded font: Go Regular or Go Mono from golang.org/x/image/font/gofont (default;
  document the choice). No system fonts.
- Key handling via Gio events, re-layout on resize.
- Debian/Ubuntu system deps (document in README and check in the plan):
  Wayland: libwayland-dev libxkbcommon-dev libegl1-mesa-dev
  X11: libx11-dev libxkbcommon-x11-dev libxcursor-dev libxfixes-dev
  Vulkan: libvulkan-dev; plus gcc and pkg-config.
  Must work on X11 and Wayland, or document the limitation.
- No display or missing GUI libs/`nogui` build: `--ui gui` prints a one-line clear error to
  stderr and exits 1. It never panics.
- Headless testing: use Xvfb (+ xdotool if available) to smoke-test the window; if not
  available, say so explicitly.
</display_modes>

<rendering_rules>
- Exactly one word at a time. Focus letter red, all other letters white.
- Focus index is computed on the word's letters (runes; leading/trailing punctuation excluded):
  1 letter -> 0; 2-5 -> 1; 6-9 -> 2; 10-13 -> 3; 14+ -> 4.
  Punctuation stays attached and visible but is never the focus. Digits count as letters.
  A token with zero letters or digits (e.g. "--", "—") uses its middle rune as focus.
- Focus letter is always at the centre column/pixel. Pad on the left; never re-centre the word.
- Short vertical guide ticks directly above and below the focus position, always visible
  (between words and when paused).
- Unicode: handle accents, umlauts, combining marks and emoji (grapheme-cluster aware; must
  never crash or misalign the focus letter).
- Words longer than the available width: split with a trailing hyphen across successive display
  steps, or shrink in the GUI. Never overflow, never crash.
</rendering_rules>

<controls>
Flags (all validated; invalid -> usage text on stderr, exit 2):
  --wpm N (default 300, range 50-1500)   --size N (default 2, range 1-5)
  --ui tui|gui                           --start N (0-based word index; overrides resume)
  --no-resume                            --no-progress
  -h/--help                              --version
Live keys (both modes):
  Space pause/resume | Up/Down = wpm +/-25 | [ / ] = size -/+
  Left/Right = -1/+1 word when paused, -10/+10 when playing | Home = restart
  p = toggle progress | ? = toggle help line | q, Esc, Ctrl+C = quit
</controls>

<timing_and_extras>
- Pure function `Delay(word string, wpm int) time.Duration`. Base = 60000/wpm ms.
  Constants (default, easy to change in one place): sentence end (. ! ? and … ) x2.0;
  clause end (, ; : ) x1.5; long word (>8 letters) +5% per extra letter, capped at +50%;
  paragraph break x2.5. Multipliers multiply once (do not stack beyond the max of the group).
- Progress: thin bar + "word i/N" + effective wpm (words shown / elapsed unpaused time),
  on by default, toggled with `p`, disabled by --no-progress. It must not move the focus position.
- Resume: store last word index per source (file path + SHA-256 of contents) as JSON in
  $XDG_STATE_HOME/fastread/ (fallback ~/.local/state/fastread/). Write atomically on quit and
  on SIGINT/SIGTERM. Raw text and stdin are never persisted. If the hash differs, ignore the
  saved position. Finishing the text deletes the entry.
</timing_and_extras>

<engineering_requirements>
- Idiomatic Go, layout: cmd/fastread, internal/{input,tokenize,orp,timing,state,tui,gui}.
  Logic packages have no UI imports. Gio is only imported from internal/gui.
- Build tag `nogui`: TUI-only binary, no cgo. Verify both `go build ./...` and
  `go build -tags nogui ./...` (and `CGO_ENABLED=0` with the tag).
- Errors wrapped with %w; sentinel/typed errors for missing file, corrupt EPUB, empty text,
  unsupported type. Exit codes: 0 ok, 1 runtime error, 2 usage error.
- Context with signal.NotifyContext for shutdown; no goroutine leaks (check with a test).
- Tests (table-driven): ORP incl. Unicode/punctuation, tokenizer, Delay, flag validation, each
  loader (build tiny EPUB/FB2/PDF fixtures in testdata or programmatically), golden test for
  TUI frame rendering of known words at sizes 1-5, GUI layout math tested without a display.
- README: what it is, install/build (incl. Gio deps and nogui), an example for every input
  type, flag and key tables, known limitations.
- Must pass: gofmt -l (empty), go vet ./..., go test ./... (also with -race and -tags nogui).
</engineering_requirements>

<workflow>
1. PLAN (short, concrete): packages, deps with justification, TUI scaling, focus alignment in
   TUI and Gio, risks (PDF extraction quality, cgo), and "Assumptions" for anything I left open.
   Show the plan, then continue immediately unless a decision is costly to reverse.
   If it is, ask me one batched question and wait. No code before the plan is shown.
2. IMPLEMENT in stages, each ending with a gate (build + vet + test must be green):
   a core (tokenize, orp, timing) b loaders c TUI d GUI e resume + progress + README.
   Do not start a stage while the previous gate is red.
3. VERIFY: gofmt, vet, tests, both build variants, and run the real binary on every input type
   (raw, stdin, txt, md, epub, fb2, pdf) plus error cases (missing file, empty text, bad flag).
   TUI: run it inside a pty (for example tmux) and capture frames to confirm the focus column is
   constant. Anything you cannot test, say so explicitly.
4. REPORT: what was built, real command output, assumptions, deviations, known limitations.
</workflow>

<success_criteria>
- `fastread "Hello wonderful world"` and `cat book.txt | fastread` work.
- All listed formats load; broken input gives a clear error and the correct exit code.
- In both modes the focus letter's position is identical for every word (asserted by a test),
  and the guide ticks are visible.
- wpm and size change by flag and live by key.
- `--ui gui` opens a window, or fails with a clear message.
- Every check in step 3 passes, or is reported as "not run" with the reason.
</success_criteria>
