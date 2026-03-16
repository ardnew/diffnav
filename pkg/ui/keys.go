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
		key.WithHelp("l", "expand"),
	),
	CollapseNode: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h", "collapse"),
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
		key.WithHelp("ctrl+f", "page down"),
	),
	CtrlB: key.NewBinding(
		key.WithKeys("ctrl+b", "pgup"),
		key.WithHelp("ctrl+b", "page up"),
	),
	ScrollTop: key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g", "scroll to top"),
	),
	ScrollBottom: key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G", "scroll to bottom"),
	),
	ToggleFileTree: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "toggle file tree"),
	),
	Search: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "search files"),
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
		key.WithHelp("o", "open"),
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

// aliasBindings maps bindings that have alias keys to copies with both
// primary and alias key names shown in the help text.
var aliasBindings = map[*key.Binding]key.Binding{
	&keys.ExpandNode: key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("l/→", "expand"),
	),
	&keys.CollapseNode: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h/←", "collapse"),
	),
	&keys.CtrlF: key.NewBinding(
		key.WithKeys("ctrl+f", "pgdown"),
		key.WithHelp("ctrl+f/PgDn", "page down"),
	),
	&keys.CtrlB: key.NewBinding(
		key.WithKeys("ctrl+b", "pgup"),
		key.WithHelp("ctrl+b/PgUp", "page up"),
	),
	&keys.ScrollTop: key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g/Home", "scroll to top"),
	),
	&keys.ScrollBottom: key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G/End", "scroll to bottom"),
	),
}

func keyGroupsWith(aliases bool) [][]key.Binding {
	resolve := func(b *key.Binding) key.Binding {
		if aliases {
			if alias, ok := aliasBindings[b]; ok {
				return alias
			}
		}
		return *b
	}

	return [][]key.Binding{{
		resolve(&keys.SwitchPanel),
		resolve(&keys.Up),
		resolve(&keys.Down),
		resolve(&keys.NextFile),
		resolve(&keys.PrevFile),
		resolve(&keys.ExpandNode),
		resolve(&keys.CollapseNode),
		resolve(&keys.ToggleNode),
	}, {
		resolve(&keys.CtrlD),
		resolve(&keys.CtrlU),
		resolve(&keys.CtrlF),
		resolve(&keys.CtrlB),
		resolve(&keys.ScrollTop),
		resolve(&keys.ScrollBottom),
	}, {
		resolve(&keys.ToggleFileTree),
		resolve(&keys.Search),
		resolve(&keys.Copy),
		resolve(&keys.OpenInEditor),
		resolve(&keys.ToggleDiffView),
		resolve(&keys.ToggleIconStyle),
	}, {
		resolve(&keys.ToggleHelp),
		resolve(&keys.Quit),
	}}
}

func KeyGroups() [][]key.Binding {
	return keyGroupsWith(false)
}

func KeyGroupsAll() [][]key.Binding {
	return keyGroupsWith(true)
}
