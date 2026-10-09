# fastread guide: install and how to use it

This guide is for people who want to build and use `fastread`. It is not part of the
spec-driven Claude loop-coding workflow; for the full flag, key and behaviour reference see
[usage.md](usage.md).

## What it is

`fastread` is a Linux command-line speed reader (RSVP). It shows one word at a time at a fixed
position with one red focus letter, so your eyes never move. It reads raw text, stdin, and
`.txt`, `.md`, `.epub`, `.fb2` and text-layer `.pdf` files, in the terminal (default) or in a
window (`--ui gui`).

## 1. Install

### Requirements

- Linux
- Go 1.24.1 or newer (`go version`). Debian/Ubuntu's `golang` package may be older; install a
  newer Go from <https://go.dev/dl/> if needed.
- Git

### Option A: terminal UI only (simplest)

No C compiler and no system libraries needed.

```sh
git clone https://github.com/0x6c6d/fastread.git
cd fastread
CGO_ENABLED=0 go build -tags nogui -o fastread ./cmd/fastread
```

`--ui gui` is not available in this build (it prints an error and exits 1).

### Option B: terminal UI and GUI

The GUI uses Gio through cgo, so you need a C compiler and some system libraries. On
Debian/Ubuntu:

```sh
sudo apt install gcc pkg-config libwayland-dev libxkbcommon-dev libegl1-mesa-dev \
  libx11-dev libxkbcommon-x11-dev libxcursor-dev libxfixes-dev libvulkan-dev libx11-xcb-dev
go build -o fastread ./cmd/fastread
```

### Put it on your PATH

```sh
install -Dm755 fastread ~/.local/bin/fastread   # make sure ~/.local/bin is on PATH
fastread --version
```

Or install straight from the module path with Go (builds the full version when cgo and the
libraries are available):

```sh
go install github.com/0x6c6d/fastread/cmd/fastread@latest
```

### Check the build (optional)

```sh
go test ./...
go test -tags nogui ./...
```

The end-to-end tests (`go test -tags e2e ./e2e/...`) also need `tmux`, `xvfb` and `xdotool`.

## 2. How to use it

### Read something

```sh
fastread "Hello wonderful world"        # raw text (quote it)
fastread notes.txt                      # plain text
fastread README.md                      # Markdown, syntax stripped
fastread book.epub                      # EPUB (no DRM)
fastread book.fb2                       # FictionBook 2
fastread paper.pdf                      # PDF with a text layer (not scanned)
cat article.txt | fastread              # from stdin
fastread --ui gui book.epub             # open a window instead of the terminal
```

If the argument is an existing file it is loaded; otherwise it is read as raw text. Quote
multi-word text and put flags before it.

### Set speed and size

```sh
fastread --wpm 450 --size 3 book.epub
```

| Flag | Meaning | Default | Range |
|---|---|---|---|
| `--wpm N` | words per minute | 300 | 50-1500 |
| `--size N` | glyph size | 2 | 1-5 |
| `--ui tui\|gui` | interface | `tui` | |
| `--start N` | start at 0-based word N | | |
| `--no-resume` | ignore the saved position | | |
| `--no-progress` | hide the progress row | | |

### Keys while reading

| Key | Action |
|---|---|
| Space | pause / resume |
| Up / Down | speed +25 / -25 wpm |
| `[` / `]` | smaller / bigger text |
| Left / Right | previous / next word (10 words while playing) |
| Home | restart |
| `p` | show / hide progress |
| `?` | show / hide help line |
| `q`, Esc, Ctrl+C | quit (position is saved) |

### Resume

For files, `fastread` remembers where you stopped and continues there next time (stored under
`$XDG_STATE_HOME/fastread/`, or `~/.local/state/fastread/`). Use `--no-resume` to start at the
beginning or `--start N` to pick a word. Raw text and stdin are never saved.

### Exit codes

`0` ok, `1` runtime error (file not found, unsupported or corrupt file, empty text, no
display, ...), `2` usage error.

## 3. Troubleshooting

- **`GUI not available in this build`**: you built with `nogui` or without cgo. Use Option B.
- **`no display: neither DISPLAY nor WAYLAND_DISPLAY is set`**: `--ui gui` needs a graphical
  session.
- **`no extractable text (scanned PDF?)`**: the PDF has no text layer; fastread does no OCR.
- **Build fails with missing `x11-xcb` or `wayland` headers**: install the packages from
  Option B, or use the TUI-only build.
- **Colours look wrong**: fastread reads `NO_COLOR`, `COLORTERM` and `TERM`; see
  [usage.md](usage.md#flags).
