package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines all key bindings for the TUI
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Toggle    key.Binding
	Enter     key.Binding
	Tab       key.Binding
	PrevType  key.Binding
	NextType  key.Binding
	Top       key.Binding
	Bottom    key.Binding
	HalfUp   key.Binding
	HalfDown key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	ExpandAll    key.Binding
	CollapseAll  key.Binding
	ToggleClosed key.Binding
	Refresh      key.Binding
	Quit         key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h", "collapse"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l", "expand"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "detail"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "pane"),
		),
		PrevType: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "prev type"),
		),
		NextType: key.NewBinding(
			key.WithKeys("]", "/"),
			key.WithHelp("]/", "next type"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		HalfUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("C-u", "half up"),
		),
		HalfDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("C-d", "half down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("shift+up", "pgup"),
			key.WithHelp("S-up", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("shift+down", "pgdown"),
			key.WithHelp("S-down", "page down"),
		),
		ExpandAll: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "expand all"),
		),
		CollapseAll: key.NewBinding(
			key.WithKeys("O"),
			key.WithHelp("O", "collapse all"),
		),
		ToggleClosed: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "closed"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}
