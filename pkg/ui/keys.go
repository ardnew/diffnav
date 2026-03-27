package ui

import "charm.land/bubbles/v2/key"

type KeyMap struct {
	ExpandNode      key.Binding
	CollapseNode    key.Binding
	ToggleNode      key.Binding
	Up              key.Binding
	Down            key.Binding
	NextFile        key.Binding
	PrevFile        key.Binding
	CtrlD           key.Binding
	CtrlU           key.Binding
	CtrlF           key.Binding
	CtrlB           key.Binding
	ScrollTop       key.Binding
	ScrollBottom    key.Binding
	ToggleFileTree  key.Binding
	Search          key.Binding
	Quit            key.Binding
	Copy            key.Binding
	SwitchPanel     key.Binding
	OpenInEditor    key.Binding
	ToggleDiffView  key.Binding
	ToggleIconStyle key.Binding
	ToggleHelp      key.Binding
}

var keys = &KeyMap{
	ExpandNode: key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("→/l", "expand"),
	),
	CollapseNode: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("←/h", "collapse"),
	),
	ToggleNode: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "toggle"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "prev node"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "next node"),
	),
	NextFile: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "next file"),
	),
	PrevFile: key.NewBinding(
		key.WithKeys("p", "N"),
		key.WithHelp("p/N", "prev file"),
	),
	CtrlD: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "½ page down"),
	),
	CtrlU: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "½ page up"),
	),
	CtrlF: key.NewBinding(
		key.WithKeys("ctrl+f", "pgdown"),
		key.WithHelp("ctrl+f/PgDn", "page down"),
	),
	CtrlB: key.NewBinding(
		key.WithKeys("ctrl+b", "pgup"),
		key.WithHelp("ctrl+b/PgUp", "page up"),
	),
	ScrollTop: key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g/Home", "scroll to top"),
	),
	ScrollBottom: key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G/End", "scroll to bottom"),
	),
	ToggleFileTree: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "toggle file tree"),
	),
	Search: key.NewBinding(
		key.WithKeys("f3"),
		key.WithHelp("F3", "filter files"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Copy: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "copy file path"),
	),
	SwitchPanel: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
	OpenInEditor: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "edit file"),
	),
	ToggleDiffView: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "toggle side-by-side"),
	),
	ToggleIconStyle: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "toggle icon style"),
	),
	ToggleHelp: key.NewBinding(
		key.WithKeys("?", "f1"),
		key.WithHelp("F1/?", "toggle help"),
	),
}

func keyGroups() [][]key.Binding {
	spacer := key.NewBinding(key.WithKeys(""), key.WithHelp(" ", " "))

	return [][]key.Binding{{
		keys.SwitchPanel,
		keys.Up,
		keys.Down,
		keys.NextFile,
		keys.PrevFile,
		keys.ExpandNode,
		keys.CollapseNode,
		keys.ToggleNode,
		spacer,
		keys.CtrlD,
		keys.CtrlU,
		keys.CtrlF,
		keys.CtrlB,
		keys.ScrollTop,
		keys.ScrollBottom,
	}, {
		keys.ToggleFileTree,
		keys.Search,
		keys.Copy,
		keys.OpenInEditor,
		keys.ToggleDiffView,
		keys.ToggleIconStyle,
		spacer,
		keys.ToggleHelp,
		keys.Quit,
	}}
}

func KeyGroups() [][]key.Binding {
	return keyGroups()
}
