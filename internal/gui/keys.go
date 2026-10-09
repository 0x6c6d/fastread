//go:build !nogui && cgo

package gui

import (
	"gioui.org/io/event"
	"gioui.org/io/key"

	"github.com/0x6c6d/fastread/internal/state"
)

// keyRow is one accepted key: filter fields plus the action it triggers.
type keyRow struct {
	name     key.Name
	required key.Modifiers
	optional key.Modifiers
	action   state.Action
}

// keyTable is the single source of truth for keyFilters and keyAction.
var keyTable = []keyRow{
	{name: key.NameSpace, action: state.ActTogglePause},
	{name: key.NameUpArrow, action: state.ActWPMUp},
	{name: key.NameDownArrow, action: state.ActWPMDown},
	{name: "[", action: state.ActSizeDown},
	{name: "]", action: state.ActSizeUp},
	{name: key.NameLeftArrow, action: state.ActPrev},
	{name: key.NameRightArrow, action: state.ActNext},
	{name: key.NameHome, action: state.ActHome},
	{name: "P", action: state.ActToggleProgress},
	{name: "?", optional: key.ModShift, action: state.ActToggleHelp},
	{name: "Q", action: state.ActQuit},
	{name: key.NameEscape, action: state.ActQuit},
	{name: "C", required: key.ModCtrl, action: state.ActQuit},
}

// keyFilters returns the filters the window passes to gtx.Event each frame: exactly one
// key.Filter (Focus nil) per row of the table.
func keyFilters() []event.Filter {
	fs := make([]event.Filter, 0, len(keyTable))
	for _, r := range keyTable {
		fs = append(fs, key.Filter{Name: r.name, Required: r.required, Optional: r.optional})
	}
	return fs
}

// keyAction maps a key event to a player action (R26). ok is false for Release events and
// for every event no filter row matches.
func keyAction(e key.Event) (a state.Action, ok bool) {
	if e.State != key.Press {
		return state.ActNone, false
	}
	for _, r := range keyTable {
		if e.Name == r.name && e.Modifiers&r.required == r.required &&
			e.Modifiers&^(r.required|r.optional) == 0 {
			return r.action, true
		}
	}
	return state.ActNone, false
}
