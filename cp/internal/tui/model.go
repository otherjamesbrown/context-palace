package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otherjamesbrown/context-palace/cp/internal/client"
)

// BrowseModel is the top-level Bubble Tea model
type BrowseModel struct {
	client        *client.Client
	includeClosed bool
	keys          KeyMap
	styles        Styles

	// Type tabs
	types      []client.ShardType
	activeType int

	// Tree state
	roots    []*TreeNode
	flatList []*TreeNode
	cursor   int
	nodeMap  map[string]*TreeNode // id -> node for quick lookup

	// Board tab
	boardMode   bool
	boardResult *client.BoardResult

	// Detail pane
	detail        *client.ShardDetailResult
	detailID      string // ID of shard being displayed or loading
	detailLoading bool
	detailVP      viewport.Model

	// Layout
	width     int
	height    int
	focusLeft bool // true = tree focused, false = detail focused
	treeWidth int

	// Loading state
	loading    bool
	loadingMsg string
	errMsg     string
}

// NewBrowseModel creates a new browse model
func NewBrowseModel(c *client.Client, includeClosed bool) BrowseModel {
	return BrowseModel{
		client:        c,
		includeClosed: includeClosed,
		keys:          DefaultKeyMap(),
		styles:        DefaultStyles(),
		focusLeft:     true,
		nodeMap:       make(map[string]*TreeNode),
		loading:       true,
		loadingMsg:    "Loading board...",
		boardMode:     true,
	}
}

// -- Messages --

type typesLoadedMsg struct {
	types []client.ShardType
}

type rootsLoadedMsg struct {
	shardType string
	nodes     []client.ShardTreeNode
}

type childrenLoadedMsg struct {
	parentID string
	nodes    []client.ShardTreeNode
}

type detailLoadedMsg struct {
	id     string
	detail *client.ShardDetailResult
}

type boardLoadedMsg struct {
	result *client.BoardResult
}

type typesRefreshedMsg struct {
	types []client.ShardType
}

type errMsg struct {
	err error
}

// -- Commands --

func (m BrowseModel) loadTypes() tea.Msg {
	ctx := context.Background()
	types, err := m.client.GetShardTypes(ctx, m.includeClosed)
	if err != nil {
		return errMsg{err}
	}
	return typesLoadedMsg{types}
}

func (m BrowseModel) refreshTypes() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		types, err := m.client.GetShardTypes(ctx, m.includeClosed)
		if err != nil {
			return errMsg{err}
		}
		return typesRefreshedMsg{types}
	}
}

func (m BrowseModel) loadBoard() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := m.client.GetBoardShards(ctx, client.BoardOpts{})
		if err != nil {
			return errMsg{err}
		}
		return boardLoadedMsg{result: result}
	}
}

func (m BrowseModel) loadRoots(shardType string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		nodes, err := m.client.GetShardTreeRoots(ctx, shardType, m.includeClosed)
		if err != nil {
			return errMsg{err}
		}
		return rootsLoadedMsg{shardType: shardType, nodes: nodes}
	}
}

func (m BrowseModel) loadChildren(parentID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		nodes, err := m.client.GetShardTreeChildren(ctx, parentID, m.includeClosed)
		if err != nil {
			return errMsg{err}
		}
		return childrenLoadedMsg{parentID: parentID, nodes: nodes}
	}
}

func (m BrowseModel) loadDetail(id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		detail, err := m.client.GetShardDetail(ctx, id)
		if err != nil {
			return errMsg{err}
		}
		// For knowledge shards, also fetch children
		if detail.Type == "knowledge" {
			children, err := m.client.ListKnowledgeChildren(ctx, id)
			if err == nil && len(children) > 0 {
				detail.KnowledgeChildren = children
			}
		}
		return detailLoadedMsg{id: id, detail: detail}
	}
}

// -- Init --

func (m BrowseModel) Init() tea.Cmd {
	return tea.Batch(m.loadBoard(), m.loadTypes)
}

// -- Update --

func (m BrowseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.treeWidth = m.width * 45 / 100
		if m.treeWidth < 30 {
			m.treeWidth = 30
		}
		detailWidth := m.width - m.treeWidth - 3 // border chars
		detailHeight := m.height - 5              // tabs + status bar + borders
		m.detailVP = viewport.New(detailWidth, detailHeight)
		if m.detail != nil {
			m.detailVP.SetContent(RenderDetail(m.detail, detailWidth, m.styles))
		}
		return m, nil

	case boardLoadedMsg:
		m.boardResult = msg.result
		if !m.boardMode {
			return m, nil
		}
		m.loading = false
		m.roots = BuildBoardTree(msg.result)
		m.nodeMap = make(map[string]*TreeNode)
		for _, root := range m.roots {
			addToMap(root, m.nodeMap)
		}
		m.flatList = Flatten(m.roots)
		m.cursor = 0
		m.errMsg = ""
		if len(m.flatList) > 0 {
			m, cmd := m.maybeLoadDetail()
			return m, cmd
		}
		m.detail = nil
		m.detailID = ""
		return m, nil

	case typesRefreshedMsg:
		m.types = msg.types
		return m, nil

	case typesLoadedMsg:
		m.types = msg.types
		if m.boardMode {
			// Types loaded in background; don't switch away from board
			return m, nil
		}
		m.loading = false
		m.activeType = 0
		if len(m.types) > 0 {
			m.loading = true
			m.loadingMsg = fmt.Sprintf("Loading %s...", m.types[0].Type)
			return m, m.loadRoots(m.types[0].Type)
		}
		return m, nil

	case rootsLoadedMsg:
		// Discard stale results
		if m.boardMode {
			return m, nil
		}
		if len(m.types) > 0 && m.types[m.activeType].Type != msg.shardType {
			return m, nil
		}
		m.loading = false
		m.roots = BuildRoots(msg.nodes)
		// Apply per-type grouping
		switch msg.shardType {
		case "message":
			m.roots = GroupByDate(m.roots)
		case "memory":
			m.roots = GroupByLabel(m.roots)
		}
		m.nodeMap = make(map[string]*TreeNode)
		for _, root := range m.roots {
			addToMap(root, m.nodeMap)
		}
		m.flatList = Flatten(m.roots)
		m.cursor = 0
		m.errMsg = ""
		// Auto-load detail for first item
		if len(m.flatList) > 0 {
			m, cmd := m.maybeLoadDetail()
			return m, cmd
		}
		m.detail = nil
		m.detailID = ""
		return m, nil

	case childrenLoadedMsg:
		node, ok := m.nodeMap[msg.parentID]
		if !ok {
			return m, nil
		}
		InsertChildren(node, msg.nodes)
		// Add children to node map
		for _, child := range node.Children {
			m.nodeMap[child.ID] = child
		}
		node.Expanded = true
		m.flatList = Flatten(m.roots)
		return m, nil

	case detailLoadedMsg:
		// Discard stale results
		if m.detailID != msg.id {
			return m, nil
		}
		m.detail = msg.detail
		m.detailLoading = false
		detailWidth := m.width - m.treeWidth - 3
		m.detailVP.SetContent(RenderDetail(m.detail, detailWidth, m.styles))
		m.detailVP.GotoTop()
		return m, nil

	case errMsg:
		m.loading = false
		m.errMsg = msg.err.Error()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to viewport if detail focused
	if !m.focusLeft {
		var cmd tea.Cmd
		m.detailVP, cmd = m.detailVP.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m BrowseModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Quit always works
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}

	// Tab switches pane focus
	if key.Matches(msg, m.keys.Tab) {
		m.focusLeft = !m.focusLeft
		return m, nil
	}

	// Type switching: number keys (1=Board, 2-9=shard types)
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if r == '1' {
			return m.switchToBoard()
		}
		if r >= '2' && r <= '9' {
			idx := int(r - '2') // '2' -> types[0], '3' -> types[1], etc.
			if idx < len(m.types) {
				return m.switchType(idx)
			}
			return m, nil
		}
	}

	// Type cycling: [ and ] cycle through board + shard types
	// Total tabs = 1 (board) + len(types)
	if key.Matches(msg, m.keys.PrevType) {
		totalTabs := 1 + len(m.types)
		if totalTabs <= 1 {
			return m, nil
		}
		current := 0
		if !m.boardMode {
			current = m.activeType + 1
		}
		prev := (current - 1 + totalTabs) % totalTabs
		if prev == 0 {
			return m.switchToBoard()
		}
		return m.switchType(prev - 1)
	}
	if key.Matches(msg, m.keys.NextType) {
		totalTabs := 1 + len(m.types)
		if totalTabs <= 1 {
			return m, nil
		}
		current := 0
		if !m.boardMode {
			current = m.activeType + 1
		}
		next := (current + 1) % totalTabs
		if next == 0 {
			return m.switchToBoard()
		}
		return m.switchType(next - 1)
	}

	// Shift+Up/Down: scroll by 10 in whichever pane is focused
	if key.Matches(msg, m.keys.PageDown) {
		if !m.focusLeft {
			m.detailVP.LineDown(10)
			return m, nil
		}
		m.cursor += 10
		if m.cursor >= len(m.flatList) {
			m.cursor = len(m.flatList) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m.maybeLoadDetail()
	}
	if key.Matches(msg, m.keys.PageUp) {
		if !m.focusLeft {
			m.detailVP.LineUp(10)
			return m, nil
		}
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m.maybeLoadDetail()
	}

	// Detail pane gets viewport keys when focused
	if !m.focusLeft {
		var cmd tea.Cmd
		m.detailVP, cmd = m.detailVP.Update(msg)
		return m, cmd
	}

	// Tree navigation
	if key.Matches(msg, m.keys.Down) {
		if m.cursor < len(m.flatList)-1 {
			m.cursor++
			return m.maybeLoadDetail()
		}
		return m, nil
	}
	if key.Matches(msg, m.keys.Up) {
		if m.cursor > 0 {
			m.cursor--
			return m.maybeLoadDetail()
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Top) {
		m.cursor = 0
		return m.maybeLoadDetail()
	}
	if key.Matches(msg, m.keys.Bottom) {
		if len(m.flatList) > 0 {
			m.cursor = len(m.flatList) - 1
			return m.maybeLoadDetail()
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.HalfDown) {
		viewH := m.treeViewHeight()
		m.cursor += viewH / 2
		if m.cursor >= len(m.flatList) {
			m.cursor = len(m.flatList) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m.maybeLoadDetail()
	}
	if key.Matches(msg, m.keys.HalfUp) {
		viewH := m.treeViewHeight()
		m.cursor -= viewH / 2
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m.maybeLoadDetail()
	}

	// Expand / collapse
	if key.Matches(msg, m.keys.Right) || key.Matches(msg, m.keys.Toggle) || key.Matches(msg, m.keys.Enter) {
		return m.toggleExpand()
	}
	if key.Matches(msg, m.keys.Left) {
		return m.collapseOrParent()
	}

	if key.Matches(msg, m.keys.ExpandAll) {
		ExpandAll(m.roots)
		m.flatList = Flatten(m.roots)
		// Load unloaded nodes (best effort: load first-level only)
		var cmds []tea.Cmd
		for _, node := range m.flatList {
			if node.ChildCount > 0 && !node.Loaded {
				cmds = append(cmds, m.loadChildren(node.ID))
			}
		}
		return m, tea.Batch(cmds...)
	}
	if key.Matches(msg, m.keys.CollapseAll) {
		CollapseAll(m.roots)
		m.flatList = Flatten(m.roots)
		m.cursor = 0
		return m.maybeLoadDetail()
	}

	if key.Matches(msg, m.keys.ToggleClosed) {
		m.includeClosed = !m.includeClosed
		m.loading = true
		m.loadingMsg = "Reloading..."
		m.roots = nil
		m.flatList = nil
		m.nodeMap = make(map[string]*TreeNode)
		m.cursor = 0
		m.detail = nil
		m.detailID = ""
		if m.boardMode {
			return m, tea.Batch(m.loadBoard(), m.loadTypes)
		}
		return m, m.loadTypes
	}

	if key.Matches(msg, m.keys.Refresh) {
		m.loading = true
		m.loadingMsg = "Refreshing..."
		m.roots = nil
		m.flatList = nil
		m.nodeMap = make(map[string]*TreeNode)
		m.cursor = 0
		m.detail = nil
		m.detailID = ""
		if m.boardMode {
			return m, tea.Batch(m.loadBoard(), m.refreshTypes())
		}
		if len(m.types) > 0 {
			return m, tea.Batch(m.refreshTypes(), m.loadRoots(m.types[m.activeType].Type))
		}
		return m, m.loadTypes
	}

	return m, nil
}

func (m BrowseModel) switchType(idx int) (tea.Model, tea.Cmd) {
	if idx == m.activeType && !m.boardMode {
		return m, nil
	}
	m.activeType = idx
	m.boardMode = false
	m.roots = nil
	m.flatList = nil
	m.nodeMap = make(map[string]*TreeNode)
	m.cursor = 0
	m.detail = nil
	m.detailID = ""
	m.loading = true
	m.loadingMsg = fmt.Sprintf("Loading %s...", m.types[idx].Type)
	return m, m.loadRoots(m.types[idx].Type)
}

func (m BrowseModel) switchToBoard() (tea.Model, tea.Cmd) {
	if m.boardMode {
		return m, nil
	}
	m.boardMode = true
	m.roots = nil
	m.flatList = nil
	m.nodeMap = make(map[string]*TreeNode)
	m.cursor = 0
	m.detail = nil
	m.detailID = ""
	m.loading = true
	m.loadingMsg = "Loading board..."
	return m, m.loadBoard()
}

func (m BrowseModel) toggleExpand() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.flatList) {
		return m, nil
	}
	node := m.flatList[m.cursor]

	if node.Expanded {
		// Collapse
		node.Expanded = false
		m.flatList = Flatten(m.roots)
		return m, nil
	}

	if node.ChildCount == 0 {
		return m, nil
	}

	if !node.Loaded {
		// Need to fetch children
		return m, m.loadChildren(node.ID)
	}

	// Expand (already loaded)
	node.Expanded = true
	m.flatList = Flatten(m.roots)
	return m, nil
}

func (m BrowseModel) collapseOrParent() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.flatList) {
		return m, nil
	}
	node := m.flatList[m.cursor]

	if node.Expanded {
		node.Expanded = false
		m.flatList = Flatten(m.roots)
		return m, nil
	}

	// Jump to parent
	if node.Parent != nil {
		for i, n := range m.flatList {
			if n == node.Parent {
				m.cursor = i
				return m.maybeLoadDetail()
			}
		}
	}
	return m, nil
}

// maybeLoadDetail updates detailID on the model and returns a load command.
// Returns the modified model so callers get the state change.
func (m BrowseModel) maybeLoadDetail() (BrowseModel, tea.Cmd) {
	if m.cursor >= len(m.flatList) {
		return m, nil
	}
	node := m.flatList[m.cursor]
	if node.IsGroup {
		m.detail = nil
		m.detailID = ""
		m.detailLoading = false
		return m, nil
	}
	if node.ID == m.detailID && m.detail != nil {
		return m, nil // Already loaded
	}
	m.detailID = node.ID
	m.detailLoading = true
	m.detail = nil
	return m, m.loadDetail(node.ID)
}

func addToMap(node *TreeNode, m map[string]*TreeNode) {
	m[node.ID] = node
	for _, child := range node.Children {
		addToMap(child, m)
	}
}

func (m BrowseModel) treeViewHeight() int {
	return m.height - 5 // tabs + status + borders
}

// -- View --

func (m BrowseModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var sections []string

	// Tab bar
	sections = append(sections, m.renderTabs())

	// Main content: banner + tree + detail split
	banner := m.renderBanner()
	treeContent := banner + m.renderTree()
	detailContent := m.renderDetailPane()

	contentHeight := m.height - 3 // tabs + status bar

	// Apply pane styles
	var treePane, detailPane string
	detailWidth := m.width - m.treeWidth - 3
	if detailWidth < 10 {
		detailWidth = 10
	}

	treeStyle := m.styles.DimPane.Width(m.treeWidth).Height(contentHeight)
	detailStyle := m.styles.DimPane.Width(detailWidth).Height(contentHeight)
	if m.focusLeft {
		treeStyle = m.styles.ActivePane.Width(m.treeWidth).Height(contentHeight)
	} else {
		detailStyle = m.styles.ActivePane.Width(detailWidth).Height(contentHeight)
	}

	treePane = treeStyle.Render(treeContent)
	detailPane = detailStyle.Render(detailContent)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, treePane, detailPane)
	sections = append(sections, mainContent)

	// Status bar
	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m BrowseModel) renderTabs() string {
	var tabs []string

	// Board tab (always first)
	boardLabel := "1:Board"
	if m.boardMode {
		tabs = append(tabs, m.styles.ActiveTab.Render(boardLabel))
	} else {
		tabs = append(tabs, m.styles.InactiveTab.Render(boardLabel))
	}

	for i, t := range m.types {
		label := fmt.Sprintf("%d:%s (%d)", i+2, t.Type, t.Count)
		if !m.boardMode && i == m.activeType {
			tabs = append(tabs, m.styles.ActiveTab.Render(label))
		} else {
			tabs = append(tabs, m.styles.InactiveTab.Render(label))
		}
	}
	return m.styles.TabBar.Render(lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...))
}

func (m BrowseModel) renderBanner() string {
	if m.boardMode {
		if m.boardResult == nil {
			return ""
		}
		boardStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(TypeColor("board"))
		name := boardStyle.Render("BOARD")

		var parts []string
		parts = append(parts, name)
		if m.boardResult.UnreadCount > 0 {
			parts = append(parts, m.styles.StatusOpen.Render(fmt.Sprintf("%d unread", m.boardResult.UnreadCount)))
		}
		if m.boardResult.MemoryCount > 0 {
			parts = append(parts, m.styles.Muted.Render(fmt.Sprintf("%d memories", m.boardResult.MemoryCount)))
		}
		return fmt.Sprintf(" %s\n\n", strings.Join(parts, "  "))
	}

	if len(m.types) == 0 || m.activeType >= len(m.types) {
		return ""
	}
	t := m.types[m.activeType]

	typeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(TypeColor(t.Type))

	name := typeStyle.Render(strings.ToUpper(t.Type))

	open := m.styles.StatusOpen.Render(fmt.Sprintf("%d open", t.OpenCount))
	closed := m.styles.StatusClosed.Render(fmt.Sprintf("%d closed", t.ClosedCount))

	return fmt.Sprintf(" %s  %s  %s\n\n", name, open, closed)
}

func (m BrowseModel) renderTree() string {
	if m.loading {
		return m.styles.Muted.Render(m.loadingMsg)
	}
	if m.errMsg != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: " + m.errMsg)
	}
	if len(m.flatList) == 0 {
		return m.styles.Muted.Render("  No shards found")
	}

	viewH := m.treeViewHeight()
	if viewH <= 0 {
		viewH = 10
	}

	// Scroll window
	start := 0
	if m.cursor >= viewH {
		start = m.cursor - viewH + 1
	}
	end := start + viewH
	if end > len(m.flatList) {
		end = len(m.flatList)
	}

	var lines []string
	for i := start; i < end; i++ {
		node := m.flatList[i]
		line := m.renderTreeLine(node, i == m.cursor)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m BrowseModel) renderTreeLine(node *TreeNode, isCursor bool) string {
	// Build indent + tree prefix
	indent := strings.Repeat("  ", node.Depth)

	// Expand/collapse icon
	var icon string
	if node.ChildCount > 0 {
		if node.Expanded {
			icon = m.styles.ExpandIcon.Render("▾ ")
		} else {
			icon = m.styles.ExpandIcon.Render("▸ ")
		}
	} else {
		icon = "  "
	}

	var line string
	if node.IsGroup {
		// Group node: bold title with count, no type icon
		maxTitleLen := m.treeWidth - 8 // icon + padding + count
		title := node.Title
		if len(title) > maxTitleLen && maxTitleLen > 3 {
			title = title[:maxTitleLen-3] + "..."
		}
		groupTitle := m.styles.GroupTitle.Render(title)
		count := m.styles.ChildCount.Render(fmt.Sprintf(" (%d)", node.ChildCount))
		line = fmt.Sprintf("%s%s%s%s", indent, icon, groupTitle, count)
	} else {
		// Regular shard node
		typeStyle := lipgloss.NewStyle().Foreground(TypeColor(node.Type))
		typeIcon := typeStyle.Render(TypeIcon(node.Type))

		maxTitleLen := m.treeWidth - (node.Depth*2 + 6) // icon + type + padding
		title := node.Title
		if len(title) > maxTitleLen && maxTitleLen > 3 {
			title = title[:maxTitleLen-3] + "..."
		}

		childHint := ""
		if node.ChildCount > 0 && !node.Expanded {
			childHint = m.styles.ChildCount.Render(fmt.Sprintf(" (%d)", node.ChildCount))
		}

		line = fmt.Sprintf("%s%s%s %s%s", indent, icon, typeIcon, title, childHint)
	}

	if isCursor {
		// Pad to full width so highlight covers the line
		padded := lipgloss.NewStyle().Width(m.treeWidth).Render(line)
		return m.styles.CursorLine.Render(padded)
	}
	return line
}

func (m BrowseModel) renderDetailPane() string {
	if m.detailLoading {
		return RenderLoading(m.detailID, m.styles)
	}
	if m.detail == nil {
		return RenderEmpty(m.styles)
	}

	return m.detailVP.View()
}

func (m BrowseModel) renderStatusBar() string {
	s := m.styles

	help := []struct{ key, desc string }{
		{"j/k", "nav"},
		{"h/l", "tree"},
		{"1", "board"},
		{"[/]", "type"},
		{"r", "refresh"},
		{"c", "closed"},
		{"tab", "pane"},
		{"q", "quit"},
	}

	var parts []string
	for _, h := range help {
		parts = append(parts, s.StatusKey.Render(h.key)+s.StatusBar.Render(":"+h.desc))
	}

	bar := strings.Join(parts, s.StatusBar.Render("  "))

	// Right-align cursor position + closed indicator + version
	var right []string
	if m.includeClosed {
		right = append(right, s.StatusKey.Render("CLOSED"))
	}
	if len(m.flatList) > 0 {
		right = append(right, fmt.Sprintf("%d/%d", m.cursor+1, len(m.flatList)))
	}
	right = append(right, s.StatusBar.Render("v0.2.0"))
	pos := strings.Join(right, s.StatusBar.Render("  "))

	gap := m.width - lipgloss.Width(bar) - lipgloss.Width(pos) - 2
	if gap < 0 {
		gap = 0
	}

	return s.StatusBar.Width(m.width).Render(
		bar + strings.Repeat(" ", gap) + pos,
	)
}
