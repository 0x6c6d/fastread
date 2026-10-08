package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	return names
}

func perm(t *testing.T, p string) os.FileMode {
	t.Helper()
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", p, err)
	}
	return fi.Mode().Perm()
}

func setUmask(t *testing.T, m int) {
	old := syscall.Umask(m)
	t.Cleanup(func() { syscall.Umask(old) })
}

func TestResumeDirFallback(t *testing.T) {
	tests := []struct {
		name, xdg, home, want string
		err                   error
	}{
		{"xdg", "/x/state", "", "/x/state/fastread", nil},
		{"xdg trailing slash", "/x/state/", "", "/x/state/fastread", nil},
		{"home fallback", "", "/home/u", "/home/u/.local/state/fastread", nil},
		{"relative xdg ignored", "rel/dir", "/home/u", "/home/u/.local/state/fastread", nil},
		{"nothing", "", "", "", ErrNoStateDir},
		{"both relative", "rel", "u", "", ErrNoStateDir},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Dir(envMap(map[string]string{"XDG_STATE_HOME": tc.xdg, "HOME": tc.home}))
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if got != tc.want {
				t.Fatalf("Dir = %q, want %q", got, tc.want)
			}
		})
	}
	tmp := t.TempDir()
	d, err := Dir(envMap(map[string]string{"XDG_STATE_HOME": tmp, "HOME": tmp}))
	if err != nil || d != filepath.Join(tmp, "fastread") {
		t.Fatalf("Dir = %q, %v", d, err)
	}
	if names := listDir(t, tmp); len(names) != 0 {
		t.Fatalf("Dir created files: %v", names)
	}
}

func TestResumeModes(t *testing.T) {
	setUmask(t, 0)
	src := "/books/a.txt"
	var sum [32]byte

	t.Run("fresh dir with missing parents", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "a", "b", "fastread")
		s := NewStore(dir)
		if err := s.Save(src, sum, 1); err != nil {
			t.Fatal(err)
		}
		if p := perm(t, dir); p != 0o700 {
			t.Errorf("dir mode %o, want 700", p)
		}
		if p := perm(t, s.EntryPath(src)); p != 0o600 {
			t.Errorf("entry mode %o, want 600", p)
		}
	})
	t.Run("existing 0755 dir and 0644 entry", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "fastread")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		s := NewStore(dir)
		if err := os.WriteFile(s.EntryPath(src), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := s.Save(src, sum, 2); err != nil {
			t.Fatal(err)
		}
		if p := perm(t, dir); p != 0o700 {
			t.Errorf("dir mode %o, want 700", p)
		}
		if p := perm(t, s.EntryPath(src)); p != 0o600 {
			t.Errorf("entry mode %o, want 600", p)
		}
	})
	t.Run("dir is a regular file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "fastread")
		content := []byte("not a dir")
		if err := os.WriteFile(dir, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := NewStore(dir).Save(src, sum, 0); err == nil {
			t.Fatal("Save succeeded, want error")
		}
		got, err := os.ReadFile(dir)
		if err != nil || !bytes.Equal(got, content) {
			t.Fatalf("file changed: %q, %v", got, err)
		}
		if p := perm(t, dir); p != 0o644 {
			t.Errorf("file mode %o, want 644", p)
		}
	})
	t.Run("dir is a symlink to a directory", func(t *testing.T) {
		base := t.TempDir()
		target := filepath.Join(base, "real")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(base, "fastread")
		if err := os.Symlink(target, dir); err != nil {
			t.Fatal(err)
		}
		if err := NewStore(dir).Save(src, sum, 0); err == nil {
			t.Fatal("Save succeeded, want error")
		}
		if names := listDir(t, target); len(names) != 0 {
			t.Fatalf("target dir written: %v", names)
		}
	})
	bad := []struct {
		name  string
		path  string
		index int
	}{
		{"negative index", "/books/a.txt", -1},
		{"relative path", "books/a.txt", 0},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "fastread")
			if err := NewStore(dir).Save(tc.path, sum, tc.index); err == nil {
				t.Fatal("Save succeeded, want error")
			}
			if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("dir created or stat error: %v", err)
			}
		})
	}
}

func TestResumeAtomic(t *testing.T) {
	src := "/books/p.txt"
	var sum [32]byte
	sum[0] = 0xab

	failing := []struct {
		name  string
		setup func()
	}{
		{"rename fails", func() {
			renameFile = func(string, string) error { return errors.New("rename boom") }
		}},
		{"write half then fail", func() {
			writeData = func(f *os.File, b []byte) error {
				if _, err := f.Write(b[:len(b)/2]); err != nil {
					return err
				}
				return errors.New("write boom")
			}
		}},
	}
	for _, tc := range failing {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStore(filepath.Join(t.TempDir(), "fastread"))
			if err := s.Save(src, sum, 3); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(s.EntryPath(src))
			if err != nil {
				t.Fatal(err)
			}
			oldW, oldR := writeData, renameFile
			t.Cleanup(func() { writeData, renameFile = oldW, oldR })
			tc.setup()
			if err := s.Save(src, sum, 5); err == nil {
				t.Fatal("Save succeeded, want error")
			}
			after, err := os.ReadFile(s.EntryPath(src))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("entry changed: %q -> %q (%v)", before, after, err)
			}
			names := listDir(t, s.Dir())
			if len(names) != 1 || names[0] != filepath.Base(s.EntryPath(src)) {
				t.Fatalf("dir contents %v, want only the entry", names)
			}
		})
	}

	t.Run("concurrent saves", func(t *testing.T) {
		s := NewStore(filepath.Join(t.TempDir(), "fastread"))
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs <- s.Save(src, sum, i)
			}(i)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		b, err := os.ReadFile(s.EntryPath(src))
		if err != nil {
			t.Fatal(err)
		}
		var e entry
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatalf("entry not JSON: %q: %v", b, err)
		}
		if e.Index < 0 || e.Index > 7 || e.Path != src {
			t.Fatalf("entry = %+v", e)
		}
		if names := listDir(t, s.Dir()); len(names) != 1 {
			t.Fatalf("dir contents %v, want only the entry", names)
		}
	})

	t.Run("entry is a symlink", func(t *testing.T) {
		base := t.TempDir()
		outside := filepath.Join(base, "outside.txt")
		content := []byte("secret outside")
		if err := os.WriteFile(outside, content, 0o644); err != nil {
			t.Fatal(err)
		}
		s := NewStore(filepath.Join(base, "fastread"))
		if err := os.Mkdir(s.Dir(), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, s.EntryPath(src)); err != nil {
			t.Fatal(err)
		}
		if err := s.Save(src, sum, 4); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(outside)
		if err != nil || !bytes.Equal(got, content) {
			t.Fatalf("outside file changed: %q, %v", got, err)
		}
		fi, err := os.Lstat(s.EntryPath(src))
		if err != nil {
			t.Fatal(err)
		}
		if !fi.Mode().IsRegular() || fi.Mode().Perm() != 0o600 {
			t.Fatalf("entry mode %v, want regular 0600", fi.Mode())
		}
	})
}

func TestResumeDeleteAtEnd(t *testing.T) {
	var sum [32]byte
	for i := range sum {
		sum[i] = byte(i)
	}
	a, b := "/books/a.txt", "/books/b.txt"

	t.Run("save then delete twice", func(t *testing.T) {
		s := NewStore(filepath.Join(t.TempDir(), "fastread"))
		if err := s.Save(a, sum, 7); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err := s.Delete(a); err != nil {
				t.Fatalf("Delete #%d: %v", i+1, err)
			}
			if _, err := os.Lstat(s.EntryPath(a)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("entry still present: %v", err)
			}
			if p := perm(t, s.Dir()); p != 0o700 {
				t.Fatalf("dir mode %o, want 700", p)
			}
			if names := listDir(t, s.Dir()); len(names) != 0 {
				t.Fatalf("dir not empty: %v", names)
			}
		}
	})
	t.Run("delete other path keeps entry", func(t *testing.T) {
		s := NewStore(filepath.Join(t.TempDir(), "fastread"))
		if err := s.Save(a, sum, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(b); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(s.EntryPath(a)); err != nil {
			t.Fatalf("entry for a gone: %v", err)
		}
	})
	t.Run("two paths, exact keys", func(t *testing.T) {
		s := NewStore(filepath.Join(t.TempDir(), "fastread"))
		cases := []struct {
			path  string
			index int
		}{{a, 10}, {b, 20}}
		for _, c := range cases {
			if err := s.Save(c.path, sum, c.index); err != nil {
				t.Fatal(err)
			}
		}
		if s.EntryPath(a) == s.EntryPath(b) {
			t.Fatal("entry paths collide")
		}
		if names := listDir(t, s.Dir()); len(names) != 2 {
			t.Fatalf("dir contents %v, want 2 entries", names)
		}
		wantSum := fmt.Sprintf("%x", sum)
		for _, c := range cases {
			raw, err := os.ReadFile(s.EntryPath(c.path))
			if err != nil {
				t.Fatal(err)
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			if len(m) != 3 {
				t.Fatalf("keys %v, want exactly path, sha256, index", m)
			}
			if m["path"] != c.path || m["sha256"] != wantSum || m["index"] != float64(c.index) {
				t.Fatalf("entry %v", m)
			}
		}
	})
}
