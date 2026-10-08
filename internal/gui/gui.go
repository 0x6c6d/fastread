// Package gui is the Gio window UI; it is the only package that imports the Gio module.
package gui

import (
	"errors"
	"os"
)

// Options configures Run.
type Options struct {
	Getenv func(string) string // nil -> os.Getenv; used for DISPLAY / WAYLAND_DISPLAY
}

// Errors returned synchronously by Run.
var (
	ErrUnavailable = errors.New("GUI not available in this build (built with nogui or without cgo)")
	ErrNoDisplay   = errors.New("no display: neither DISPLAY nor WAYLAND_DISPLAY is set")
)

// getenv returns opts.Getenv, or os.Getenv when it is nil.
func (o Options) getenv() func(string) string {
	if o.Getenv == nil {
		return os.Getenv
	}
	return o.Getenv
}

// hasDisplay reports whether DISPLAY or WAYLAND_DISPLAY is non-empty.
func hasDisplay(getenv func(string) string) bool {
	return getenv("DISPLAY") != "" || getenv("WAYLAND_DISPLAY") != ""
}
