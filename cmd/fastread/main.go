// Command fastread is an RSVP speed reader for the terminal and a Gio window.
package main

import (
	"os"
	"runtime/debug"
)

// version is set with -ldflags "-X main.version=<v>".
var version = ""

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv)) }

// versionString returns "fastread <v>": version if set, else the main module version from
// debug.ReadBuildInfo() unless it is "" or "(devel)", else "dev".
func versionString() string {
	v := version
	if v == "" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			v = bi.Main.Version
		}
	}
	if v == "" {
		v = "dev"
	}
	return "fastread " + v
}
