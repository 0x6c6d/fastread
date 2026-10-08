//go:build nogui || !cgo

package gui

import (
	"context"
	"errors"
	"testing"

	"github.com/0x6c6d/fastread/internal/state"
)

func TestStubUnavailable(t *testing.T) {
	called := false
	opts := Options{Getenv: func(k string) string {
		if k == "DISPLAY" {
			return ":0"
		}
		return ""
	}}
	err := Run(context.Background(), &state.Player{}, opts, func(int, error) { called = true })
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if called {
		t.Fatal("finish was called")
	}
}
