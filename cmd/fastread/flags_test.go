package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

var parseErrorRows = []struct {
	name string
	args []string
}{
	{"wpm too low", []string{"--wpm", "49"}},
	{"wpm too high", []string{"--wpm", "1501"}},
	{"wpm not int", []string{"--wpm", "x"}},
	{"size too low", []string{"--size", "0"}},
	{"size too high", []string{"--size", "6"}},
	{"ui unknown", []string{"--ui", "foo"}},
	{"start negative", []string{"--start", "-1"}},
	{"unknown flag", []string{"--bogus"}},
	{"two positionals", []string{"a", "b"}},
	{"flag after text", []string{"hello", "--wpm", "100"}},
}

func TestParseFlags(t *testing.T) {
	ok := []struct {
		name string
		args []string
		want options
	}{
		{"defaults", nil, options{wpm: 300, size: 2, ui: "tui"}},
		{
			"all set",
			[]string{"--wpm", "100", "--size", "3", "--ui", "gui", "--start", "5", "--no-resume", "--no-progress", "hello"},
			options{wpm: 100, size: 3, ui: "gui", start: 5, startSet: true, noResume: true, noProgress: true, arg: "hello", hasArg: true},
		},
		{"single dash", []string{"-wpm", "120", "x"}, options{wpm: 120, size: 2, ui: "tui", arg: "x", hasArg: true}},
		{"double dash ends flags", []string{"--", "--wpm"}, options{wpm: 300, size: 2, ui: "tui", arg: "--wpm", hasArg: true}},
		{"start zero", []string{"--start", "0", "x"}, options{wpm: 300, size: 2, ui: "tui", start: 0, startSet: true, arg: "x", hasArg: true}},
		{"help short", []string{"-h"}, options{wpm: 300, size: 2, ui: "tui", help: true}},
		{"help long", []string{"--help"}, options{wpm: 300, size: 2, ui: "tui", help: true}},
		{"version", []string{"--version"}, options{wpm: 300, size: 2, ui: "tui", version: true}},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseFlags(tc.args)
			if err != nil {
				t.Fatalf("parseFlags(%q) error: %v", tc.args, err)
			}
			if got != tc.want {
				t.Errorf("parseFlags(%q) = %+v, want %+v", tc.args, got, tc.want)
			}
		})
	}
	for _, tc := range parseErrorRows {
		t.Run("error/"+tc.name, func(t *testing.T) {
			_, err := parseFlags(tc.args)
			if err == nil {
				t.Fatalf("parseFlags(%q) = nil error, want error", tc.args)
			}
			if strings.Contains(err.Error(), "\n") {
				t.Errorf("error message is not one line: %q", err)
			}
		})
	}
}

func TestRunUsage(t *testing.T) {
	getenv := func(string) string { return "" }
	for _, tc := range parseErrorRows {
		t.Run("error/"+tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := run(tc.args, strings.NewReader(""), &out, &errb, getenv)
			if code != 2 {
				t.Errorf("exit = %d, want 2", code)
			}
			first, _, _ := strings.Cut(errb.String(), "\n")
			if !strings.HasPrefix(first, "fastread: ") {
				t.Errorf("stderr first line = %q, want prefix %q", first, "fastread: ")
			}
			if !strings.Contains(errb.String(), "Usage:") {
				t.Errorf("stderr lacks usage: %q", errb.String())
			}
			if out.Len() != 0 {
				t.Errorf("stdout = %q, want empty", out.String())
			}
		})
	}
	flagNames := []string{"--wpm", "--size", "--ui", "--start", "--no-resume", "--no-progress", "-h", "--help", "--version"}
	for _, h := range []string{"-h", "--help"} {
		t.Run("help/"+h, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run([]string{h}, strings.NewReader(""), &out, &errb, getenv); code != 0 {
				t.Errorf("exit = %d, want 0", code)
			}
			if !strings.Contains(out.String(), "Usage:") {
				t.Errorf("stdout lacks usage: %q", out.String())
			}
			for _, f := range flagNames {
				if !strings.Contains(out.String(), f) {
					t.Errorf("usage lacks %s", f)
				}
			}
			if errb.Len() != 0 {
				t.Errorf("stderr = %q, want empty", errb.String())
			}
		})
	}
	t.Run("version", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := run([]string{"--version"}, strings.NewReader(""), &out, &errb, getenv); code != 0 {
			t.Errorf("exit = %d, want 0", code)
		}
		if !regexp.MustCompile(`^fastread \S+\n$`).MatchString(out.String()) {
			t.Errorf("stdout = %q, want %q", out.String(), "fastread <v>\\n")
		}
		if errb.Len() != 0 {
			t.Errorf("stderr = %q, want empty", errb.String())
		}
	})
}
