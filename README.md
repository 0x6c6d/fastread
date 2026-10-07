# fastread

## 1. Overview

`fastread` is a Linux command-line tool written in Go for RSVP (rapid serial visual presentation) speed reading. It shows exactly one word at a time at a fixed screen position, with one red focus letter (the optimal recognition point, ORP) and all other letters white, so the reader's eyes never move. It reads raw text, stdin, and `.txt`, `.md`, `.epub`, `.fb2` and text-layer `.pdf` files, and renders either in a terminal (TUI, default) or in a Gio window (GUI). Playback speed (50–1500 wpm) and glyph size (levels 1–5) change by flag and live by key, and reading position is resumed per file. The target is a single Linux binary, MIT licensed, module `github.com/0x6c6d/fastread`.

## 2. Core requirements

**Input**

- **R1** Source selection order: (1) a positional argument that is not an existing file path is raw text; an existing path is loaded as a file; (2) with no argument and stdin not a TTY, stdin is read as UTF-8 with invalid bytes replaced by U+FFFD; (3) with no argument and stdin a TTY, usage is printed to stderr and the exit code is 2.
- **R2** File type is detected by extension AND magic bytes (EPUB = zip `PK\x03\x04`, PDF = `%PDF-`, FB2 = XML with `<FictionBook` root). An extension that contradicts the magic bytes yields the typed "unsupported type" error, exit 1.
- **R3** `.txt` loads as UTF-8 text. `.md` is stripped of Markdown syntax: heading markers, emphasis markers, link and image syntax (link text kept, URL dropped), and fenced and inline code blocks (fenced blocks dropped entirely).
- **R4** `.epub` is read from its zip container in OPF spine order; each XHTML item is converted to text (tags dropped, entities decoded, block elements separate paragraphs). A corrupt EPUB yields the typed "corrupt EPUB" error, exit 1.
- **R5** `.fb2` loads text from `<body>` elements only (not `<description>` or binary blobs); each `<p>` is a paragraph.
- **R6** `.pdf` loads the text layer only. A PDF with no extractable text fails with the message `no extractable text (scanned PDF?)`, exit 1.
- **R7** Empty text (zero tokens after tokenizing) yields the typed "empty text" error, exit 1. A missing file path that looks like a path (see assumption A3) yields the typed "file not found" error, exit 1.

**Tokenizing, focus and timing**

- **R8** The tokenizer splits on Unicode whitespace into words and records paragraph breaks (blank line or paragraph boundary from a loader). Punctuation stays attached to its word.
- **R9** Focus index is computed on the word's letters and digits (runes, leading and trailing punctuation excluded): 1 → index 0; 2–5 → 1; 6–9 → 2; 10–13 → 3; 14+ → 4. A token with zero letters or digits (e.g. `--`, `—`) uses its middle rune. Grapheme clusters are never split.
- **R10** `timing.Delay(word string, wpm int) time.Duration` is a pure function. Base = 60000/wpm ms. Multipliers (defined as constants in one place): sentence end (`.`, `!`, `?`, `…`) ×2.0; clause end (`,`, `;`, `:`) ×1.5; long word (more than 8 letters) +5% per extra letter, capped at +50%; paragraph break ×2.5. Within the punctuation group only the single largest multiplier applies; the long-word bonus multiplies on top; the paragraph break multiplier applies to the last word of a paragraph and replaces the sentence/clause multiplier if larger.

**Flags and exit codes**

- **R11** Flags: `--wpm N` (default 300, range 50–1500), `--size N` (default 2, range 1–5), `--ui tui|gui` (default tui), `--start N` (0-based word index, overrides resume), `--no-resume`, `--no-progress`, `-h/--help`, `--version`. An invalid value or unknown flag prints an error plus usage to stderr and exits 2. `--start` beyond the last word is a usage error (exit 2).
- **R12** Exit codes: 0 success (including normal quit and finishing the text), 1 runtime error (I/O, loader, display), 2 usage error. Errors are wrapped with `%w`, with sentinel or typed errors for: file not found, corrupt EPUB, empty text, unsupported type.

**TUI**

- **R13** TUI uses the alternate screen, restored on normal exit, `q`/Esc/Ctrl+C, SIGTERM, and panic (defer + recover). Raw mode is also restored.
- **R14** Focus colour: truecolor red when `COLORTERM` is `truecolor` or `24bit`; else ANSI 256 red; else ANSI 16 bright red. With `NO_COLOR` set (any non-empty value), no colour escape is emitted and the focus letter is bold + reverse instead.
- **R15** Size level 1 renders the word as normal text on one row. Levels 2–5 render block glyphs 3, 5, 7 and 9 rows high from an embedded bitmap font covering ASCII 0x20–0x7E and Latin-1 0xA0–0xFF. A rune outside that set falls back to level-1 text for that word.
- **R16** The focus letter's column is always the centre column of the terminal (`width/2`, integer division), for every word and every size. Words are left-padded, never re-centred.
- **R17** On SIGWINCH the layout is recomputed and the focus stays at the new centre column. If the terminal is too small for the chosen size, the largest size that fits is used automatically; below 20 columns × 5 rows the screen shows `terminal too small` and nothing else, without crashing.
- **R18** Rune width uses display width: wide and emoji runes occupy 2 cells, combining marks 0 cells. Alignment uses grapheme clusters.
- **R19** Two short vertical guide ticks (one above, one below the word area) are drawn at the focus column, visible always: between words and when paused.
- **R20** A word wider than the available width is split into successive display steps, each non-final part ending with `-`; it never overflows or crashes.

**GUI**

- **R21** `--ui gui` opens a Gio window (default 600×300, resizable, black background) using the embedded Go Regular font from `golang.org/x/image/font/gofont`; no system fonts. Text size scales as real font size with the size level.
- **R22** The word is drawn with Gio text shaping; glyph advances are measured so the horizontal centre of the focus letter is exactly on the window's vertical centre line (`width/2`, rounded consistently with one documented rounding rule). The baseline y is fixed for all words. Two guide ticks (above and below the focus letter) are drawn with clip/paint ops at the same x and are always visible.
- **R23** A word wider than the window shrinks its font size (minimum documented) until it fits, then is split as in R20; it never overflows or crashes. Key handling uses Gio events; resize triggers re-layout.
- **R24** With no display, missing GUI libraries, or a `nogui` build, `--ui gui` prints a single-line error to stderr and exits 1; it never panics. The window works on X11 and on Wayland (Wayland verified only if a Wayland session is available; otherwise reported "not run").
- **R25** The GUI layout math (focus x position, baseline, tick geometry, font-shrink) is a pure function, unit tested without a display.

**Controls (both modes)**

- **R26** Live keys: Space pause/resume; Up/Down wpm ±25 (clamped 50–1500); `[` / `]` size −1/+1 (clamped 1–5); Left/Right −1/+1 word when paused and −10/+10 when playing (clamped to valid indices); Home restart at word 0; `p` toggle progress; `?` toggle help line; `q`, Esc, Ctrl+C quit. Playback starts playing (not paused).
- **R27** Progress display: a thin bar, `word i/N` (1-based i) and effective wpm (words shown ÷ elapsed unpaused time), on by default, toggled with `p`, disabled with `--no-progress`. It occupies rows/pixels outside the word area and never moves the focus position.
- **R28** Help line (toggled with `?`) lists the live keys; it also never moves the focus position.

**Resume**

- **R29** On quit and on SIGINT/SIGTERM the last word index is stored as JSON in `$XDG_STATE_HOME/fastread/` (fallback `~/.local/state/fastread/`), keyed by absolute file path + SHA-256 of the file contents, written atomically (temp file + rename). Raw text and stdin are never persisted. On start, a saved position is used unless `--start` or `--no-resume` is given; if the stored hash differs from the current file, the saved position is ignored. Reaching the end of the text deletes the entry. Directory mode is 0700, file mode 0600.

**Engineering**

- **R30** Layout: `cmd/fastread`, `internal/{input,tokenize,orp,timing,state,tui,gui}`. Logic packages (`input`, `tokenize`, `orp`, `timing`, `state`) import no UI packages. `gioui.org` is imported only from `internal/gui`.
- **R31** Build tag `nogui` produces a TUI-only binary: `go build -tags nogui ./...` succeeds, also with `CGO_ENABLED=0`, and the nogui binary contains no `gioui.org` code.
- **R32** Shutdown uses `signal.NotifyContext`; no goroutine leaks (asserted by a test that compares goroutine counts or uses a leak detector after a run).
- **R33** Tests are table-driven and cover: ORP (including Unicode and punctuation), tokenizer, `Delay`, flag validation, each loader (tiny EPUB, FB2 and PDF fixtures built programmatically or in `testdata/`), golden TUI frames for known words at sizes 1–5, and an assertion that the focus column is identical for every word and size in both TUI and GUI layout math.

## 3. Flow

1. `main` parses flags (exit 2 on error; `--help` and `--version` print and exit 0).
2. `input` selects the source per R1, detects the type per R2, loads and returns text with paragraph boundaries, or a typed error (exit 1).
3. `tokenize` produces a word list with per-word paragraph-break flags; empty list → empty-text error.
4. `state` computes the source key (path + SHA-256) for files and looks up the saved index; the start index is `--start`, else the saved index (unless `--no-resume` or hash mismatch), else 0.
5. `main` builds a `signal.NotifyContext` and starts the selected UI (`tui` or `gui`) with the word list, start index, wpm and size.
6. The UI loop: for the current word, `orp` gives the focus index; the UI renders it with the focus at the fixed position; it waits `timing.Delay(word, wpm)`; it advances. Keys and resize events are handled between and during delays.
7. On quit, signal, or end of text, the UI returns the last index; `state` saves it atomically (or deletes the entry at the end); the terminal is restored; the process exits 0.

## 4. Tech stack

- Go: current stable, `go.mod` `go 1.24` or newer (assumption A1); module `github.com/0x6c6d/fastread`.
- TUI: `golang.org/x/term` (raw mode, size), `golang.org/x/sys/unix`, `github.com/rivo/uniseg` (grapheme clusters, display width); ANSI escapes written by hand.
- GUI: `gioui.org` (version pinned in `go.mod`; the planner picks the latest stable release), `golang.org/x/image/font/gofont/goregular`.
- PDF: `github.com/ledongthuc/pdf`. Alternative considered: `github.com/pdfcpu/pdfcpu` (more robust, heavier, geared to manipulation rather than text extraction).
- EPUB/FB2: stdlib `archive/zip`, `encoding/xml`, plus `golang.org/x/net/html` for XHTML. Alternative considered for EPUB: `github.com/taylorskalyo/goreader`.
- Markdown: hand-written stripper (no dependency).
- Block-glyph font: own embedded bitmap font (Go source tables), no dependency.
- Tests: stdlib `testing` only; golden files in `testdata/`.
- Every third-party dependency is listed in `docs/usage.md` with a one-line justification.
- Verification tools: `tmux` (pty frame capture), `Xvfb` and `xdotool` (GUI smoke test).

## 5. Constraints & out of scope

**Security (high-risk)**
- Loaders process untrusted files: no execution of embedded content; EPUB zip entry names are never used as filesystem paths (no extraction to disk; zip-slip is impossible by design); decompressed size is capped (assumption A4: 256 MiB total) to defeat zip bombs; XML parsing must not resolve external entities.
- Terminal output derived from input text must have control characters (C0, C1, ESC) stripped before rendering so a file cannot inject escape sequences.
- No network access at runtime or in tests.

**Data/privacy (high-risk)**
- Resume state is the only persisted data: path, SHA-256 and index. Raw text and stdin are never persisted. No telemetry, no logging of text content.
- State directory 0700, files 0600, atomic writes; a corrupt or unreadable state file is ignored (not fatal) with at most one stderr warning.

**Legal (high-risk)**
- Licence MIT with a `LICENSE` file naming "Lucas Menke" as copyright holder (assumption A5). Embedded fonts (Go fonts, BSD-style licence) require their licence text to be shipped in `docs/` or `third_party/`.
- DRM'd EPUBs are not supported and no circumvention is built.

**Performance**
- Loading a 5 MB UTF-8 text file and tokenizing it takes under 2 seconds on the dev machine.
- Frame render for one word in the TUI takes under 5 ms (benchmark or test with a generous bound of 20 ms to stay stable).
- Timing accuracy: the loop uses absolute deadlines so drift does not accumulate; measured over 100 words at 600 wpm, total time is within ±5% of the sum of `Delay` values.

**Out of scope (must NOT be built)**
- Windows/macOS, scanned/OCR PDFs, DRM'd EPUBs, RTL text shaping, config files, network fetching of text, themes beyond red/white, any GUI toolkit other than Gio, other languages' UI strings.

**Files that must not be touched**
- `prompts/`, `.claude/`, the "Delegation workflow" section of `CLAUDE.md`, and `README.md` (the spec).
- Managed by the loop only: `Plan.md`, `Tasks.md`, `tasks/`, `scripts/verify/`, `scripts/check-protected.sh`, `scripts/protected.sha256`, `Improvements.md`.

## 6. Environment

- Code lives in the repo root (`/home/sa/workspace/code/fastread`). No nested project folder, no `git init`.
- Required tools (each check exits 0). The user installs these before the loop starts; the loop never installs system software:

| Tool | Check |
|---|---|
| Go ≥ 1.24 | `go version` and `test "$(go env GOVERSION | sed 's/go1\.\([0-9]*\).*/\1/')" -ge 24` |
| gcc | `gcc --version` |
| pkg-config | `pkg-config --version` |
| Wayland/X11/Vulkan dev libs | `pkg-config --exists wayland-client xkbcommon xkbcommon-x11 x11 xcursor xfixes egl vulkan` |
| tmux | `tmux -V` |
| Xvfb | `command -v Xvfb` |
| xdotool | `command -v xdotool` |
| git | `git --version` |

- Install line (Debian/Ubuntu): `sudo apt install golang gcc pkg-config libwayland-dev libxkbcommon-dev libegl1-mesa-dev libx11-dev libxkbcommon-x11-dev libxcursor-dev libxfixes-dev libvulkan-dev xvfb xdotool tmux`.
- Go module downloads need network during development (Go module proxy only).
- Secrets: none. No environment variables with credentials are used. Read-only environment variables: `XDG_STATE_HOME`, `HOME`, `NO_COLOR`, `COLORTERM`, `TERM`, `DISPLAY`, `WAYLAND_DISPLAY`.
- Live systems the build may touch: the Go module proxy, and local `Xvfb` and `tmux` processes (own sockets/sessions only). Tests must use a temporary `XDG_STATE_HOME` and never write to the real `~/.local/state`. Everything else is off limits.

## 7. Deliverables

- Source: `cmd/fastread/`, `internal/{input,tokenize,orp,timing,state,tui,gui}/`, `go.mod`, `go.sum`.
- Tests with fixtures and golden files under `testdata/` directories (R33).
- `LICENSE` (MIT).
- `docs/usage.md`: what it is, install/build (including Gio system deps and `nogui`), one example per input type (raw, stdin, txt, md, epub, fb2, pdf), flag table, key table, dependency list with justifications, GUI font choice, known limitations. `README.md` stays the spec.
- Updated `CLAUDE.md` Commands/Architecture sections are NOT part of this build (only the protected Delegation section exists; the loop may log needed edits in `Improvements.md`).
- A verification report in the final loop summary: real command output for every check in section 8, and any check reported "not run" with its reason.

## 8. Acceptance criteria

- [ ] [R1,R2,R7] `fastread --ui tui --no-resume "Hello wonderful world"` under tmux shows the three words in order; `printf 'a b c' | fastread --no-resume` works; a missing path-like argument (e.g. `./nope.txt`) exits 1 with a "not found" message; no-arg on a TTY exits 2 with usage.
- [ ] [R3] Unit test: Markdown fixture with headings, emphasis, links, images, inline code and a fenced block yields exactly the expected word list (link text kept, URL and fenced code absent).
- [ ] [R4] Unit test: programmatically built EPUB with 3 spine items in non-alphabetical manifest order yields words in spine order; a truncated zip returns the corrupt-EPUB error (`errors.Is`).
- [ ] [R5] Unit test: FB2 fixture with `<description>` and `<body>` yields only body words.
- [ ] [R6] Unit test: PDF fixture with text yields its words; a PDF with no text layer returns the no-text error and the binary exits 1 with `no extractable text`.
- [ ] [R2,R12] Test: `.pdf` extension on a non-PDF file returns the unsupported-type error; the binary exits 1.
- [ ] [R8,R9] Table-driven ORP tests pass, including: `a`→0, `ab`→1, `hello`→1, `wonderful`→2, `reading,`→2 (punctuation excluded), `"extraordinary"`→3 (13 letters), `internationally`→4, `--`→middle rune, `héllo`, `naïve`, a word with combining marks, and an emoji word, none crashing.
- [ ] [R10] Table-driven `Delay` tests: at 300 wpm `word`=200 ms, `word.`=400 ms, `word,`=300 ms, a 12-letter word = 200 ms × 1.20, a 30-letter word capped at ×1.50, paragraph-final word ×2.5, and a sentence-ending paragraph-final word ×2.5 (not ×5).
- [ ] [R11,R12] Flag test table: `--wpm 49`, `--wpm 1501`, `--wpm x`, `--size 0`, `--size 6`, `--ui foo`, `--start -1`, `--start 9999`, unknown flag each exit 2 with usage on stderr; `--help` and `--version` exit 0 and print to stdout.
- [ ] [R13] Under tmux: after `q`, after Ctrl+C, after `kill -TERM`, the pane is back on the main screen (alt-screen off, cursor visible) — scripted check via `tmux capture-pane` and `tput`-independent escape inspection; a test forces a panic in the render path and asserts the restore sequence is written.
- [ ] [R14] Unit tests: output with `COLORTERM=truecolor` contains `38;2;255;0;0` (or the documented red) around exactly one letter; with `NO_COLOR=1` output contains no `\x1b[38` / `\x1b[3x` colour sequence and contains bold+reverse around the focus letter; 256- and 16-colour fallbacks tested.
- [ ] [R15] Golden tests at sizes 1–5 for the words `a`, `Hello`, `wonderful`, `naïve` have row counts 1, 3, 5, 7, 9; a word with an unsupported rune (e.g. `日本`) falls back to size-1 text.
- [ ] [R16,R33] Test: for a list of ≥ 30 mixed words (ASCII, accents, punctuation, emoji, wide, long) at each size 1–5 and widths 80 and 121, the rendered focus column equals `width/2` every time. A tmux run captures frames of ≥ 20 words and a script confirms the focus column is constant.
- [ ] [R17] Test: resize from 80×24 to 40×8 to 19×4 renders a smaller size, then `terminal too small`, without panic; a tmux `resize-window` test confirms the same live.
- [ ] [R18] Test: a wide-rune word and a combining-mark word keep the focus on the centre column.
- [ ] [R19] Golden frames contain a tick glyph in the row above and the row below the word at the focus column, also in the paused frame.
- [ ] [R20] Test: a 60-letter word at width 20 splits into parts, each ≤ 20 cells, non-final parts ending in `-`, concatenated parts (minus hyphens) equal the original.
- [ ] [R21,R22,R25] GUI layout unit tests (no display): for ≥ 30 words and several window widths, focus-centre x equals the centre line within the documented rounding rule; baseline y identical for all words; ticks share the focus x.
- [ ] [R21,R23,R24] With `Xvfb :99 &` and `DISPLAY=:99`, `fastread --ui gui "hello world"` starts and stays alive ≥ 2 s, `xdotool` sends `q` and the process exits 0; a screenshot-free pixel check (`xwd`/`import` if available, else reported "not run") is optional.
- [ ] [R24] `env -u DISPLAY -u WAYLAND_DISPLAY fastread --ui gui "x"` prints exactly one line to stderr and exits 1; the `nogui` binary does the same.
- [ ] [R26] Unit tests of the key handler: Up/Down clamp at 1500/50, `[`/`]` clamp at 1/5, Left/Right step 1 paused and 10 playing and clamp at bounds, Home resets to 0, `p`/`?` toggle, `q`/Esc/Ctrl+C quit.
- [ ] [R27,R28] Golden frames with progress on/off and help on/off show the same focus column; `--no-progress` frame has no `word 1/` text; effective-wpm test with a fake clock.
- [ ] [R29] Tests with temp `XDG_STATE_HOME`: save/load round-trip; atomic write leaves no partial file on simulated failure; changed contents ignore the saved index; `--start` and `--no-resume` override; finishing deletes the entry; stdin and raw text create no file; modes 0700/0600.
- [ ] [R30] `go list -deps ./internal/{input,tokenize,orp,timing,state}` contains no `internal/tui`, `internal/gui` or `gioui.org`; `grep -rl gioui.org --include=*.go . ` lists only files under `internal/gui/`.
- [ ] [R31] `go build -tags nogui ./...` and `CGO_ENABLED=0 go build -tags nogui ./...` exit 0, and `go version -m` / `go tool nm` of the nogui binary shows no `gioui.org` symbols.
- [ ] [R32] A leak test starts and stops the TUI loop (with a fake terminal) and finds no extra goroutines; `kill -TERM` on a running binary exits promptly (< 1 s) with the state saved.
- [ ] [R12] Error-wrapping tests: `errors.Is` matches `ErrNotFound`, `ErrCorruptEPUB`, `ErrEmpty`, `ErrUnsupported` through the full call chain.
- [ ] Performance: 5 MB text load + tokenize < 2 s (benchmark or timed test); timing-drift test within ±5% (fake clock or real clock with generous bound).
- [ ] [R1–R33] **Everything-check**, exit 0 only if healthy: `test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go test -race ./... && go test -tags nogui ./... && go build ./... && go build -tags nogui ./... && CGO_ENABLED=0 go build -tags nogui ./...`
- [ ] [R30] `scripts/check-protected.sh` exits 0.
- [ ] Deliverables: `LICENSE` exists and starts with `MIT License`; `docs/usage.md` exists and contains a section for each of: Install/Build, Examples (raw, stdin, txt, md, epub, fb2, pdf), Flags, Keys, Dependencies, Limitations.
- [ ] [R1–R12] The built binary is run on every input type (raw, stdin, txt, md, epub, fb2, pdf) and on the error cases (missing file, empty text, bad flag, scanned PDF); real outputs and exit codes are in the final report.

## 9. Nice-to-haves (not part of v1)

- Wayland-specific automated test (needs a headless compositor such as `weston`/`cage`).
- Pixel-diff screenshot test of the GUI.
- Seeking by percentage (`--start 50%`), chapter navigation for EPUB.
- Configurable timing constants by flag or file.
- Additional fonts (Go Mono) or user-selectable focus colour.
- Shell completions and a man page.
- Release packaging (deb, goreleaser).
