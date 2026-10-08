package main

import (
	"fmt"
	"io"

	"github.com/0x6c6d/fastread/internal/input"
	"github.com/0x6c6d/fastread/internal/state"
)

// resumer persists the position of a file source; a nil *resumer does nothing.
type resumer struct {
	store *state.Store
	path  string
	sum   [32]byte
}

// newResumer returns nil for raw text and stdin (doc.Path == "") without reading the
// environment or the filesystem. Otherwise it derives the state directory; when that fails
// it writes one warning line to stderr and returns nil. It creates nothing on disk.
func newResumer(doc input.Document, getenv func(string) string, stderr io.Writer) *resumer {
	if doc.Path == "" {
		return nil
	}
	dir, err := state.Dir(getenv)
	if err != nil {
		errLine(stderr, fmt.Errorf("warning: resume disabled: %w", err))
		return nil
	}
	return &resumer{store: state.NewStore(dir), path: doc.Path, sum: doc.SHA256}
}

// start returns the first word index for a text of n words. The saved position is loaded
// only when r != nil and neither --start nor --no-resume was given; a load error writes one
// warning line and counts as nothing saved.
func (r *resumer) start(o options, n int, stderr io.Writer) int {
	saved, have := 0, false
	if r != nil && !o.startSet && !o.noResume {
		i, ok, err := r.store.Load(r.path, r.sum)
		if err != nil {
			errLine(stderr, fmt.Errorf("warning: ignoring saved position: %w", err))
		} else {
			saved, have = i, ok
		}
	}
	return state.StartIndex(o.start, o.startSet, o.noResume, saved, have, n)
}

// finish deletes the entry when the text was read to the end and saves last otherwise.
func (r *resumer) finish(p *state.Player, last int) error {
	if r == nil {
		return nil
	}
	var err error
	if p.Finished() {
		err = r.store.Delete(r.path)
	} else {
		err = r.store.Save(r.path, r.sum, last)
	}
	if err != nil {
		return fmt.Errorf("saving reading position: %w", err)
	}
	return nil
}
