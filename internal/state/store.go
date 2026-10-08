package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoStateDir reports that no resume directory can be derived from the environment.
var ErrNoStateDir = errors.New("no state directory: XDG_STATE_HOME is not absolute and HOME is not set")

// Seams swapped by tests; production code never changes them.
var (
	writeData  = func(f *os.File, b []byte) error { _, err := f.Write(b); return err }
	renameFile = os.Rename
)

// Dir returns the resume directory without creating anything: filepath.Join(XDG_STATE_HOME,
// "fastread") when XDG_STATE_HOME is an absolute path; else filepath.Join(HOME, ".local",
// "state", "fastread") when HOME is an absolute path; else ErrNoStateDir. A relative
// XDG_STATE_HOME is ignored (XDG rule).
func Dir(getenv func(string) string) (string, error) {
	if x := getenv("XDG_STATE_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "fastread"), nil
	}
	if h := getenv("HOME"); h != "" && filepath.IsAbs(h) {
		return filepath.Join(h, ".local", "state", "fastread"), nil
	}
	return "", ErrNoStateDir
}

// Store keeps one JSON entry per source file in its directory.
type Store struct{ dir string }

// NewStore returns a store rooted at dir; nothing is created until Save.
func NewStore(dir string) *Store { return &Store{dir: dir} }

// Dir returns the store directory.
func (s *Store) Dir() string { return s.dir }

// EntryPath is filepath.Join(dir, hex(sha256([]byte(path))) + ".json") (lowercase hex).
func (s *Store) EntryPath(path string) string {
	h := sha256.Sum256([]byte(path))
	return filepath.Join(s.dir, hex.EncodeToString(h[:])+".json")
}

type entry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Index  int    `json:"index"`
}

// Save atomically writes {"path":path,"sha256":hex(sum),"index":index} for path.
func (s *Store) Save(path string, sum [32]byte, index int) error {
	dst := s.EntryPath(path)
	if index < 0 {
		return fmt.Errorf("save %s: negative index %d", dst, index)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("save %s: source path is not absolute", dst)
	}
	if err := s.prepareDir(); err != nil {
		return fmt.Errorf("save %s: %w", dst, err)
	}
	b, err := json.Marshal(entry{Path: path, SHA256: hex.EncodeToString(sum[:]), Index: index})
	if err != nil {
		return fmt.Errorf("save %s: %w", dst, err)
	}
	if err := writeAtomic(s.dir, dst, b); err != nil {
		return fmt.Errorf("save %s: %w", dst, err)
	}
	if d, err := os.Open(s.dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func (s *Store) prepareDir() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	fi, err := os.Lstat(s.dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return errors.New("state directory is not a directory")
	}
	if fi.Mode().Perm() != 0o700 {
		if err := os.Chmod(s.dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func writeAtomic(dir, dst string, b []byte) (err error) {
	f, err := os.CreateTemp(dir, ".entry-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	closed := false
	defer func() {
		if err != nil {
			if !closed {
				_ = f.Close()
			}
			_ = os.Remove(tmp)
		}
	}()
	if err = f.Chmod(0o600); err != nil {
		return err
	}
	if err = writeData(f, b); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	closed = true
	if err = f.Close(); err != nil {
		return err
	}
	return renameFile(tmp, dst)
}

// Delete removes the entry for path; a missing entry is not an error.
func (s *Store) Delete(path string) error {
	dst := s.EntryPath(path)
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete %s: %w", dst, err)
	}
	return nil
}
