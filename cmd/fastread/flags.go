package main

import (
	"flag"
	"fmt"
	"io"
)

// Flag ranges and defaults.
const (
	defaultWPM  = 300
	minWPM      = 50
	maxWPM      = 1500
	defaultSize = 2
	minSize     = 1
	maxSize     = 5
	defaultUI   = "tui"
)

type options struct {
	wpm, size            int    // defaults 300 and 2
	ui                   string // "tui" (default) or "gui"
	start                int    // valid only if startSet
	startSet             bool
	noResume, noProgress bool
	help, version        bool
	arg                  string // the positional argument
	hasArg               bool
}

// parseFlags parses args (without the program name). Every error it returns is a usage
// error (exit 2) with a one-line message.
func parseFlags(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("fastread", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	fs.IntVar(&o.wpm, "wpm", defaultWPM, "")
	fs.IntVar(&o.size, "size", defaultSize, "")
	fs.StringVar(&o.ui, "ui", defaultUI, "")
	fs.IntVar(&o.start, "start", 0, "")
	fs.BoolVar(&o.noResume, "no-resume", false, "")
	fs.BoolVar(&o.noProgress, "no-progress", false, "")
	fs.BoolVar(&o.help, "h", false, "")
	fs.BoolVar(&o.help, "help", false, "")
	fs.BoolVar(&o.version, "version", false, "")

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "start" {
			o.startSet = true
		}
	})

	switch rest := fs.Args(); len(rest) {
	case 0:
	case 1:
		o.arg, o.hasArg = rest[0], true
	default:
		return options{}, fmt.Errorf("too many arguments (%d): quote raw text with spaces, and put flags before the text", len(rest))
	}

	if o.wpm < minWPM || o.wpm > maxWPM {
		return options{}, fmt.Errorf("--wpm %d out of range %d-%d", o.wpm, minWPM, maxWPM)
	}
	if o.size < minSize || o.size > maxSize {
		return options{}, fmt.Errorf("--size %d out of range %d-%d", o.size, minSize, maxSize)
	}
	if o.ui != "tui" && o.ui != "gui" {
		return options{}, fmt.Errorf("--ui %q invalid: want tui or gui", o.ui)
	}
	if o.startSet && o.start < 0 {
		return options{}, fmt.Errorf("--start %d invalid: must be >= 0", o.start)
	}
	return o, nil
}

// usage writes the usage text: "Usage: fastread [flags] [text | file]", one line per flag
// with default and range, and a note that raw text with spaces must be quoted.
func usage(w io.Writer) {
	fmt.Fprintf(w, `Usage: fastread [flags] [text | file]

Shows text one word at a time (RSVP). The argument is a file path or raw text;
without it, text is read from stdin. Raw text with spaces must be quoted.

Flags (before the text; -- ends flags):
  --wpm N          words per minute, %d-%d (default %d)
  --size N         glyph size level, %d-%d (default %d)
  --ui tui|gui     user interface (default %s)
  --start N        start at 0-based word index N, >= 0 (overrides resume)
  --no-resume      ignore the saved resume position at start
  --no-progress    hide the progress indicator
  -h, --help       show this help and exit
  --version        print the version and exit
`, minWPM, maxWPM, defaultWPM, minSize, maxSize, defaultSize, defaultUI)
}
