package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/tree"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/charmbracelet/log"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/diffnav/pkg/config"
	"github.com/dlvhdr/diffnav/pkg/dirnode"
	"github.com/dlvhdr/diffnav/pkg/filenode"
	"github.com/dlvhdr/diffnav/pkg/ui/common"
	"github.com/dlvhdr/diffnav/pkg/ui/panes/diffviewer"
	"github.com/dlvhdr/diffnav/pkg/ui/panes/filetree"
	"github.com/dlvhdr/diffnav/pkg/ui/panes/help"
	"github.com/dlvhdr/diffnav/pkg/utils"
)

const (
	minResizeStep = 6
	footerHeight  = 1
	headerHeight  = 2
	filterHeight  = 3

	// Zone IDs for bubblezone click detection.
	zoneFilterBox     = "filterbox"
	zoneFileTree      = "filetree"
	zoneFilterResults = "filterresults"
	zoneDiffViewer    = "diffviewer"

	// Sidebar resize detection threshold in pixels.
	sidebarGrabThreshold = 2

	// Sidebar width constraints.
	sidebarMinWidth  = 20
	sidebarHideWidth = 10

	// Scroll speed in lines per wheel tick.
	scrollLines = 3
)

type Panel int

const (
	FileTreePanel Panel = iota
	DiffViewerPanel
)

type mainModel struct {
	input             string
	files             []*gitdiff.File
	fileTree          filetree.Model
	diffViewer        diffviewer.Model
	width             int
	height            int
	isShowingFileTree bool
	activePanel       Panel
	filter            textinput.Model
	resultsVp         viewport.Model
	resultsCursor     int
	filtering         bool
	filtered          []string
	config            config.Config
	draggingSidebar   bool
	iconStyle         string
	sideBySide        bool
	help              help.Model
	helpOpen          bool
}

func New(input string, cfg config.Config) mainModel {
	m := mainModel{
		input: input, isShowingFileTree: cfg.UI.ShowFileTree,
		activePanel: FileTreePanel, config: cfg, iconStyle: cfg.UI.Icons, sideBySide: cfg.UI.SideBySide,
	}
	m.fileTree = filetree.New(cfg)
	m.fileTree.SetSize(cfg.UI.FileTreeWidth, 0)
	m.diffViewer = diffviewer.New(cfg.UI.SideBySide)
	m.help = help.New()
	m.help.SetKeys(KeyGroups())

	m.filter = textinput.New()
	m.filter.ShowSuggestions = true
	m.filter.KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("tab"))
	m.filter.Prompt = " "
	m.filter.Placeholder = fmt.Sprintf("(%s) Filter files", keys.Filter.Help().Key)
	m.filter.SetStyles(textinput.Styles{
		Focused: textinput.StyleState{
			Placeholder: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
			Prompt:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		},
	})
	m.filter.SetWidth(cfg.UI.FileTreeWidth - 2)

	m.resultsVp = viewport.Model{}

	return m
}

func (m mainModel) Init() tea.Cmd {
	return tea.Batch(m.fetchFileTree, m.diffViewer.Init())
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle mouse events regardless of filter mode
	if msg, ok := msg.(tea.MouseMsg); ok {
		return m.handleMouse(msg)
	}

	// Filter out key release events to prevent double-firing.
	// bubbletea/v2 generates both KeyPressMsg and KeyReleaseMsg as standard
	// events (not Kitty Keyboard Protocol); we only handle key presses.
	if _, ok := msg.(tea.KeyReleaseMsg); ok {
		return m, nil
	}

	// Help toggle is always available, even while filtering.
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, keys.ToggleHelp):
			m.helpOpen = !m.helpOpen
			if !m.helpOpen {
				m.closeHelp()
			}
			return m, nil
		case m.helpOpen && key.Matches(msg, keys.Escape):
			m.closeHelp()
			return m, nil
		case m.helpOpen:
			// Block all other keys while help is open
			return m, nil

		// Esc priority chain (help already handled above):
		// 2. Diff view active → activate explorer pane (when visible)
		// 3. Filter files mode → return to normal tree explorer
		// 4. Otherwise → quit
		case key.Matches(msg, keys.Escape):
			if m.activePanel == DiffViewerPanel {
				// Only switch focus to the file tree if it is currently visible.
				if m.isShowingFileTree {
					m.activePanel = FileTreePanel
					return m, nil
				}
				// If the file tree is hidden, do not switch focus to a non-visible pane.
			}
			if m.filtering {
				m.stopFilter()
				dfCmd := m.diffViewer.SetSize(m.width-m.sidebarWidth(), m.mainContentHeight())
				return m, dfCmd
			}
			return m, tea.Quit
		}
	}

	if m.filtering {
		var sCmds []tea.Cmd
		m, sCmds = m.filterUpdate(msg)
		cmds = append(cmds, sCmds...)
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Filter):
			m.filtering = true
			m.filter.SetWidth(m.filterWidth())
			m.filter.SetValue("")
			m.resultsCursor = 0
			m.setFilterResults()

			m.resultsVp.SetWidth(m.config.UI.FilterTreeWidth)
			m.resultsVp.SetHeight(m.mainContentHeight() - filterHeight)
			m.resultsVp.SetContent(m.resultsView())

			dfCmd := m.diffViewer.SetSize(m.width-m.sidebarWidth(), m.mainContentHeight())
			cmds = append(cmds, dfCmd, m.filter.Focus())
			return m, tea.Batch(cmds...)
		case key.Matches(msg, keys.ToggleFileTree):
			m.isShowingFileTree = !m.isShowingFileTree
			sidebarWidth := m.sidebarWidth()
			h := m.mainContentHeight()

			if !m.isShowingFileTree {
				m.activePanel = DiffViewerPanel
			} else {
				m.activePanel = FileTreePanel
			}
			treeWidth := sidebarWidth
			if sidebarWidth == 0 {
				treeWidth = m.config.UI.FileTreeWidth
			}

			m.fileTree.SetSize(treeWidth, h-filterHeight)
			m.filter.SetWidth(m.filterWidth())
			dfCmd := m.diffViewer.SetSize(m.width-sidebarWidth, h)
			cmds = append(cmds, dfCmd)
			return m, tea.Batch(cmds...)
		case key.Matches(msg, keys.ToggleIconStyle):
			m.cycleIconStyle()
			return m, nil
		case key.Matches(msg, keys.ToggleDiffView):
			m.sideBySide = !m.sideBySide
			cmd = m.diffViewer.SetSideBySide(m.sideBySide)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		case key.Matches(msg, keys.SwitchPanel):
			if m.isShowingFileTree {
				if m.activePanel == FileTreePanel {
					m.activePanel = DiffViewerPanel
				} else {
					m.activePanel = FileTreePanel
				}
			}
			return m, nil
		case key.Matches(msg, keys.PrevFile):
			m, cmd = m.moveToFile(-1)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		case key.Matches(msg, keys.NextFile):
			m, cmd = m.moveToFile(1)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)

		// Scroll-oriented keybindings always operate on the diff viewer.
		case key.Matches(msg, keys.ScrollTop):
			m.diffViewer.GoToTop()
			return m, nil
		case key.Matches(msg, keys.ScrollBottom):
			m.diffViewer.GoToBottom()
			return m, nil
		case key.Matches(msg, keys.CtrlF):
			m.diffViewer.PageDown()
			return m, nil
		case key.Matches(msg, keys.CtrlB):
			m.diffViewer.PageUp()
			return m, nil
		case key.Matches(msg, keys.CtrlD):
			m.diffViewer.PageDown()
			return m, nil
		case key.Matches(msg, keys.CtrlU):
			m.diffViewer.PageUp()
			return m, nil

		// Tree traversal keys operate on whichever view is active.
		case key.Matches(msg, keys.Up):
			if m.activePanel == FileTreePanel {
				m, cmd = m.moveCursor(-1)
				cmds = append(cmds, cmd)
			} else {
				m.diffViewer.ScrollUp(1)
			}
			return m, tea.Batch(cmds...)
		case key.Matches(msg, keys.Down):
			if m.activePanel == FileTreePanel {
				m, cmd = m.moveCursor(1)
				cmds = append(cmds, cmd)
			} else {
				m.diffViewer.ScrollDown(1)
			}
			return m, tea.Batch(cmds...)

		// Expand/collapse/toggle operate on the file tree.
		case key.Matches(msg, keys.ExpandNode):
			if m.activePanel == FileTreePanel {
				m.fileTree.ExpandAndDescend()
				node := m.fileTree.GetCurrNode()
				m, cmd = m.setNodeDiff(node)
				cmds = append(cmds, cmd)
				m.diffViewer.GoToTop()
			}
			return m, tea.Batch(cmds...)
		case key.Matches(msg, keys.CollapseNode):
			if m.activePanel == FileTreePanel {
				m.fileTree.CollapseOrMoveToParent()
				node := m.fileTree.GetCurrNode()
				m, cmd = m.setNodeDiff(node)
				cmds = append(cmds, cmd)
				m.diffViewer.GoToTop()
			}
			return m, tea.Batch(cmds...)
		case key.Matches(msg, keys.ToggleNode):
			if m.activePanel == FileTreePanel {
				m.fileTree.ToggleCurrentNode()
				node := m.fileTree.GetCurrNode()
				m, cmd = m.setNodeDiff(node)
				cmds = append(cmds, cmd)
				m.diffViewer.GoToTop()
			}
			return m, tea.Batch(cmds...)

		case key.Matches(msg, keys.Copy):
			cmd = m.fileTree.CopyCurrNodePath()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		case key.Matches(msg, keys.OpenInEditor):
			cmd = m.openInEditor()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

	case tea.WindowSizeMsg:
		log.Info("got tea.WindowSizeMsg", "width", msg.Width, "height", msg.Height)
		m.help.Update(msg)
		m.width = msg.Width
		m.height = msg.Height
		dfCmd := m.diffViewer.SetSize(m.width-m.sidebarWidth(), m.mainContentHeight())
		cmds = append(cmds, dfCmd)

		tWidth, tHeight := m.sidebarWidth(), m.mainContentHeight()-filterHeight

		m.fileTree.SetSize(tWidth, tHeight)
		m.filter.SetWidth(m.filterWidth())

	case fileTreeMsg:
		m.files = msg.files
		if len(m.files) == 0 {
			return m, tea.Quit
		}
		m.fileTree = m.fileTree.SetFiles(m.files)
		m.diffViewer.SetPreamble(strings.TrimSpace(msg.preamble))
		m.diffViewer, cmd = m.diffViewer.SetDirPatch("/", m.fileTree.GetCurrNodeDesendantDiffs())
		cmds = append(cmds, cmd)

	case common.ErrMsg:
		fmt.Printf("Error: %v\n", msg.Err)
		log.Fatal(msg.Err)
	}

	// Route non-key messages to sub-components for internal processing
	// (e.g. diffContentMsg). All key events return early above or are
	// filtered out, so only non-key messages reach this point.
	m.diffViewer, cmd = m.diffViewer.Update(msg)
	cmds = append(cmds, cmd)
	m.fileTree.Update(msg)

	return m, tea.Batch(cmds...)
}

func (m *mainModel) mainContentHeight() int {
	return m.height - m.headerHeight() - m.footerHeight()
}

func (m *mainModel) cycleIconStyle() {
	switch m.iconStyle {
	case filenode.IconsASCII:
		m.iconStyle = filenode.IconsUnicode
	case filenode.IconsUnicode:
		m.iconStyle = filenode.IconsNerdStatus
	case filenode.IconsNerdStatus:
		m.iconStyle = filenode.IconsNerdSimple
	case filenode.IconsNerdSimple:
		m.iconStyle = filenode.IconsNerdFiletype
	case filenode.IconsNerdFiletype:
		m.iconStyle = filenode.IconsNerdFull
	default:
		m.iconStyle = filenode.IconsASCII
	}
	m.fileTree.SetIconStyle(m.iconStyle)
}

func (m mainModel) filterUpdate(msg tea.Msg) (mainModel, []tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Filter out key release events to prevent double-firing.
	if _, ok := msg.(tea.KeyReleaseMsg); ok {
		return m, nil
	}

	if m.filter.Focused() {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "ctrl+c":
				return m, []tea.Cmd{tea.Quit}
			case "enter":
				m.stopFilter()
				dfCmd := m.diffViewer.SetSize(m.width-m.sidebarWidth(), m.mainContentHeight())
				cmds = append(cmds, dfCmd)

				if selected, ok := m.selectedFilterResult(); ok {
					for _, f := range m.files {
						if filenode.GetFileName(f) == selected {
							m.diffViewer, cmd = m.diffViewer.SetFilePatch(f)
							m.fileTree.SetCursorByPath(filenode.GetFileName(f))
							cmds = append(cmds, cmd)
							break
						}
					}
				}

			case "ctrl+n", "down":
				if len(m.filtered) > 0 {
					m.resultsCursor = min(len(m.filtered)-1, m.resultsCursor+1)
					m.resultsVp.ScrollDown(1)
				}
			case "ctrl+p", "up":
				if len(m.filtered) > 0 {
					m.resultsCursor = max(0, m.resultsCursor-1)
					m.resultsVp.ScrollUp(1)
				}
			default:
				m.resultsCursor = 0
			}
		}
		s, sc := m.filter.Update(msg)
		cmds = append(cmds, sc)
		m.filter = s
		m.setFilterResults()
		m.resultsVp.SetContent(m.resultsView())
	}

	return m, cmds
}

func (m mainModel) View() tea.View {
	var view tea.View
	view.AltScreen = true
	view.MouseMode = tea.MouseModeAllMotion

	// Determine colors based on active panel.
	leftColor := lipgloss.Color("8")
	rightColor := lipgloss.Color("8")
	if m.activePanel == FileTreePanel && !m.filtering {
		leftColor = lipgloss.Color("4")
	} else if m.activePanel == DiffViewerPanel {
		rightColor = lipgloss.Color("4")
	}

	// Build T-shaped separator line.
	separator := ""
	if m.width > 0 {
		if m.isSidebarVisible() {
			sidebarW := m.sidebarWidth()
			rightW := max(m.width-sidebarW, 0)
			leftLine := lipgloss.NewStyle().
				Foreground(leftColor).
				Render(strings.Repeat("─", sidebarW))
			junction := lipgloss.NewStyle().Foreground(leftColor).Render("┬")
			rightLine := lipgloss.NewStyle().
				Foreground(rightColor).
				Render(strings.Repeat("─", rightW))
			separator = leftLine + junction + rightLine
		} else {
			separator = lipgloss.NewStyle().Foreground(rightColor).Render(strings.Repeat("─", m.width))
		}
	}

	sidebar := ""
	if m.isSidebarVisible() {
		filterBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Width(m.sidebarWidth()).
			Render(m.filter.View())
		filterBox = zone.Mark(zoneFilterBox, filterBox)

		content := ""
		if m.filtering {
			content = zone.Mark(zoneFilterResults, m.resultsVp.View())
		} else {
			content = zone.Mark(zoneFileTree, m.fileTree.View())
		}
		content = lipgloss.NewStyle().
			Render(lipgloss.JoinVertical(lipgloss.Left, filterBox, content))

		sidebar = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(leftColor).Render(content)
	} else {
		// Show a thin grab line when sidebar is hidden.
		// Width(0) means only the border is rendered (1 char).
		grabLine := lipgloss.NewStyle().
			Width(0).
			Height(m.mainContentHeight()-1).
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("8")).
			Render("")
		sidebar = grabLine
	}

	dv := zone.Mark(zoneDiffViewer, m.diffViewer.View())
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, dv)

	var sections []string

	if !m.config.UI.HideHeader {
		header := lipgloss.NewStyle().Width(m.width).
			Foreground(lipgloss.Color("6")).
			Bold(true).
			Render("DIFFNAV")
		sections = append(sections, header)
	}

	sections = append(sections, separator)
	sections = append(sections, mainContent)

	if !m.config.UI.HideFooter {
		sections = append(sections, m.footerView())
	}

	appView := zone.Scan(lipgloss.JoinVertical(lipgloss.Left, sections...))
	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(appView),
	}

	if m.helpOpen {
		helpView := m.help.View()
		s := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			Padding(1, 3).
			BorderForeground(lipgloss.Blue)
		row := m.height/4 - 2 // just a bit above the center
		col := m.width / 2
		col -= lipgloss.Width(helpView) / 2
		layers = append(
			layers,
			lipgloss.NewLayer(s.Render(helpView)).X(col).Y(row),
		)
	}

	comp := lipgloss.NewCompositor(layers...)

	view.Content = comp.Render()

	return view
}

type fileTreeMsg struct {
	files    []*gitdiff.File
	preamble string
}

func (m mainModel) fetchFileTree() tea.Msg {
	// TODO: handle error
	files, preamble, err := gitdiff.Parse(strings.NewReader(m.input + "\n"))
	if err != nil {
		return common.ErrMsg{Err: err}
	}
	sortFiles(files)

	return fileTreeMsg{files: files, preamble: preamble}
}

func (m mainModel) footerView() string {
	base := lipgloss.NewStyle().Background(common.Colors[common.DarkerSelected])
	files := fmt.Sprintf(" %d files", len(m.files))
	sep := lipgloss.NewStyle().Foreground(lipgloss.BrightBlack).Render(" • ")
	added, deleted := m.diffViewer.RootDiffStats()
	help := base.Background(lipgloss.BrightBlack).PaddingLeft(1).PaddingRight(1).Render("F1/? help")
	stats := filenode.ViewDiffStats(added, deleted, base)
	spacing := base.Render(strings.Repeat(" ", max(0, m.width-lipgloss.Width(stats)-
		lipgloss.Width(help)-lipgloss.Width(files)-lipgloss.Width(sep))))
	return base.
		Width(m.width).
		Height(1).
		Render(lipgloss.JoinHorizontal(lipgloss.Top, files, sep, stats, spacing, help))
}

func (m mainModel) resultsView() string {
	sb := strings.Builder{}
	query := m.filter.Value()
	for i, f := range m.filtered {
		fName := utils.TruncateString(" "+f, m.config.UI.FilterTreeWidth-2)
		base := lipgloss.NewStyle()
		if i == m.resultsCursor {
			base = base.Background(lipgloss.Color("#1b1b33")).Bold(true)
		}
		sb.WriteString(highlightMatch(fName, query, base) + "\n")
	}
	return sb.String()
}

// highlightMatch renders s with the first case-insensitive occurrence of query
// underlined. If query is empty or not found the full string is returned
// styled with base only.
//
// The match position is computed by scanning s rune-by-rune and comparing with
// strings.EqualFold. This avoids the byte-offset mismatch that can occur when
// strings.ToLower changes byte lengths for certain Unicode characters (e.g.
// Kelvin sign U+212A lowercases to ASCII 'k').
func highlightMatch(s, query string, base lipgloss.Style) string {
	if query == "" {
		return base.Render(s)
	}
	qRunes := utf8.RuneCountInString(query)
	// Slide a window of qRunes runes across s, comparing with EqualFold.
	i := 0
	for i < len(s) {
		// Advance j past qRunes runes starting at i.
		j := i
		n := 0
		for n < qRunes && j < len(s) {
			_, size := utf8.DecodeRuneInString(s[j:])
			j += size
			n++
		}
		if n < qRunes {
			// Not enough runes remaining for a match.
			break
		}
		if strings.EqualFold(s[i:j], query) {
			hl := base.Copy().Underline(true)
			return base.Render(s[:i]) + hl.Render(s[i:j]) + base.Render(s[j:])
		}
		// Advance i by one rune.
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return base.Render(s)
}

func (m mainModel) sidebarWidth() int {
	if m.filtering {
		return m.config.UI.FilterTreeWidth
	}

	if m.isShowingFileTree {
		return m.fileTree.Width()
	}

	return 0
}

func (m mainModel) headerHeight() int {
	if m.config.UI.HideHeader {
		return 0
	}
	return headerHeight
}

func (m mainModel) footerHeight() int {
	if m.config.UI.HideFooter {
		return 0
	}
	return footerHeight
}

func (m *mainModel) filterWidth() int {
	return max(0, m.sidebarWidth()-5)
}

func (m *mainModel) stopFilter() {
	m.filtering = false
	m.filter.SetValue("")
	m.filter.Blur()
	m.filter.SetWidth(m.filterWidth())
}

func (m *mainModel) closeHelp() {
	m.helpOpen = false
}

func (m mainModel) openInEditor() tea.Cmd {
	if len(m.files) == 0 {
		return nil
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		return nil
	}

	fullpath := m.fileTree.CurrNodePath()
	c := exec.Command(editor, fullpath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return nil
	})
}

func (m mainModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle scroll wheel first.
	if msg.Mouse().Button == tea.MouseWheelUp || msg.Mouse().Button == tea.MouseWheelDown {
		return m.handleScroll(msg)
	}

	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			// Keep coordinate check for resize border (hybrid approach).
			sidebarWidth := m.sidebarWidth()
			if !m.filtering && m.isShowingFileTree && abs(msg.X-sidebarWidth) <= sidebarGrabThreshold {
				m.draggingSidebar = true
				return m, nil
			}
			// Allow grabbing the line when sidebar is hidden.
			if !m.isSidebarVisible() && msg.X <= sidebarGrabThreshold {
				m.draggingSidebar = true
				m.isShowingFileTree = true
				return m, nil
			}

			// Zone-based detection for everything else.
			if zone.Get(zoneFilterBox).InBounds(msg) {
				return m.handleFilterBoxClick()
			}
			if m.filtering && zone.Get(zoneFilterResults).InBounds(msg) {
				return m.handleFilterResultClick(msg)
			}
			if !m.filtering && zone.Get(zoneFileTree).InBounds(msg) {
				return m.handleFileTreeClick(msg)
			}
		}

	case tea.MouseReleaseMsg:
		m.draggingSidebar = false

	case tea.MouseMotionMsg:
		if m.draggingSidebar {
			return m.handleSidebarDrag(msg)
		}
	}

	return m, nil
}

func (m mainModel) handleFilterResultClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Use zone-relative coordinates.
	_, y := zone.Get(zoneFilterResults).Pos(msg)
	if y < 0 {
		return m, nil
	}
	clickedIndex := y + m.resultsVp.YOffset()
	if clickedIndex >= len(m.filtered) {
		return m, nil
	}

	// Select the clicked result.
	selected := m.filtered[clickedIndex]
	m.stopFilter()

	var cmd tea.Cmd
	var cmds []tea.Cmd
	dfCmd := m.diffViewer.SetSize(m.width-m.sidebarWidth(), m.mainContentHeight())
	cmds = append(cmds, dfCmd)

	for _, f := range m.files {
		if filenode.GetFileName(f) == selected {
			m.diffViewer, cmd = m.diffViewer.SetFilePatch(f)
			m.fileTree.SetCursorByPath(filenode.GetFileName(f))
			cmds = append(cmds, cmd)
			break
		}
	}

	return m, tea.Batch(cmds...)
}

func (m mainModel) handleFilterBoxClick() (tea.Model, tea.Cmd) {
	if m.filtering {
		return m, nil
	}
	m.filtering = true
	m.filter.SetWidth(m.filterWidth())
	m.filter.SetValue("")
	m.resultsCursor = 0
	m.setFilterResults()

	m.resultsVp.SetWidth(m.config.UI.FilterTreeWidth)
	m.resultsVp.SetHeight(m.mainContentHeight() - filterHeight)
	m.resultsVp.SetContent(m.resultsView())

	dfCmd := m.diffViewer.SetSize(m.width-m.sidebarWidth(), m.mainContentHeight())
	return m, tea.Batch(dfCmd, m.filter.Focus())
}

func (m mainModel) handleFileTreeClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Use zone-relative coordinates.
	_, y := zone.Get(zoneFileTree).Pos(msg)
	if y < 0 {
		return m, nil
	}
	clickedY := y + m.fileTree.ViewportYOffset()

	node := m.fileTree.GetNodeAtY(clickedY)
	if node == nil {
		return m, nil
	}

	var cmd tea.Cmd
	m, cmd = m.setNodeDiff(node)
	// Use SetCursorNoScroll to avoid jumping the file tree view.
	m.fileTree.SetCursorNoScroll(node.YOffset())
	return m, cmd
}

func (m mainModel) handleScroll(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	lines := scrollLines

	// Check if scrolling in sidebar (file tree or filter results).
	if zone.Get(zoneFileTree).InBounds(msg) || zone.Get(zoneFilterResults).InBounds(msg) {
		if msg.Mouse().Button == tea.MouseWheelUp {
			if m.filtering {
				m.resultsVp.ScrollUp(lines)
			} else {
				m.fileTree.ScrollUp(lines)
			}
		} else {
			if m.filtering {
				m.resultsVp.ScrollDown(lines)
			} else {
				m.fileTree.ScrollDown(lines)
			}
		}
		return m, nil
	}

	// Check if scrolling in diff viewer.
	if zone.Get(zoneDiffViewer).InBounds(msg) {
		if msg.Mouse().Button == tea.MouseWheelUp {
			m.diffViewer.ScrollUp(lines)
		} else {
			m.diffViewer.ScrollDown(lines)
		}
	}
	return m, nil
}

func (m mainModel) handleSidebarDrag(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		m.draggingSidebar = false
		return m, nil
	}

	// Hide sidebar if dragged below threshold.
	if msg.Mouse().X < sidebarHideWidth {
		m.isShowingFileTree = false
		m.draggingSidebar = false
		cmd := m.diffViewer.SetSize(m.width, m.mainContentHeight())
		return m, cmd
	}

	// Clamp to reasonable bounds.
	minWidth := sidebarMinWidth
	maxWidth := m.width / 2
	newWidth := max(minWidth, min(maxWidth, msg.Mouse().X))

	// TODO: for some reason setting a value smaller than minResizeStep
	// will garble up the output when resizing. I have no idea why.
	if abs(newWidth-m.sidebarWidth()) < minResizeStep {
		return m, nil
	}

	// Resize components.
	cmds := []tea.Cmd{}

	cmds = append(cmds, m.diffViewer.SetSize(m.width-newWidth, m.mainContentHeight()))
	m.fileTree.SetSize(newWidth-1, m.mainContentHeight()-filterHeight-1)

	return m, tea.Batch(cmds...)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (m mainModel) moveToFile(movement int) (mainModel, tea.Cmd) {
	var cmd tea.Cmd
	var moved bool
	switch movement {
	case -1:
		moved = m.fileTree.PrevFile()
	case 1:
		moved = m.fileTree.NextFile()
	}

	if !moved {
		return m, nil
	}

	node := m.fileTree.GetCurrNode()
	m, cmd = m.setNodeDiff(node)
	m.diffViewer.GoToTop()

	return m, cmd
}

func (m mainModel) moveCursor(movement int) (mainModel, tea.Cmd) {
	var cmd tea.Cmd
	switch movement {
	case -1:
		m.fileTree.Up()
	case 1:
		m.fileTree.Down()
	}

	node := m.fileTree.GetCurrNode()
	m, cmd = m.setNodeDiff(node)
	m.diffViewer.GoToTop()

	return m, cmd
}

func (m mainModel) setNodeDiff(node *tree.Node) (mainModel, tea.Cmd) {
	var cmd tea.Cmd
	switch val := node.GivenValue().(type) {
	case *filenode.FileNode:
		m.diffViewer, cmd = m.diffViewer.SetFilePatch(val.File)
	case string, *dirnode.DirNode:
		files := m.fileTree.GetCurrNodeDesendantDiffs()

		fullPath := "/"
		if val, ok := node.GivenValue().(*dirnode.DirNode); ok {
			fullPath = val.FullPath
		}
		m.diffViewer, cmd = m.diffViewer.SetDirPatch(fullPath, files)
	}

	return m, cmd
}

func (m *mainModel) setFilterResults() {
	filtered := make([]string, 0)
	for _, f := range m.files {
		if strings.Contains(
			strings.ToLower(filenode.GetFileName(f)),
			strings.ToLower(m.filter.Value()),
		) {
			filtered = append(filtered, filenode.GetFileName(f))
		}
	}
	m.filtered = filtered
	switch {
	case len(m.filtered) == 0:
		m.resultsCursor = 0
	case m.resultsCursor < 0:
		m.resultsCursor = 0
	case m.resultsCursor >= len(m.filtered):
		m.resultsCursor = len(m.filtered) - 1
	}
}

func (m mainModel) selectedFilterResult() (string, bool) {
	if len(m.filtered) == 0 {
		return "", false
	}
	if m.resultsCursor < 0 || m.resultsCursor >= len(m.filtered) {
		return "", false
	}
	return m.filtered[m.resultsCursor], true
}

func (m mainModel) isSidebarVisible() bool {
	return m.isShowingFileTree || m.filtering
}
