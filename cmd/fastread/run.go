package main

import (
	"fmt"
	"io"
)

// run is the whole program; main only exits with the code it returns.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	opts, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "fastread: %v\n", err)
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
	_, _ = stdin, getenv
	// TODO(T010): wiring
	fmt.Fprintln(stderr, "fastread: not implemented yet")
	return 1
}
