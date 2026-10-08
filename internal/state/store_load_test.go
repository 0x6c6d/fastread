package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

var (
	sumA = sha256.Sum256([]byte("text A"))
	sumB = sha256.Sum256([]byte("text B"))
)

const loadPath = "/home/user/book.txt"

func validEntry(path string, sum [32]byte, index int) []byte {
	b, err := json.Marshal(entry{Path: path, SHA256: hex.EncodeToString(sum[:]), Index: index})
	if err != nil {
		panic(err)
	}
	return b
}

func TestResumeRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fastread")
	s := NewStore(dir)
	for _, idx := range []int{0, 1, 41, 1 << 30} {
		if err := s.Save(loadPath, sumA, idx); err != nil {
			t.Fatalf("Save(%d): %v", idx, err)
		}
		got, ok, err := s.Load(loadPath, sumA)
		if err != nil || !ok || got != idx {
			t.Errorf("Load after Save(%d) = (%d, %v, %v), want (%d, true, nil)", idx, got, ok, err, idx)
		}
	}

	other := "/home/user/other.txt"
	if err := s.Save(loadPath, sumA, 5); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(other, sumA, 9); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		want int
	}{{loadPath, 5}, {other, 9}} {
		got, ok, err := s.Load(tc.path, sumA)
		if err != nil || !ok || got != tc.want {
			t.Errorf("Load(%s) = (%d, %v, %v), want (%d, true, nil)", tc.path, got, ok, err, tc.want)
		}
	}

	if got, ok, err := s.Load("/never/saved.txt", sumA); got != 0 || ok || err != nil {
		t.Errorf("Load(never saved) = (%d, %v, %v), want (0, false, nil)", got, ok, err)
	}

	missing := filepath.Join(t.TempDir(), "no", "such", "dir")
	ms := NewStore(missing)
	if got, ok, err := ms.Load(loadPath, sumA); got != 0 || ok || err != nil {
		t.Errorf("Load on missing dir = (%d, %v, %v), want (0, false, nil)", got, ok, err)
	}
	if _, err := os.Lstat(filepath.Dir(filepath.Dir(missing))); err == nil {
		t.Errorf("Load created %s", filepath.Dir(filepath.Dir(missing)))
	}
}

func TestResumeHashMismatch(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "fastread"))
	if err := s.Save(loadPath, sumA, 12); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		name   string
		save   bool
		sum    [32]byte
		idx    int
		wantI  int
		wantOK bool
	}{
		{name: "load B after save A", sum: sumB, wantI: 0, wantOK: false},
		{name: "load A after save A", sum: sumA, wantI: 12, wantOK: true},
		{name: "save B", save: true, sum: sumB, idx: 30},
		{name: "load B after save B", sum: sumB, wantI: 30, wantOK: true},
		{name: "load A after save B", sum: sumA, wantI: 0, wantOK: false},
	}
	for _, st := range steps {
		if st.save {
			if err := s.Save(loadPath, st.sum, st.idx); err != nil {
				t.Fatalf("%s: %v", st.name, err)
			}
			continue
		}
		got, ok, err := s.Load(loadPath, st.sum)
		if err != nil || ok != st.wantOK || got != st.wantI {
			t.Errorf("%s: Load = (%d, %v, %v), want (%d, %v, nil)", st.name, got, ok, err, st.wantI, st.wantOK)
		}
	}
}

// corruptRows are entry contents that Load and decodeEntry must reject for loadPath/sumA.
func corruptRows() []struct {
	name string
	data []byte
} {
	h := hex.EncodeToString(sumA[:])
	obj := func(s string) []byte { return []byte(s) }
	return []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"open brace", obj("{")},
		{"null", obj("null")},
		{"array", obj("[]")},
		{"string", obj(`"x"`)},
		{"missing index", obj(`{"path":"` + loadPath + `","sha256":"` + h + `"}`)},
		{"missing path", obj(`{"sha256":"` + h + `","index":3}`)},
		{"negative index", obj(`{"path":"` + loadPath + `","sha256":"` + h + `","index":-1}`)},
		{"string index", obj(`{"path":"` + loadPath + `","sha256":"` + h + `","index":"3"}`)},
		{"float index", obj(`{"path":"` + loadPath + `","sha256":"` + h + `","index":1.5}`)},
		{"unknown key", obj(`{"path":"` + loadPath + `","sha256":"` + h + `","index":3,"extra":1}`)},
		{"sha256 63 chars", obj(`{"path":"` + loadPath + `","sha256":"` + h[:63] + `","index":3}`)},
		{"uppercase hex", obj(`{"path":"` + loadPath + `","sha256":"` + strings.ToUpper(h) + `","index":3}`)},
		{"non-hex", obj(`{"path":"` + loadPath + `","sha256":"` + strings.Repeat("g", 64) + `","index":3}`)},
		{"other path", validEntry("/home/user/other.txt", sumA, 3)},
		{"two objects", append(validEntry(loadPath, sumA, 3), validEntry(loadPath, sumA, 4)...)},
		{"100 KiB of spaces", append(validEntry(loadPath, sumA, 3), bytes.Repeat([]byte(" "), 100<<10)...)},
	}
}

func TestResumeCorruptIgnored(t *testing.T) {
	type snapshot struct {
		mode os.FileMode
		data []byte
		link string
	}
	snap := func(t *testing.T, p string) snapshot {
		t.Helper()
		fi, err := os.Lstat(p)
		if err != nil {
			t.Fatalf("Lstat(%s): %v", p, err)
		}
		sn := snapshot{mode: fi.Mode()}
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			sn.link, _ = os.Readlink(p)
		case fi.Mode().IsRegular() && fi.Mode().Perm()&0o400 != 0:
			sn.data, _ = os.ReadFile(p)
		}
		return sn
	}

	type row struct {
		name  string
		setup func(t *testing.T, s *Store, dst string)
		// saveFails: Save cannot replace this entry (rename onto a directory fails).
		saveFails bool
	}
	var rows []row
	for _, c := range corruptRows() {
		c := c
		rows = append(rows, row{name: c.name, setup: func(t *testing.T, s *Store, dst string) {
			if err := os.WriteFile(dst, c.data, 0o600); err != nil {
				t.Fatal(err)
			}
		}})
	}
	rows = append(rows,
		row{name: "directory", saveFails: true, setup: func(t *testing.T, s *Store, dst string) {
			if err := os.Mkdir(dst, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		row{name: "symlink to valid entry", setup: func(t *testing.T, s *Store, dst string) {
			target := filepath.Join(t.TempDir(), "elsewhere.json")
			if err := os.WriteFile(target, validEntry(loadPath, sumA, 3), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, dst); err != nil {
				t.Fatal(err)
			}
		}},
		row{name: "fifo", setup: func(t *testing.T, s *Store, dst string) {
			if err := syscall.Mkfifo(dst, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		row{name: "mode 000", setup: func(t *testing.T, s *Store, dst string) {
			if os.Geteuid() == 0 {
				t.Skip("root ignores file modes")
			}
			if err := os.WriteFile(dst, validEntry(loadPath, sumA, 3), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dst, 0); err != nil {
				t.Fatal(err)
			}
		}},
	)

	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "fastread")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			s := NewStore(dir)
			dst := s.EntryPath(loadPath)
			tc.setup(t, s, dst)
			before := snap(t, dst)

			type result struct {
				idx int
				ok  bool
				err error
			}
			done := make(chan result, 1)
			go func() {
				i, ok, err := s.Load(loadPath, sumA)
				done <- result{i, ok, err}
			}()
			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()
			var r result
			select {
			case r = <-done:
			case <-timer.C:
				t.Fatal("Load did not return within 5 s")
			}
			if r.ok || r.idx != 0 || !errors.Is(r.err, ErrCorruptState) {
				t.Fatalf("Load = (%d, %v, %v), want (0, false, ErrCorruptState)", r.idx, r.ok, r.err)
			}
			after := snap(t, dst)
			if after.mode != before.mode || after.link != before.link || !bytes.Equal(after.data, before.data) {
				t.Fatalf("entry changed by Load: before %+v, after %+v", before.mode, after.mode)
			}

			err := s.Save(loadPath, sumA, 7)
			if tc.saveFails {
				if err == nil {
					t.Fatal("Save over a directory entry unexpectedly succeeded")
				}
				return
			}
			if err != nil {
				t.Fatalf("Save over corrupt entry: %v", err)
			}
			if i, ok, err := s.Load(loadPath, sumA); i != 7 || !ok || err != nil {
				t.Fatalf("Load after Save = (%d, %v, %v), want (7, true, nil)", i, ok, err)
			}
		})
	}
}

func TestStartIndex(t *testing.T) {
	cases := []struct {
		name               string
		start              int
		startSet, noResume bool
		saved              int
		haveSaved          bool
		n, want            int
	}{
		{"start wins over saved", 3, true, false, 7, true, 10, 3},
		{"start 0 wins over saved 5", 0, true, false, 5, true, 10, 0},
		{"noResume ignores saved", 0, false, true, 7, true, 10, 0},
		{"saved 7 of 10", 0, false, false, 7, true, 10, 7},
		{"saved 10 of 10 is stale", 0, false, false, 10, true, 10, 0},
		{"saved negative", 0, false, false, -1, true, 10, 0},
		{"no saved entry", 0, false, false, 7, false, 10, 0},
		{"start 99 clamps to 9", 99, true, false, 0, false, 10, 9},
		{"start -3 clamps to 0", -3, true, false, 0, false, 10, 0},
		{"n 1 with start", 5, true, false, 0, false, 1, 0},
		{"n 1 with saved", 0, false, false, 3, true, 1, 0},
		{"n 1 with saved 0", 0, false, false, 0, true, 1, 0},
		{"n 1 no resume", 0, false, true, 0, true, 1, 0},
	}
	for _, tc := range cases {
		got := StartIndex(tc.start, tc.startSet, tc.noResume, tc.saved, tc.haveSaved, tc.n)
		if got != tc.want {
			t.Errorf("%s: StartIndex = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func FuzzDecodeEntry(f *testing.F) {
	f.Add(validEntry(loadPath, sumA, 3), loadPath)
	f.Add(validEntry(loadPath, sumB, 3), loadPath)
	for _, c := range corruptRows() {
		f.Add(c.data, loadPath)
	}
	f.Fuzz(func(t *testing.T, b []byte, path string) {
		idx, ok, err := decodeEntry(b, path, sumA)
		if err != nil {
			if !errors.Is(err, ErrCorruptState) {
				t.Fatalf("error not ErrCorruptState: %v", err)
			}
			if ok || idx != 0 {
				t.Fatalf("err with (%d, %v)", idx, ok)
			}
			return
		}
		if ok {
			if idx < 0 {
				t.Fatalf("ok with negative index %d", idx)
			}
			return
		}
		if idx != 0 {
			t.Fatalf("!ok with index %d", idx)
		}
		// !ok, nil err: must be a valid entry whose sha256 differs from sumA.
		var e struct {
			SHA256 string `json:"sha256"`
		}
		if json.Unmarshal(b, &e) != nil {
			t.Fatal("!ok with nil err for unparsable entry")
		}
		raw, herr := hex.DecodeString(e.SHA256)
		if herr != nil || len(raw) != 32 || e.SHA256 == hex.EncodeToString(sumA[:]) {
			t.Fatalf("!ok with nil err but sha256 %q is not a differing valid hash", e.SHA256)
		}
		var other [32]byte
		copy(other[:], raw)
		if _, ok2, err2 := decodeEntry(b, path, other); !ok2 || err2 != nil {
			t.Fatalf("entry not valid under its own hash: ok=%v err=%v", ok2, err2)
		}
	})
}
