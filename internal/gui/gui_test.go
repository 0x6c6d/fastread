package gui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/0x6c6d/fastread/internal/state"
)

func TestRunNoDisplay(t *testing.T) {
	called := false
	opts := Options{Getenv: func(string) string { return "" }}
	err := Run(context.Background(), &state.Player{}, opts, func(int, error) { called = true })
	if err == nil {
		t.Fatal("Run returned nil error without a display")
	}
	if !errors.Is(err, ErrNoDisplay) && !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrNoDisplay or ErrUnavailable", err)
	}
	if strings.Contains(err.Error(), "\n") {
		t.Fatalf("error message contains a newline: %q", err.Error())
	}
	if called {
		t.Fatal("finish was called")
	}
}

func TestHasDisplay(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	if hasDisplay(env(nil)) {
		t.Error("empty env reported a display")
	}
	if !hasDisplay(env(map[string]string{"DISPLAY": ":0"})) {
		t.Error("DISPLAY not detected")
	}
	if !hasDisplay(env(map[string]string{"WAYLAND_DISPLAY": "wayland-0"})) {
		t.Error("WAYLAND_DISPLAY not detected")
	}
}
