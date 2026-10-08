//go:build !nogui && cgo

package gui

import (
	"context"
	"errors"
	"testing"

	"github.com/0x6c6d/fastread/internal/state"
)

func TestWindowNoDisplay(t *testing.T) {
	called := false
	opts := Options{Getenv: func(string) string { return "" }}
	err := Run(context.Background(), &state.Player{}, opts, func(int, error) { called = true })
	if !errors.Is(err, ErrNoDisplay) {
		t.Fatalf("err = %v, want ErrNoDisplay", err)
	}
	if called {
		t.Fatal("finish was called")
	}
}

func TestWordSize(t *testing.T) {
	want := map[int]float32{0: 20, 1: 20, 2: 32, 3: 48, 4: 64, 5: 96, 9: 96}
	for lvl, sp := range want {
		if got := float32(wordSize(lvl)); got != sp {
			t.Errorf("wordSize(%d) = %v, want %v", lvl, got, sp)
		}
	}
}

func TestNewShaper(t *testing.T) {
	if _, err := newShaper(); err != nil {
		t.Fatal(err)
	}
}
