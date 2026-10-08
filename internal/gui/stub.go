//go:build nogui || !cgo

package gui

import (
	"context"

	"github.com/0x6c6d/fastread/internal/state"
)

// Run is unavailable in this build; it returns ErrUnavailable and never calls finish.
func Run(ctx context.Context, p *state.Player, opts Options, finish func(last int, err error)) error {
	return ErrUnavailable
}
