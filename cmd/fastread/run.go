package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/0x6c6d/fastread/internal/gui"
	"github.com/0x6c6d/fastread/internal/input"
	"github.com/0x6c6d/fastread/internal/state"
	"github.com/0x6c6d/fastread/internal/tokenize"
	"github.com/0x6c6d/fastread/internal/tui"
)

// Seams replaced by tests.
var (
	openTerminal = tui.OpenTTY
	stdinIsTTY   = func(r io.Reader) bool {
		f, ok := r.(*os.File)
		return ok && f != nil && term.IsTerminal(int(f.Fd()))
	}
	exitProcess = os.Exit // only the GUI finish callback calls it
)

// errLine writes err as exactly one "fastread: <message>" line.
func errLine(w io.Writer, err error) {
	msg := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(err.Error())
	fmt.Fprintf(w, "fastread: %s\n", msg)
}

// run is the whole program; main only exits with the code it returns.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	opts, err := parseFlags(args)
	if err != nil {
		errLine(stderr, err)
		usage(stderr)
		return 2
	}
	if opts.help {
		usage(stdout)
		return 0
	}
	if opts.version {
		fmt.Fprintln(stdout, versionString())
		return 0
	}

	src, err := input.Select(opts.arg, opts.hasArg, stdinIsTTY(stdin))
	if err != nil {
		errLine(stderr, err)
		if errors.Is(err, input.ErrNoInput) {
			usage(stderr)
			return 2
		}
		return 1
	}
	doc, err := input.Load(src, stdin)
	if err != nil {
		errLine(stderr, err)
		return 1
	}
	tokens := tokenize.Tokenize(doc.Paragraphs)
	if len(tokens) == 0 {
		errLine(stderr, fmt.Errorf("%w: no words to read", input.ErrEmpty))
		return 1
	}
	start := 0
	if opts.startSet {
		if opts.start >= len(tokens) {
			errLine(stderr, fmt.Errorf("--start %d is beyond the last word (text has %d words)", opts.start, len(tokens)))
			usage(stderr)
			return 2
		}
		start = opts.start
	}
	// TODO(phase3): resume (load saved position unless --no-resume, save on quit, delete at end).

	p := state.NewPlayer(state.Config{
		Tokens:       tokens,
		Start:        start,
		WPM:          opts.wpm,
		Size:         opts.size,
		ShowProgress: !opts.noProgress,
		Clock:        state.SystemClock{},
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if opts.ui == "gui" {
		finish := func(last int, err error) {
			_ = last // TODO(phase3): resume
			if err != nil {
				errLine(stderr, err)
				exitProcess(1)
				return
			}
			exitProcess(0)
		}
		if err := gui.Run(ctx, p, gui.Options{Getenv: getenv}, finish); err != nil {
			errLine(stderr, err)
			return 1
		}
		return 0
	}

	t, err := openTerminal()
	if err != nil {
		errLine(stderr, err)
		return 1
	}
	if _, err := tui.Run(ctx, t, p, tui.Options{Getenv: getenv}); err != nil {
		errLine(stderr, err)
		return 1
	}
	return 0
}
