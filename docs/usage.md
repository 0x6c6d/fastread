# fastread usage

`fastread` is a Linux command-line RSVP (rapid serial visual presentation) speed reader: it
shows one word at a time at a fixed position, with one red focus letter and all other letters
white, so your eyes never have to move. It reads raw text, stdin and `.txt`, `.md`, `.epub`,
`.fb2` and text-layer `.pdf` files. It runs in the terminal by default and opens a Gio window
with `--ui gui`.

## Install and build

fastread runs on Linux only. It needs Go 1.24.1 or newer (`go.mod` says `go 1.24.1`,
`toolchain go1.24.4`).

Full build (terminal UI and GUI). The GUI uses Gio through cgo, so this build needs a C
compiler, `pkg-config` and the Gio system libraries. On Debian/Ubuntu:

```sh
sudo apt install golang gcc pkg-config libwayland-dev libxkbcommon-dev libegl1-mesa-dev libx11-dev libxkbcommon-x11-dev libxcursor-dev libxfixes-dev libvulkan-dev xvfb xdotool tmux
sudo apt install libx11-xcb-dev   # Gio also needs x11-xcb, which the line above lacks
go build -o fastread ./cmd/fastread
```

The distribution's `golang` package may be older than 1.24; install a newer Go if
`go version` reports less.

TUI-only build: no cgo and no system libraries. The GUI code is left out (`nogui` tag, and
also whenever cgo is off), so `--ui gui` prints
`fastread: GUI not available in this build (built with nogui or without cgo)` and exits 1.

```sh
CGO_ENABLED=0 go build -tags nogui -o fastread ./cmd/fastread
```

Toolchain: set `GOTOOLCHAIN=local` (for example `GOTOOLCHAIN=local go build ...`) to build
with the installed Go only; Go then never downloads another toolchain and fails instead if
the installed one is too old.

The version printed by `--version` is `fastread ` plus the value set with
`-ldflags "-X main.version=v1.0.0"`, else the module version from the build info, else `dev`.

Tests:

```sh
go test ./...                 # unit and golden tests, no display or tmux needed
go test -race ./...
go test -tags nogui ./...
go test -tags e2e -count=1 -timeout 600s ./e2e/...   # end-to-end, needs tmux, Xvfb and xdotool
```

The e2e tests build the binary into a temporary directory and drive it through their own tmux
server and their own Xvfb display, with a temporary `XDG_STATE_HOME`.

## Examples

```sh
fastread "Hello wonderful world"          # raw text (quote text with spaces)
printf 'Read this from a pipe.' | fastread # stdin
cat notes.txt | fastread                  # stdin from a file
fastread notes.txt                        # plain text
fastread README.md                        # Markdown, syntax stripped
fastread book.epub                        # EPUB (no DRM)
fastread book.fb2                         # FictionBook 2
fastread paper.pdf                        # PDF with a text layer
fastread --ui gui book.epub               # Gio window instead of the terminal
fastread --wpm 450 --size 3 notes.txt     # faster, bigger glyphs
fastread --start 120 book.epub            # start at word 120 (0-based), ignore the saved position
fastread --no-resume book.epub            # start at word 0 even if a position was saved
fastread --no-progress "short text here"  # hide the progress row
fastread -- --wpm
```

The last line shows the raw text `--wpm` (one word): `--` ends the flags, so an argument that
starts with `--` is read as text.

### Text or file?

fastread takes at most one positional argument, and flags must come before it (`--` ends the
flags; quote multi-word text, otherwise it exits 2 with `too many arguments`). The argument is
resolved in this order. If it names an existing regular file, the file is loaded; an existing
directory (or FIFO, socket, device) is an unsupported file type, exit 1. Otherwise, if it is
path-like, fastread prints `fastread: <argument>: file not found` and exits 1; path-like means
the argument contains no whitespace and either contains `/`, starts with `.` or `~`, or ends
case-insensitively in `.txt`, `.md`, `.epub`, `.fb2` or `.pdf`. Anything else is raw text. With
no argument, text is read from stdin when stdin is not a terminal; when stdin is a terminal,
fastread prints `fastread: no input: give text or a file, or pipe text on stdin` plus the usage
and exits 2. There is no tilde expansion and `-` is not an alias for stdin (it is the raw text
`-`).

The file type is detected from the extension and the magic bytes of the first 64 KiB:

- `.epub` needs a zip header (`PK\x03\x04`), `.pdf` needs `%PDF-`, and `.fb2` needs a
  `FictionBook` root element within the first 4 KiB; a mismatch is
  `unsupported file type: extension ".pdf" but content is plain text` (and similar), exit 1.
- `.txt` and `.md` are text, unless the content carries EPUB, PDF or FB2 magic (mismatch,
  exit 1).
- Any other extension, or none, is sniffed: EPUB, PDF or FB2 magic picks that loader; valid
  UTF-8 without NUL bytes is plain text; anything else is an unsupported file type.

Text input (raw text, stdin, `.txt`) is decoded as UTF-8 (invalid bytes become U+FFFD, a BOM
is dropped); blank lines separate paragraphs. Markdown loses its syntax, and code blocks and
inline code are dropped. EPUB reads the linear spine documents in order. FB2 reads the `body`
elements only. PDF reads the text layer page by page.

## Flags

Output of `fastread --help`, as a table:

| Flag | Meaning | Default | Range |
|---|---|---|---|
| `--wpm N` | words per minute | 300 | 50-1500 |
| `--size N` | glyph size level | 2 | 1-5 |
| `--ui tui\|gui` | user interface | `tui` | `tui` or `gui` |
| `--start N` | start at 0-based word index N (overrides resume) | none | >= 0 and below the word count |
| `--no-resume` | do not use the saved resume position | off | |
| `--no-progress` | hide the progress indicator | off (progress shown) | |
| `-h`, `--help` | show the help on stdout and exit 0 | | |
| `--version` | print `fastread <version>` and exit 0 | | |

Flags are parsed by Go's `flag` package: one or two dashes both work, and a value may follow
`=` (`--wpm=450`). An unknown flag, a bad value, a value out of range, more than one argument,
or `--start` at or beyond the last word (`--start 9999 is beyond the last word (text has 12
words)`) prints one `fastread: ...` line and the usage on stderr and exits 2.

Exit codes:

| Exit code | Meaning |
|---|---|
| 0 | read to the end, quit by key, closed the window, SIGINT or SIGTERM; also `--help`, `--version` |
| 1 | runtime error: file not found, unsupported or corrupt file, empty text (`empty text: no words to read`), input too large, no terminal, no display, GUI not available, saving the position failed |
| 2 | usage error (see above), or no argument with stdin on a terminal |

Every error is exactly one line on stderr starting with `fastread: `.

Resume. For file sources only, fastread remembers where you stopped:

- The state directory is `$XDG_STATE_HOME/fastread/` when `XDG_STATE_HOME` is an absolute
  path, else `~/.local/state/fastread/` (from `HOME`). A relative `XDG_STATE_HOME` is ignored.
  When neither works, fastread prints `fastread: warning: resume disabled: ...` and reads on.
- Each file has one entry, `<sha256 of the path>.json`, holding the absolute, symlink-resolved
  path, the SHA-256 of the file contents and the word index:
  `{"path":"/home/me/book.epub","sha256":"…","index":120}`. No text is stored.
- The directory is created with mode 0700 and entries with mode 0600; entries are written
  atomically (temporary file plus rename).
- Raw text and stdin are never saved; nothing is created on disk for them.
- On quit (key, window close, SIGINT, SIGTERM) the index of the word on screen is saved.
  Reaching the end of the text deletes the entry.
- Start position: `--start N` wins and the saved entry is not read; else `--no-resume` starts
  at word 0; else the saved index is used if the file's SHA-256 still matches and the index is
  inside the text; else word 0. An unreadable or corrupt entry prints one
  `fastread: warning: ignoring saved position: ...` line and counts as nothing saved.
- As built, `--no-resume` and `--start` only affect where reading starts: the position at
  quit is still saved (and the entry deleted at the end of the text), even though `--help`
  says `--no-resume` does not save.

Environment variables read:

| Variable | Use |
|---|---|
| `NO_COLOR` | non-empty: no colour escapes in the terminal; the focus letter is bold and reversed |
| `COLORTERM` | `truecolor` or `24bit`: 24-bit red focus letter |
| `TERM` | contains `256color`: 256-colour red; otherwise 16-colour red |
| `DISPLAY`, `WAYLAND_DISPLAY` | `--ui gui` needs one of them non-empty, else `fastread: no display: neither DISPLAY nor WAYLAND_DISPLAY is set`, exit 1 |
| `XDG_STATE_HOME` | resume directory (absolute paths only) |
| `HOME` | fallback resume directory `~/.local/state/fastread/` |

## Keys

The same keys work in the terminal and in the window. Playback starts playing.

| Key | Action |
|---|---|
| Space | pause / resume |
| Up / Down | speed +25 / -25 wpm, clamped to 50-1500 |
| `[` / `]` | size level -1 / +1, clamped to 1-5 |
| Left / Right | previous / next word: 1 word when paused, 10 words when playing (clamped to the text) |
| Home | restart at word 0 |
| `p` | show / hide the progress row |
| `?` | show / hide the help line |
| `q`, Esc, Ctrl+C | quit (the position is saved) |

In the terminal a lone Esc is recognised after 50 ms without further bytes; keys are read
from the controlling terminal (`/dev/tty`), so they work while the text comes from a pipe.

The help line (hidden at start, top row of the terminal, top left of the window) reads:

```
space pause  ↑↓ wpm  [ ] size  ←→ word  home restart  p progress  ? help  q quit
```

The progress row (shown unless `--no-progress`; bottom row of the terminal, bottom left of
the window) is `word N/T  E wpm`: the 1-based word number, the word count and the effective
speed (words completed per minute of unpaused playing time), or `—` until at least two words
were shown, for example `word 12/3400  287 wpm`. In the terminal a bar of `━` (read) and `─`
(unread) fills the rest of the row when the terminal is at least two columns wider than the
text; the window draws a thin grey bar above the text.

## Dependencies

Go modules (every `require` line of `go.mod`):

- `gioui.org` v0.10.3: the GUI toolkit (window, input, text shaping, GPU rendering); only
  `internal/gui` imports it.
- `github.com/ledongthuc/pdf` (pseudo-version 2026-09-07): extracts the text layer of PDFs.
- `github.com/rivo/uniseg` v0.4.7: grapheme clusters and display widths for the focus letter
  and terminal layout.
- `golang.org/x/image` v0.36.0: the embedded Go Regular font (`font/gofont/goregular`) and
  26.6 fixed-point layout math.
- `golang.org/x/net` v0.50.0: the HTML/XHTML parser for EPUB content documents.
- `golang.org/x/term` v0.40.0: raw mode, terminal size and TTY detection.
- `gioui.org/shader` v1.0.9 (indirect): GPU shaders, needed by `gioui.org`.
- `github.com/go-text/typesetting` v0.3.5 (indirect): font shaping, needed by `gioui.org`.
- `github.com/godbus/dbus/v5` v5.2.2 (indirect): D-Bus (IBus input method), needed by
  `gioui.org`.
- `golang.org/x/exp/shiny` (indirect): icon vector graphics, needed by `gioui.org`.
- `golang.org/x/sys` v0.41.0 (indirect): system calls, needed by `golang.org/x/term`,
  `golang.org/x/net` and `gioui.org`.
- `golang.org/x/text` v0.34.0 (indirect): bidi and Unicode tables, needed by `gioui.org`,
  `golang.org/x/net` and `golang.org/x/image`.

System libraries of the full (GUI) build, linked dynamically: libEGL, libwayland-client,
libwayland-egl, libwayland-cursor, libxkbcommon, libxkbcommon-x11, libX11, libX11-xcb,
libXcursor, libXfixes (development packages in the apt line above, plus `libx11-xcb-dev`).
The TUI-only build links none of them.

Test tools: `tmux` (terminal frame capture), `Xvfb` (virtual X display) and `xdotool` (key
presses to the window); only the e2e tests need them.

## GUI font

The window draws all text with Go Regular (`golang.org/x/image/font/gofont/goregular`),
embedded in the binary; the shaper is created with no system fonts. This keeps rendering and
measurements identical on every machine, needs no font packages, and the focus maths can be
tested without a display. Licence: `third_party/gofont/LICENSE` (BSD-style, Go project).

Word size per level, in sp:

| Level | 1 | 2 | 3 | 4 | 5 |
|---|---|---|---|---|---|
| Size | 20 sp | 32 sp | 48 sp | 64 sp | 96 sp |

A word that does not fit the window width is shrunk in 2 sp steps down to 10 sp; if it still
does not fit at 10 sp it is split into hyphenated parts shown one after another. The help and
progress lines use 14 sp. The window opens at 600 x 300 dp, titled `fastread`, white text on
black with the focus letter in red.

Focus rounding: the centre line is `width/2` (integer division of the window width in pixels).
The pen starts at `width/2 - prefix - advance/2`, where prefix is the advance of the clusters
before the focus cluster and `advance/2` is integer division in 26.6 fixed-point units, so the
centre of the focus cluster lies on the centre line. The baseline is
`height/2 + (ascent - descent)/2`; two short white ticks mark the centre line above and
below the word.

The terminal UI needs no font: level 1 is ordinary text, levels 2-5 are block glyphs (`█`,
`▀`, `▄`) generated from the X11 misc-fixed bitmap fonts, which are public domain: 4x6 (level
2, 4x3 cells), 6x10 (level 3, 6x5 cells), 7x14 (level 4, 8x7 cells) and 9x18 (level 5, 10x9
cells). They cover Latin-1 only (U+0020-U+007E, U+00A0-U+00FF); a word with any other
character is drawn at level 1. Notice: `third_party/misc-fixed/LICENSE`.

In the terminal the focus column is `width/2`. The level drops while the glyph (plus 4 free
rows) does not fit the height or two glyph widths do not fit beside the centre; a word wider
than the terminal is split into hyphenated parts. A terminal smaller than 20 x 5 shows
`terminal too small` until it is resized.

## Limitations

- Linux only; no Windows or macOS builds.
- Scanned PDFs have no text layer and there is no OCR: fastread exits 1 with
  `no extractable text (scanned PDF?)`. Encrypted PDFs are rejected
  (`encrypted PDF not supported`). At most 10000 pages are read; later pages are ignored.
- DRM-encrypted EPUBs (spine documents listed in `META-INF/encryption.xml`) are rejected with
  `encrypted EPUB (DRM) not supported`; nothing is decrypted.
- No right-to-left (RTL) shaping: Arabic or Hebrew words are shown in logical order, and bidi
  control characters are removed.
- The GUI is tested on X11 under Xvfb only; Wayland is supported by Gio but not verified.
- A missing GUI shared library (for example `libX11-xcb.so.1`) makes the dynamic linker fail
  before fastread runs, with the linker's own message instead of a `fastread:` line; use the
  TUI-only build on such systems.
- Input limit 256 MiB for files and stdin (`input too large`), also for the total decompressed
  size of an EPUB and the extracted text of a PDF; EPUBs may have at most 10000 zip entries;
  XML in EPUB and FB2 may nest at most 256 elements deep; XML must be UTF-8 or ASCII.
- `.fb2.zip` (zipped FictionBook) is not supported; unzip it first. As a zip archive it is
  taken for an EPUB and rejected as a corrupt EPUB.
- Timing: the long-word bonus (+5 % per letter beyond 8, at most +50 %) counts letter and digit
  runes, not grapheme clusters, so text with conjoining jamo (Korean) or Indic spacing marks
  can get slightly different delays than its visible letters suggest. Other multipliers:
  sentence end (`.`, `!`, `?`, `…`) x2, clause end (`,`, `;`, `:`) x1.5, last word of a
  paragraph x2.5.
- No configuration file, themes, man page or shell completion; `--start` takes a word index,
  not a percentage; EPUB chapters cannot be navigated.
