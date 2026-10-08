package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/gui"
	"github.com/0x6c6d/fastread/internal/state"
)

// guiCall is one finish call made by the fake runGUI; last < 0 plays to the end first.
type guiCall struct {
	last int
	err  error
}

// fakeGUI controls what the replaced runGUI does.
type fakeGUI struct {
	calls   []guiCall
	syncErr error // returned without calling finish
	entered int   // p.Index() on entry
	ran     int
}

// guiSeams replaces runGUI, exitProcess and stdinIsTTY; it returns the recorded exit codes.
func guiSeams(t *testing.T, fg *fakeGUI) *[]int {
	t.Helper()
	og, oe, ot, oo := runGUI, exitProcess, stdinIsTTY, openTerminal
	t.Cleanup(func() { runGUI, exitProcess, stdinIsTTY, openTerminal = og, oe, ot, oo })
	openTerminal = uiReached // the GUI path must never open a terminal
	var codes []int
	exitProcess = func(c int) { codes = append(codes, c) }
	stdinIsTTY = func(io.Reader) bool { return false }
	runGUI = func(_ context.Context, p *state.Player, _ gui.Options, finish func(int, error)) error {
		fg.ran++
		fg.entered = p.Index()
		if fg.syncErr != nil {
			return fg.syncErr
		}
		for _, c := range fg.calls {
			last := c.last
			if last < 0 {
				last = playToEnd(p)
			}
			finish(last, c.err)
		}
		return nil
	}
	return &codes
}

// loadEntry reads the saved index of book from <tmp>/state/fastread via the store.
func loadEntry(t *testing.T, tmp, book string) (int, bool) {
	t.Helper()
	b, err := os.ReadFile(book)
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(book)
	if err != nil {
		t.Fatal(err)
	}
	i, ok, err := state.NewStore(filepath.Join(tmp, "state", "fastread")).Load(real, sha256.Sum256(b))
	if err != nil {
		t.Fatalf("load entry: %v", err)
	}
	return i, ok
}

func TestRunGUIFinishSaves(t *testing.T) {
	const book = "BOOK" // replaced by the path of a fresh w00..w19 file
	rows := []struct {
		name      string
		args      []string
		stdin     string
		seed      int  // >= 0: a first GUI run saves this index
		badState  bool // XDG_STATE_HOME lies below a regular file
		fake      fakeGUI
		wantCode  int   // run's return code
		wantExits []int // exitProcess codes
		wantErr   string
		entered   int // -1: not checked
		wantIdx   int // -1: no entry
		noState   bool
	}{
		{name: "quit saves", args: []string{book}, seed: -1,
			fake: fakeGUI{calls: []guiCall{{4, nil}}}, wantExits: []int{0}, entered: 0, wantIdx: 4},
		{name: "resumes saved", args: []string{book}, seed: 4,
			fake: fakeGUI{calls: []guiCall{{6, nil}}}, wantExits: []int{0}, entered: 4, wantIdx: 6},
		{name: "start overrides", args: []string{"--start", "2", book}, seed: 4,
			fake: fakeGUI{calls: []guiCall{{5, nil}}}, wantExits: []int{0}, entered: 2, wantIdx: 5},
		{name: "no-resume", args: []string{"--no-resume", book}, seed: 4,
			fake: fakeGUI{calls: []guiCall{{1, nil}}}, wantExits: []int{0}, entered: 0, wantIdx: 1},
		{name: "end deletes", args: []string{"--wpm", "1500", "--start", "17", book}, seed: 4,
			fake: fakeGUI{calls: []guiCall{{-1, nil}}}, wantExits: []int{0}, entered: 17, wantIdx: -1},
		{name: "window error saves", args: []string{book}, seed: -1,
			fake:      fakeGUI{calls: []guiCall{{3, errors.New("window lost")}}},
			wantExits: []int{1}, wantErr: "window lost", entered: 0, wantIdx: 3},
		{name: "save fails", args: []string{"--no-resume", book}, seed: -1, badState: true,
			fake:      fakeGUI{calls: []guiCall{{1, nil}}},
			wantExits: []int{1}, wantErr: "saving reading position", entered: 0, wantIdx: -1},
		{name: "finish twice", args: []string{book}, seed: -1,
			fake:      fakeGUI{calls: []guiCall{{2, nil}, {5, errors.New("again")}}},
			wantExits: []int{0}, entered: 0, wantIdx: 2},
		{name: "raw no state", args: []string{"a b c"}, seed: -1,
			fake: fakeGUI{calls: []guiCall{{1, nil}}}, wantExits: []int{0}, entered: 0, wantIdx: -1, noState: true},
		{name: "stdin no state", stdin: "a b c", seed: -1,
			fake: fakeGUI{calls: []guiCall{{1, nil}}}, wantExits: []int{0}, entered: 0, wantIdx: -1, noState: true},
		{name: "no display", args: []string{book}, seed: -1,
			fake:     fakeGUI{syncErr: gui.ErrNoDisplay},
			wantCode: 1, wantErr: "no display", entered: 0, wantIdx: -1, noState: true},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := writeBook(t, tmp)
			getenv := tmpEnv(tmp)
			if tc.badState {
				f := filepath.Join(tmp, "file")
				if err := os.WriteFile(f, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				getenv = envOf(map[string]string{"XDG_STATE_HOME": filepath.Join(f, "state")})
			}
			args := []string{"--ui", "gui"}
			for _, a := range tc.args {
				if a == book {
					a = path
				}
				args = append(args, a)
			}

			if tc.seed >= 0 {
				seed := &fakeGUI{calls: []guiCall{{tc.seed, nil}}}
				codes := guiSeams(t, seed)
				var out, errb bytes.Buffer
				if c := run([]string{"--ui", "gui", path}, strings.NewReader(""), &out, &errb, getenv); c != 0 || errb.Len() != 0 || len(*codes) != 1 || (*codes)[0] != 0 {
					t.Fatalf("seed run: code %d, exits %v, stderr %q", c, *codes, errb.String())
				}
			}

			fg := tc.fake
			codes := guiSeams(t, &fg)
			var out, errb bytes.Buffer
			code := run(args, strings.NewReader(tc.stdin), &out, &errb, getenv)
			if code != tc.wantCode {
				t.Errorf("code = %d, want %d", code, tc.wantCode)
			}
			if len(*codes) != len(tc.wantExits) || (len(tc.wantExits) == 1 && (*codes)[0] != tc.wantExits[0]) {
				t.Errorf("exitProcess codes = %v, want %v", *codes, tc.wantExits)
			}
			if tc.wantErr == "" {
				if errb.Len() != 0 {
					t.Errorf("stderr = %q, want empty", errb.String())
				}
			} else if lines(errb.String()) != 1 || !strings.HasPrefix(errb.String(), "fastread: ") || !strings.Contains(errb.String(), tc.wantErr) {
				t.Errorf("stderr = %q, want one fastread: line containing %q", errb.String(), tc.wantErr)
			}
			if out.Len() != 0 {
				t.Errorf("stdout = %q, want empty", out.String())
			}
			if fg.ran != 1 || (tc.entered >= 0 && fg.entered != tc.entered) {
				t.Errorf("runGUI ran %d, started at %d; want 1, %d", fg.ran, fg.entered, tc.entered)
			}
			if tc.noState {
				for _, d := range []string{"state", "home"} {
					if _, err := os.Stat(filepath.Join(tmp, d)); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("%s exists after run (err %v)", d, err)
					}
				}
				return
			}
			if tc.badState {
				return
			}
			i, ok := loadEntry(t, tmp, path)
			if tc.wantIdx < 0 {
				if ok {
					t.Errorf("entry index %d exists, want deleted", i)
				}
				return
			}
			if !ok || i != tc.wantIdx {
				t.Errorf("entry = %d (present %v), want %d", i, ok, tc.wantIdx)
			}
			fi, err := os.Stat(entryOf(t, tmp, path))
			if err != nil {
				t.Fatalf("stat entry: %v", err)
			}
			if fi.Mode().Perm() != 0o600 {
				t.Errorf("entry mode %v, want 0600", fi.Mode().Perm())
			}
		})
	}
}
