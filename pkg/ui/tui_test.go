package ui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/diffnav/pkg/config"
)

func TestSearchUpdateEnterWithNoResultsDoesNotPanic(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.searching = true
	m.search.Focus()
	m.search.SetValue("does-not-match")
	m.setSearchResults()

	updated, _ := m.searchUpdate(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	if updated.searching {
		t.Fatal("expected search to stop after pressing enter")
	}
	if updated.resultsCursor != 0 {
		t.Fatalf("expected cursor to remain at 0, got %d", updated.resultsCursor)
	}
}

func TestSearchUpdateKeepsCursorValidWhenResultsAreEmpty(t *testing.T) {
	m := newTestMainModel(t)
	m.searching = true
	m.search.Focus()
	m.filtered = nil
	m.resultsCursor = 0

	updated, _ := m.searchUpdate(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if updated.resultsCursor != 0 {
		t.Fatalf("expected cursor to remain at 0 after down on empty results, got %d", updated.resultsCursor)
	}

	updated.resultsCursor = -3
	updated.search.SetValue("does-not-match")
	updated.setSearchResults()
	if updated.resultsCursor != 0 {
		t.Fatalf("expected cursor to clamp to 0 for empty results, got %d", updated.resultsCursor)
	}
}

func TestSearchResultsRenderWhenFileTreeIsHidden(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.isShowingFileTree = false
	m.searching = true
	m.search.SetWidth(m.searchWidth())
	m.setSearchResults()
	m.resultsVp.SetWidth(m.config.UI.SearchTreeWidth)
	m.resultsVp.SetHeight(m.mainContentHeight() - searchHeight)
	m.resultsVp.SetContent(m.resultsView())

	view := m.View().Content
	if !strings.Contains(view, "yarn.lock") {
		t.Fatal("expected search results to be visible even when the file tree is hidden")
	}
}

func TestHiddenTreeSearchEnterThenToggleDoesNotPanic(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40

	m = updateMainModel(t, m, tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	m = updateMainModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF3}))
	m = updateMainModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updateMainModel(t, m, tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))

	if !m.isShowingFileTree {
		t.Fatal("expected file tree to be visible after toggling it back on")
	}
	if m.search.Width() < 0 {
		t.Fatalf("expected non-negative search width, got %d", m.search.Width())
	}
	_ = m.View().Content
}

func TestHiddenTreeSearchClickNearLeftEdgeDoesNotShowFileTree(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.isShowingFileTree = false
	m.searching = true

	updated, _ := m.handleMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: 1, Button: tea.MouseLeft}))

	result, ok := updated.(mainModel)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}
	if result.isShowingFileTree {
		t.Fatal("expected left-edge click during hidden-tree search to leave the file tree hidden")
	}
}

func TestHiddenSidebarGrabStillShowsFileTreeWhenNotSearching(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.isShowingFileTree = false
	m.searching = false

	updated, _ := m.handleMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: 1, Button: tea.MouseLeft}))

	result, ok := updated.(mainModel)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}
	if !result.isShowingFileTree {
		t.Fatal("expected left-edge click on the hidden sidebar grab line to show the file tree")
	}
}

func TestSearchSidebarBorderClickDoesNotStartDragging(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.isShowingFileTree = true
	m.searching = true
	m.fileTree.SetSize(m.config.UI.FileTreeWidth, m.mainContentHeight()-searchHeight)

	updated, _ := m.handleMouse(tea.MouseClickMsg(tea.Mouse{
		X:      m.sidebarWidth(),
		Y:      1,
		Button: tea.MouseLeft,
	}))

	result, ok := updated.(mainModel)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}
	if result.draggingSidebar {
		t.Fatal("expected sidebar dragging to stay disabled while searching")
	}
}

func TestSearchSidebarDragMotionIsIgnored(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.isShowingFileTree = true
	m.searching = true
	m.draggingSidebar = true
	m.fileTree.SetSize(m.config.UI.FileTreeWidth, m.mainContentHeight()-searchHeight)

	updated, _ := m.handleMouse(tea.MouseMotionMsg(tea.Mouse{
		X:      40,
		Y:      1,
		Button: tea.MouseLeft,
	}))

	result, ok := updated.(mainModel)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}
	if result.draggingSidebar {
		t.Fatal("expected search-mode drag motion to clear dragging state")
	}
	if result.fileTree.Width() != m.fileTree.Width() {
		t.Fatalf("expected file tree width to remain %d, got %d", m.fileTree.Width(), result.fileTree.Width())
	}
}

func newTestMainModel(t *testing.T) mainModel {
	t.Helper()
	zone.NewGlobal()

	cfg := config.DefaultConfig()
	data, err := os.ReadFile("../../examples/multiple_files.txt")
	if err != nil {
		t.Fatal(err)
	}

	files, _, err := gitdiff.Parse(strings.NewReader(string(data) + "\n"))
	if err != nil {
		t.Fatal(err)
	}

	m := New(string(data), cfg)
	m.files = files
	m.fileTree = m.fileTree.SetFiles(files)

	return m
}

func updateMainModel(t *testing.T, m mainModel, msg tea.Msg) mainModel {
	t.Helper()

	updated, _ := m.Update(msg)
	result, ok := updated.(mainModel)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}

	return result
}

func escMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
}

// TestEscDismissesHelpPopover verifies priority 1: when the help popover is
// visible, Esc dismisses it.
func TestEscDismissesHelpPopover(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.helpOpen = true

	m = updateMainModel(t, m, escMsg())

	if m.helpOpen {
		t.Fatal("expected help popover to be closed after pressing Esc")
	}
}

// TestEscSwitchesFromDiffViewToExplorer verifies priority 2: when the diff
// view pane is active, Esc activates the explorer pane.
func TestEscSwitchesFromDiffViewToExplorer(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.activePanel = DiffViewerPanel

	m = updateMainModel(t, m, escMsg())

	if m.activePanel != FileTreePanel {
		t.Fatal("expected Esc to switch from diff viewer to file tree panel")
	}
}

// TestEscCancelsSearchMode verifies priority 3: when filter files mode is
// active (with explorer pane focused), Esc returns to normal tree explorer.
func TestEscCancelsSearchMode(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.searching = true
	m.activePanel = FileTreePanel
	m.search.Focus()

	m = updateMainModel(t, m, escMsg())

	if m.searching {
		t.Fatal("expected Esc to cancel search/filter mode")
	}
}

// TestEscDiffViewTakesPriorityOverSearch verifies that when the diff view is
// active during search, Esc first switches to the explorer pane (priority 2)
// before stopping search (priority 3).
func TestEscDiffViewTakesPriorityOverSearch(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.searching = true
	m.activePanel = DiffViewerPanel
	m.search.Focus()

	m = updateMainModel(t, m, escMsg())

	if m.activePanel != FileTreePanel {
		t.Fatal("expected Esc to switch from diff viewer to file tree panel first")
	}
	if !m.searching {
		t.Fatal("expected search to remain active after first Esc (diff view priority)")
	}

	// Second Esc should cancel search.
	m = updateMainModel(t, m, escMsg())

	if m.searching {
		t.Fatal("expected second Esc to cancel search mode")
	}
}

// TestEscHelpTakesPriorityOverDiffView verifies that when the help popover is
// visible with diff view active, Esc first closes help (priority 1).
func TestEscHelpTakesPriorityOverDiffView(t *testing.T) {
	m := newTestMainModel(t)
	m.width = 100
	m.height = 40
	m.helpOpen = true
	m.activePanel = DiffViewerPanel

	m = updateMainModel(t, m, escMsg())

	if m.helpOpen {
		t.Fatal("expected help popover to be closed after pressing Esc")
	}
	if m.activePanel != DiffViewerPanel {
		t.Fatal("expected diff viewer to still be active after closing help")
	}
}

func TestHighlightMatchUnderlines(t *testing.T) {
	base := lipgloss.NewStyle()
	result := highlightMatch("football", "foo", base)
	stripped := ansi.Strip(result)
	// After stripping ANSI codes, the full text should be present.
	if stripped != "football" {
		t.Fatalf("expected stripped result to be %q, got %q", "football", stripped)
	}
	// The raw output must contain ANSI underline escape (SGR 4).
	if !strings.Contains(result, "\x1b[") {
		t.Fatal("expected ANSI escape codes for underline styling")
	}
}

func TestHighlightMatchCaseInsensitive(t *testing.T) {
	base := lipgloss.NewStyle()
	result := highlightMatch("Football", "foo", base)
	stripped := ansi.Strip(result)
	// Original case should be preserved.
	if !strings.Contains(stripped, "Foo") {
		t.Fatalf("expected stripped result to contain %q, got %q", "Foo", stripped)
	}
}

func TestHighlightMatchEmptyQuery(t *testing.T) {
	base := lipgloss.NewStyle()
	result := highlightMatch("football", "", base)
	stripped := ansi.Strip(result)
	if stripped != "football" {
		t.Fatalf("expected stripped result to be %q, got %q", "football", stripped)
	}
}

func TestHighlightMatchNoMatch(t *testing.T) {
	base := lipgloss.NewStyle()
	result := highlightMatch("football", "xyz", base)
	stripped := ansi.Strip(result)
	if stripped != "football" {
		t.Fatalf("expected stripped result to be %q, got %q", "football", stripped)
	}
}
