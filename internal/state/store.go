package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
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

// ErrCorruptState marks an entry that exists but cannot be used. Callers print at most one
// warning and continue as if there were no entry; it is never fatal.
var ErrCorruptState = errors.New("unreadable or corrupt resume state")

// maxEntrySize bounds how many bytes of an entry are ever read.
const maxEntrySize = 64 << 10

// Load returns the saved index for path. No entry → (0, false, nil). Stored sha256 differs
// from sum (the file changed) → (0, false, nil). Unusable entry → (0, false, err) with
// errors.Is(err, ErrCorruptState). Never panics, never modifies or deletes anything.
func (s *Store) Load(path string, sum [32]byte) (index int, ok bool, err error) {
	dst := s.EntryPath(path)
	corrupt := func(reason string) (int, bool, error) {
		return 0, false, fmt.Errorf("load %s: %w: %s", dst, ErrCorruptState, reason)
	}
	fi, err := os.Lstat(dst)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, false, nil
		}
		return corrupt("cannot inspect entry")
	}
	if !fi.Mode().IsRegular() {
		return corrupt("entry is not a regular file")
	}
	// O_NOFOLLOW and O_NONBLOCK guard against the entry being swapped for a symlink or a FIFO
	// between Lstat and open; the descriptor is re-checked below.
	f, err := os.OpenFile(dst, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return corrupt("cannot open entry")
	}
	defer f.Close()
	if fi, err = f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return corrupt("entry is not a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxEntrySize+1))
	if err != nil {
		return corrupt("cannot read entry")
	}
	if len(b) > maxEntrySize {
		return corrupt("entry too large")
	}
	index, ok, err = decodeEntry(b, path, sum)
	if err != nil {
		return 0, false, fmt.Errorf("load %s: %w", dst, err)
	}
	return index, ok, nil
}

type rawEntry struct {
	Path   *string `json:"path"`
	SHA256 *string `json:"sha256"`
	Index  *int    `json:"index"`
}

// decodeEntry parses entry bytes for path and sum with the same results as Load.
func decodeEntry(b []byte, path string, sum [32]byte) (index int, ok bool, err error) {
	corrupt := func(reason string) (int, bool, error) {
		return 0, false, fmt.Errorf("%w: %s", ErrCorruptState, reason)
	}
	if len(b) > maxEntrySize {
		return corrupt("entry too large")
	}
	trimmed := bytes.TrimLeft(b, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return corrupt("entry is not a JSON object")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var e rawEntry
	if err := dec.Decode(&e); err != nil {
		return corrupt("malformed JSON entry")
	}
	if rest := bytes.TrimLeft(b[dec.InputOffset():], " \t\r\n"); len(rest) != 0 {
		return corrupt("trailing data after entry")
	}
	if e.Path == nil || e.SHA256 == nil || e.Index == nil {
		return corrupt("missing field")
	}
	if *e.Path != path {
		return corrupt("entry belongs to another path")
	}
	if !isLowerHex64(*e.SHA256) {
		return corrupt("sha256 is not 64 lowercase hex characters")
	}
	if *e.Index < 0 {
		return corrupt("negative index")
	}
	if *e.SHA256 != hex.EncodeToString(sum[:]) {
		return 0, false, nil
	}
	return *e.Index, true, nil
}

func isLowerHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// StartIndex returns the first word for a text of n >= 1 words: startSet → start (the caller
// already rejected start >= n as a usage error; clamp into [0, n-1] defensively); else
// noResume or !haveSaved → 0; else saved when 0 <= saved < n, otherwise 0 (stale entry for
// a text that is now shorter).
func StartIndex(start int, startSet, noResume bool, saved int, haveSaved bool, n int) int {
	if startSet {
		if start >= n {
			start = n - 1
		}
		if start < 0 {
			start = 0
		}
		return start
	}
	if noResume || !haveSaved {
		return 0
	}
	if saved >= 0 && saved < n {
		return saved
	}
	return 0
}
