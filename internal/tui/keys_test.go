package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/state"
)

type keyRow struct {
	name    string
	chunks  []string
	want    []state.Action
	pending bool
}

func keyRows() []keyRow {
	rows := []keyRow{
		{"space", []string{" "}, []state.Action{state.ActTogglePause}, false},
		{"q", []string{"q"}, []state.Action{state.ActQuit}, false},
		{"ctrl+c", []string{"\x03"}, []state.Action{state.ActQuit}, false},
		{"[", []string{"["}, []state.Action{state.ActSizeDown}, false},
		{"]", []string{"]"}, []state.Action{state.ActSizeUp}, false},
		{"p", []string{"p"}, []state.Action{state.ActToggleProgress}, false},
		{"?", []string{"?"}, []state.Action{state.ActToggleHelp}, false},
		{"CSI A", []string{"\x1b[A"}, []state.Action{state.ActWPMUp}, false},
		{"CSI B", []string{"\x1b[B"}, []state.Action{state.ActWPMDown}, false},
		{"CSI C", []string{"\x1b[C"}, []state.Action{state.ActNext}, false},
		{"CSI D", []string{"\x1b[D"}, []state.Action{state.ActPrev}, false},
		{"SS3 A", []string{"\x1bOA"}, []state.Action{state.ActWPMUp}, false},
		{"SS3 B", []string{"\x1bOB"}, []state.Action{state.ActWPMDown}, false},
		{"SS3 C", []string{"\x1bOC"}, []state.Action{state.ActNext}, false},
		{"SS3 D", []string{"\x1bOD"}, []state.Action{state.ActPrev}, false},
		{"CSI H", []string{"\x1b[H"}, []state.Action{state.ActHome}, false},
		{"SS3 H", []string{"\x1bOH"}, []state.Action{state.ActHome}, false},
		{"CSI 1~", []string{"\x1b[1~"}, []state.Action{state.ActHome}, false},
		{"CSI 7~", []string{"\x1b[7~"}, []state.Action{state.ActHome}, false},
		{"ignored 5~", []string{"\x1b[5~"}, nil, false},
		{"ignored 1;5C", []string{"\x1b[1;5C"}, nil, false},
		{"ignored SS3 P", []string{"\x1bOP"}, nil, false},
		{"ignored 200~", []string{"\x1b[200~"}, nil, false},
		{"lone esc", []string{"\x1b"}, nil, true},
		{"esc esc CSI C", []string{"\x1b\x1b[C"}, []state.Action{state.ActQuit, state.ActNext}, false},
		{"esc x", []string{"\x1bx"}, []state.Action{state.ActQuit}, false},
		{"esc p", []string{"\x1bp"}, []state.Action{state.ActQuit, state.ActToggleProgress}, false},
		{"ctrl+c in seq", []string{"\x1b[\x03"}, []state.Action{state.ActQuit}, false},
		{"size keys", []string{"[]p?"}, []state.Action{state.ActSizeDown, state.ActSizeUp, state.ActToggleProgress, state.ActToggleHelp}, false},
		{"ignored bytes", []string{"Qx9\x7f\xc3\xa9"}, nil, false},
		{"long seq", []string{"\x1b[" + strings.Repeat("1", 40) + "~ "}, []state.Action{state.ActTogglePause}, false},
		{"huge seq", []string{"\x1b[" + strings.Repeat("1", 100000)}, nil, false},
		{"invalid in seq", []string{"\x1b[1\x01 "}, []state.Action{state.ActTogglePause}, false},
		{"esc in seq", []string{"\x1b[1\x1b[A"}, []state.Action{state.ActWPMUp}, false},
		{"split chunks", []string{"\x1b", "[", "A"}, []state.Action{state.ActWPMUp}, false},
	}
	return rows
}

func TestKeyDecode(t *testing.T) {
	for _, r := range keyRows() {
		t.Run(r.name, func(t *testing.T) {
			var d KeyDecoder
			var got []state.Action
			for _, c := range r.chunks {
				got = append(got, d.Feed([]byte(c))...)
				if len(d.buf) > keyMaxBuf {
					t.Fatalf("buffer %d > %d", len(d.buf), keyMaxBuf)
				}
			}
			if !reflect.DeepEqual(got, r.want) {
				t.Fatalf("got %v, want %v", got, r.want)
			}
			if d.Pending() != r.pending {
				t.Fatalf("Pending = %v, want %v", d.Pending(), r.pending)
			}
		})
	}

	t.Run("flush lone esc", func(t *testing.T) {
		var d KeyDecoder
		d.Feed([]byte("\x1b"))
		if got := d.Flush(); !reflect.DeepEqual(got, []state.Action{state.ActQuit}) {
			t.Fatalf("got %v", got)
		}
		if d.Pending() {
			t.Fatal("still pending")
		}
	})
	t.Run("flush unfinished drops", func(t *testing.T) {
		var d KeyDecoder
		d.Feed([]byte("\x1b[1"))
		if got := d.Flush(); len(got) != 0 || d.Pending() {
			t.Fatalf("got %v pending %v", got, d.Pending())
		}
	})
	t.Run("flush empty", func(t *testing.T) {
		var d KeyDecoder
		if got := d.Flush(); len(got) != 0 {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("splits", func(t *testing.T) {
		seqs := []string{"\x1b[A", "\x1b[B", "\x1b[C", "\x1b[D", "\x1bOA", "\x1bOB", "\x1bOC", "\x1bOD",
			"\x1b[H", "\x1bOH", "\x1b[1~", "\x1b[7~", "\x1b[1;5C", "\x1b\x1b[C", "\x1b[200~ "}
		for _, s := range seqs {
			var w KeyDecoder
			want := w.Feed([]byte(s))
			for i := 0; i <= len(s); i++ {
				var d KeyDecoder
				got := append(d.Feed([]byte(s[:i])), d.Feed([]byte(s[i:]))...)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%q split %d: got %v want %v", s, i, got, want)
				}
			}
			var d KeyDecoder
			var got []state.Action
			for i := 0; i < len(s); i++ {
				got = append(got, d.Feed([]byte{s[i]})...)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%q bytewise: got %v want %v", s, got, want)
			}
		}
	})
}

func FuzzKeyDecode(f *testing.F) {
	for _, r := range keyRows() {
		if len(r.chunks) == 1 && len(r.chunks[0]) < 1000 {
			f.Add([]byte(r.chunks[0]))
		}
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		var a, b KeyDecoder
		whole := a.Feed(in)
		var bytewise []state.Action
		for _, c := range in {
			bytewise = append(bytewise, b.Feed([]byte{c})...)
			if len(b.buf) > keyMaxBuf {
				t.Fatalf("buffer %d", len(b.buf))
			}
		}
		if len(a.buf) > keyMaxBuf {
			t.Fatalf("buffer %d", len(a.buf))
		}
		if !reflect.DeepEqual(whole, bytewise) {
			t.Fatalf("whole %v bytewise %v", whole, bytewise)
		}
		for _, act := range whole {
			if act < state.ActTogglePause || act > state.ActQuit {
				t.Fatalf("invalid action %v", act)
			}
		}
		a.Flush()
		if a.Pending() {
			t.Fatal("pending after Flush")
		}
	})
}
