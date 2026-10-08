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

// usageError marks a usage error: exit 2, usage printed after the error line.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

// exitCode maps an error: nil → 0; usageError or input.ErrNoInput in the chain → 2; else 1.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ue usageError
	if errors.As(err, &ue) || errors.Is(err, input.ErrNoInput) {
		return 2
	}
	return 1
}

// run is the whole program; main only exits with the code it returns. It is runErr plus
// one "fastread: <message>" line for a non-zero code, followed by the usage for code 2.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	code, err := runErr(args, stdin, stdout, stderr, getenv)
	if err != nil {
		errLine(stderr, err)
		if code == 2 {
			usage(stderr)
		}
	}
	return code
}

// prepare is the part of the flow before any UI starts: Select, Load, Tokenize, the empty
// check (error wrapping input.ErrEmpty) and the --start range check (usageError).
func prepare(o options, stdin io.Reader, isTTY bool) (input.Document, []tokenize.Token, error) {
	src, err := input.Select(o.arg, o.hasArg, isTTY)
	if err != nil {
		return input.Document{}, nil, err
	}
	doc, err := input.Load(src, stdin)
	if err != nil {
		return input.Document{}, nil, err
	}
	tokens := tokenize.Tokenize(doc.Paragraphs)
	if len(tokens) == 0 {
		if src.Kind == input.KindFile {
			return doc, nil, fmt.Errorf("%s: %w: no words to read", src.Path, input.ErrEmpty)
		}
		return doc, nil, fmt.Errorf("%w: no words to read", input.ErrEmpty)
	}
	if o.startSet && o.start >= len(tokens) {
		return doc, tokens, usageError{fmt.Errorf("--start %d is beyond the last word (text has %d words)", o.start, len(tokens))}
	}
	return doc, tokens, nil
}

// runErr is run without printing errors: it prints only help/version output and returns
// the exit code and the error that caused a non-zero code (nil when the code is 0).
func runErr(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) (int, error) {
	opts, err := parseFlags(args)
	if err != nil {
		var ue usageError
		if !errors.As(err, &ue) {
			err = usageError{err}
		}
		return exitCode(err), err
	}
	if opts.help {
		usage(stdout)
		return 0, nil
	}
	if opts.version {
		fmt.Fprintln(stdout, versionString())
		return 0, nil
	}

	_, tokens, err := prepare(opts, stdin, stdinIsTTY(stdin))
	if err != nil {
		return exitCode(err), err
	}
	start := 0
	if opts.startSet {
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
			return exitCode(err), err
		}
		return 0, nil
	}

	t, err := openTerminal()
	if err != nil {
		return 1, err
	}
	if _, err := tui.Run(ctx, t, p, tui.Options{Getenv: getenv}); err != nil {
		return 1, err
	}
	return 0, nil
}
