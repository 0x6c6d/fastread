//go:build !nogui && cgo

package gui

import (
	"testing"

	"gioui.org/io/key"

	"github.com/0x6c6d/fastread/internal/state"
)

func filterMatches(f key.Filter, e key.Event) bool {
	return f.Name == e.Name && e.Modifiers&f.Required == f.Required &&
		e.Modifiers&^(f.Required|f.Optional) == 0
}

func TestGUIKeyMap(t *testing.T) {
	p := func(n key.Name, m key.Modifiers) key.Event {
		return key.Event{Name: n, Modifiers: m, State: key.Press}
	}
	rel := func(n key.Name, m key.Modifiers) key.Event {
		return key.Event{Name: n, Modifiers: m, State: key.Release}
	}
	type tc struct {
		name string
		ev   key.Event
		act  state.Action
		ok   bool
	}
	cases := []tc{
		{"space", p(key.NameSpace, 0), state.ActTogglePause, true},
		{"up", p(key.NameUpArrow, 0), state.ActWPMUp, true},
		{"down", p(key.NameDownArrow, 0), state.ActWPMDown, true},
		{"[", p("[", 0), state.ActSizeDown, true},
		{"]", p("]", 0), state.ActSizeUp, true},
		{"left", p(key.NameLeftArrow, 0), state.ActPrev, true},
		{"right", p(key.NameRightArrow, 0), state.ActNext, true},
		{"home", p(key.NameHome, 0), state.ActHome, true},
		{"P", p("P", 0), state.ActToggleProgress, true},
		{"? shift", p("?", key.ModShift), state.ActToggleHelp, true},
		{"? bare", p("?", 0), state.ActToggleHelp, true},
		{"Q", p("Q", 0), state.ActQuit, true},
		{"esc", p(key.NameEscape, 0), state.ActQuit, true},
		{"ctrl+C", p("C", key.ModCtrl), state.ActQuit, true},
		{"Q shift", p("Q", key.ModShift), 0, false},
		{"P shift", p("P", key.ModShift), 0, false},
		{"C bare", p("C", 0), 0, false},
		{"C ctrl+shift", p("C", key.ModCtrl|key.ModShift), 0, false},
		{"X", p("X", 0), 0, false},
		{"PageUp", p(key.NamePageUp, 0), 0, false},
		{"F1", p("F1", 0), 0, false},
		{"Shift", p("Shift", 0), 0, false},
		{"Ctrl", p("Ctrl", 0), 0, false},
		{"empty", p("", 0), 0, false},
		{"lower q", p("q", 0), 0, false},
		{"space ctrl", p(key.NameSpace, key.ModCtrl), 0, false},
		{"up ctrl", p(key.NameUpArrow, key.ModCtrl), 0, false},
		{"left ctrl", p(key.NameLeftArrow, key.ModCtrl), 0, false},
	}
	for _, r := range keyTable {
		cases = append(cases, tc{"release " + string(r.name), rel(r.name, r.required), 0, false})
	}
	filters := keyFilters()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, ok := keyAction(c.ev)
			if ok != c.ok || (ok && a != c.act) {
				t.Fatalf("keyAction(%+v) = %v,%v; want %v,%v", c.ev, a, ok, c.act, c.ok)
			}
			matched := false
			for _, f := range filters {
				if filterMatches(f.(key.Filter), c.ev) {
					matched = true
				}
			}
			if c.ev.State == key.Press && matched != ok {
				t.Fatalf("filter match %v disagrees with ok %v", matched, ok)
			}
		})
	}
	if len(filters) != 13 || len(keyTable) != 13 {
		t.Fatalf("want 13 rows, got %d filters, %d rows", len(filters), len(keyTable))
	}
	seen := map[key.Filter]bool{}
	for _, f := range filters {
		kf, isKey := f.(key.Filter)
		if !isKey {
			t.Fatalf("filter %T is not key.Filter", f)
		}
		if kf.Focus != nil {
			t.Fatalf("filter %+v has non-nil Focus", kf)
		}
		if seen[kf] {
			t.Fatalf("duplicate filter %+v", kf)
		}
		seen[kf] = true
	}
	reach := map[state.Action]bool{}
	for _, r := range keyTable {
		reach[r.action] = true
	}
	for _, a := range []state.Action{state.ActTogglePause, state.ActWPMUp, state.ActWPMDown,
		state.ActSizeDown, state.ActSizeUp, state.ActPrev, state.ActNext, state.ActHome,
		state.ActToggleProgress, state.ActToggleHelp, state.ActQuit} {
		if !reach[a] {
			t.Errorf("action %v unreachable", a)
		}
	}
}
