package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otherjamesbrown/context-palace/cp/internal/client"
)

// Fixed tab indices
const (
	tabBoard     = 0
	tabWork      = 1
	tabKnowledge = 2
	tabMemories  = 3
	tabMessages  = 4
	tabCount     = 5
)

var tabNames = [tabCount]string{"Board", "Work", "Knowledge", "Memories", "Messages"}

// BrowseModel is the top-level Bubble Tea model
type BrowseModel struct {
	client        *client.Client
	includeClosed bool
	keys          KeyMap
	styles        Styles

	// Tab
	activeTab int

	// Tree state
	roots    []*TreeNode
	flatList []*TreeNode
	cursor   int
	nodeMap  map[string]*TreeNode // id -> node for quick lookup

	// Board tab
	boardResult *client.BoardResult

	// Detail pane
	detail        *client.ShardDetailResult
	detailRaw     string // raw output from cxp shard show
	detailID      string // ID of shard being displayed or loading
	detailLoading bool
	detailVP      viewport.Model

	// Search mode
	searchMode    bool
	searchInput   textinput.Model
	searchResult  *client.ShardContextResult
	searchLoading bool
	searchErr     string
	searchTarget  int // cursor index of target in search results

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
		activeTab:     tabBoard,
	}
}

// -- Messages --

type rootsLoadedMsg struct {
	tab   int
	nodes []client.ShardTreeNode
}

type workItemsLoadedMsg struct {
	results []client.ShardListResult
}

type kbTreeLoadedMsg struct {
	nodes []client.KBTreeNode
}

type childrenLoadedMsg struct {
	parentID string
	nodes    []client.ShardTreeNode
}

type detailLoadedMsg struct {
	id     string
	detail *client.ShardDetailResult
	raw    string // raw output from cxp shard show
}

type boardLoadedMsg struct {
	result *client.BoardResult
}

type searchLoadedMsg struct {
	result *client.ShardContextResult
	raw    string // cxp shard show output for the target
}

type messagesLoadedMsg struct {
	results []client.ShardListResult
}

type errMsg struct {
	err error
}

// -- Commands --

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

func (m BrowseModel) loadWorkItems() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		statuses := []string{"in_progress", "ready", "open", "needs-review"}
		if m.includeClosed {
			statuses = append(statuses, "closed", "deferred")
		}
		results, err := m.client.ListShardsFiltered(ctx, client.ListShardsOpts{
			Types:  []string{"task", "bug", "design"},
			Status: statuses,
			Limit:  200,
		})
		if err != nil {
			return errMsg{err}
		}
		return workItemsLoadedMsg{results: results}
	}
}

func (m BrowseModel) loadKBTree() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		rootID := ""
		if m.client.Config.KnowledgeBase != nil && m.client.Config.KnowledgeBase.Root != "" {
			rootID = m.client.Config.KnowledgeBase.Root
		}
		if rootID == "" {
			return errMsg{fmt.Errorf("knowledge_base.root not configured in ~/.cp/config.yaml")}
		}
		nodes, err := m.client.KBTree(ctx, rootID, 10, m.includeClosed)
		if err != nil {
			return errMsg{err}
		}
		return kbTreeLoadedMsg{nodes: nodes}
	}
}

func (m BrowseModel) loadRoots(tab int, shardType string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		nodes, err := m.client.GetShardTreeRoots(ctx, shardType, m.includeClosed)
		if err != nil {
			return errMsg{err}
		}
		return rootsLoadedMsg{tab: tab, nodes: nodes}
	}
}

func (m BrowseModel) loadMessages() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results, err := m.client.ListShardsFiltered(ctx, client.ListShardsOpts{
			Types: []string{"message", "handoff"},
			Limit: 200,
		})
		if err != nil {
			return errMsg{err}
		}
		return messagesLoadedMsg{results: results}
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

func (m BrowseModel) loadSearch(id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := m.client.GetShardContext(ctx, id)
		if err != nil {
			return searchLoadedMsg{} // will show error via empty result
		}
		// Get raw detail output
		out, _ := exec.Command("cxp", "shard", "show", id).Output()
		return searchLoadedMsg{result: result, raw: string(out)}
	}
}

func (m BrowseModel) loadDetail(id string) tea.Cmd {
	return func() tea.Msg {
		// Use cxp shard show for rendering
		out, err := exec.Command("cxp", "shard", "show", id).Output()
		if err != nil {
			return errMsg{fmt.Errorf("cxp shard show %s: %w", id, err)}
		}
		// Still load structured detail for metadata (type, status, etc.)
		ctx := context.Background()
		detail, _ := m.client.GetShardDetail(ctx, id)
		return detailLoadedMsg{id: id, detail: detail, raw: string(out)}
	}
}

// -- Init --

func (m BrowseModel) Init() tea.Cmd {
	return m.loadBoard()
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
		if m.detailRaw != "" {
			m.setDetailContent()
		}
		return m, nil

	case boardLoadedMsg:
		m.boardResult = msg.result
		if m.activeTab != tabBoard {
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

	case workItemsLoadedMsg:
		if m.activeTab != tabWork {
			return m, nil
		}
		m.loading = false
		// Convert ShardListResult to TreeNodes and group by status
		var nodes []*TreeNode
		for _, r := range msg.results {
			nodes = append(nodes, &TreeNode{
				ID:        r.ID,
				Title:     r.Title,
				Type:      r.Type,
				Status:    r.Status,
				Labels:    r.Labels,
				CreatedAt: r.CreatedAt,
			})
		}
		m.roots = GroupByStatus(nodes)
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

	case kbTreeLoadedMsg:
		if m.activeTab != tabKnowledge {
			return m, nil
		}
		m.loading = false
		m.roots = BuildKBTreeNodes(msg.nodes)
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

	case rootsLoadedMsg:
		if m.activeTab != msg.tab {
			return m, nil
		}
		m.loading = false
		m.roots = BuildRoots(msg.nodes)
		// Apply tab-specific grouping
		switch m.activeTab {
		case tabMemories:
			m.roots = GroupByLabel(m.roots)
		}
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

	case messagesLoadedMsg:
		if m.activeTab != tabMessages {
			return m, nil
		}
		m.loading = false
		// Convert ShardListResult to TreeNodes and group by date
		var nodes []*TreeNode
		for _, r := range msg.results {
			nodes = append(nodes, &TreeNode{
				ID:        r.ID,
				Title:     r.Title,
				Type:      r.Type,
				Status:    r.Status,
				Labels:    r.Labels,
				CreatedAt: r.CreatedAt,
			})
		}
		m.roots = GroupByDate(nodes)
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
		m.detailRaw = msg.raw
		m.detailLoading = false
		m.setDetailContent()
		m.detailVP.GotoTop()
		return m, nil

	case searchLoadedMsg:
		m.searchLoading = false
		if msg.result == nil {
			m.searchErr = "Shard not found"
			return m, nil
		}
		m.searchResult = msg.result
		m.searchErr = ""
		// Build context tree
		roots, targetIdx := BuildSearchTree(msg.result)
		m.roots = roots
		m.nodeMap = make(map[string]*TreeNode)
		for _, root := range m.roots {
			addToMap(root, m.nodeMap)
		}
		m.flatList = Flatten(m.roots)
		m.cursor = targetIdx
		m.searchTarget = targetIdx
		// Load detail for target
		m.detailID = msg.result.Target.ID
		m.detailLoading = false
		m.detailRaw = msg.raw
		m.setDetailContent()
		m.detailVP.GotoTop()
		return m, nil

	case errMsg:
		m.loading = false
		m.errMsg = msg.err.Error()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to textinput if in search input mode (for cursor blink etc)
	if m.searchMode && m.searchInput.Focused() {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
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
	// Quit always works (but not when typing in search input)
	if key.Matches(msg, m.keys.Quit) && !(m.searchMode && m.searchInput.Focused()) {
		return m, tea.Quit
	}

	// Search mode: text input captures all keys
	if m.searchMode && m.searchInput.Focused() {
		return m.handleSearchInput(msg)
	}

	// Esc exits search results back to previous view
	if m.searchResult != nil && msg.Type == tea.KeyEscape {
		return m.exitSearch()
	}

	// Tab switches pane focus
	if key.Matches(msg, m.keys.Tab) {
		m.focusLeft = !m.focusLeft
		return m, nil
	}

	// Search key enters search mode
	if key.Matches(msg, m.keys.Search) && m.focusLeft {
		return m.enterSearch()
	}

	// Tab switching: number keys 1-5
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if r >= '1' && r <= '5' {
			return m.switchTab(int(r - '1'))
		}
	}

	// Tab cycling: [ and ]
	if key.Matches(msg, m.keys.PrevType) {
		prev := (m.activeTab - 1 + tabCount) % tabCount
		return m.switchTab(prev)
	}
	if key.Matches(msg, m.keys.NextType) {
		next := (m.activeTab + 1) % tabCount
		return m.switchTab(next)
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
		return m.reloadCurrentTab()
	}

	if key.Matches(msg, m.keys.Refresh) {
		return m.reloadCurrentTab()
	}

	return m, nil
}

func (m BrowseModel) switchTab(tab int) (tea.Model, tea.Cmd) {
	if tab == m.activeTab && m.searchResult == nil {
		return m, nil
	}
	m.activeTab = tab
	m.searchMode = false
	m.searchResult = nil
	m.searchErr = ""
	m.roots = nil
	m.flatList = nil
	m.nodeMap = make(map[string]*TreeNode)
	m.cursor = 0
	m.detail = nil
	m.detailID = ""
	m.loading = true
	m.loadingMsg = fmt.Sprintf("Loading %s...", tabNames[tab])
	return m, m.loadTabCmd(tab)
}

func (m BrowseModel) loadTabCmd(tab int) tea.Cmd {
	switch tab {
	case tabBoard:
		return m.loadBoard()
	case tabWork:
		return m.loadWorkItems()
	case tabKnowledge:
		return m.loadKBTree()
	case tabMemories:
		return m.loadRoots(tabMemories, "memory")
	case tabMessages:
		return m.loadMessages()
	default:
		return nil
	}
}

func (m BrowseModel) reloadCurrentTab() (tea.Model, tea.Cmd) {
	m.loading = true
	m.loadingMsg = "Reloading..."
	m.roots = nil
	m.flatList = nil
	m.nodeMap = make(map[string]*TreeNode)
	m.cursor = 0
	m.detail = nil
	m.detailID = ""
	return m, m.loadTabCmd(m.activeTab)
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

func (m BrowseModel) enterSearch() (tea.Model, tea.Cmd) {
	m.searchMode = true
	m.searchErr = ""
	m.searchResult = nil
	ti := textinput.New()
	ti.Placeholder = "shard ID (e.g. pf-6f32c7)"
	ti.CharLimit = 40
	ti.Width = m.treeWidth - 4
	ti.Focus()
	m.searchInput = ti
	return m, textinput.Blink
}

func (m BrowseModel) exitSearch() (tea.Model, tea.Cmd) {
	m.searchMode = false
	m.searchResult = nil
	m.searchErr = ""
	m.searchInput.Blur()
	// Restore previous view
	m.roots = nil
	m.flatList = nil
	m.nodeMap = make(map[string]*TreeNode)
	m.cursor = 0
	m.detail = nil
	m.detailID = ""
	m.detailRaw = ""
	m.loading = true
	m.loadingMsg = "Reloading..."
	return m, m.loadTabCmd(m.activeTab)
}

func (m BrowseModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		// Cancel search, return to previous view
		m.searchMode = false
		m.searchInput.Blur()
		m.searchErr = ""
		return m, nil
	case tea.KeyEnter:
		// Execute search
		id := strings.TrimSpace(m.searchInput.Value())
		if id == "" {
			return m, nil
		}
		m.searchInput.Blur()
		m.searchLoading = true
		m.searchErr = ""
		return m, m.loadSearch(id)
	}
	// Forward to textinput
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
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

	contentHeight := m.height - 4 // tabs + status bar + pane borders

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

	for i, name := range tabNames {
		label := fmt.Sprintf("%d:%s", i+1, name)
		if i == m.activeTab && m.searchResult == nil {
			tabs = append(tabs, m.styles.ActiveTab.Render(label))
		} else {
			tabs = append(tabs, m.styles.InactiveTab.Render(label))
		}
	}

	// Search indicator
	if m.searchMode || m.searchResult != nil {
		tabs = append(tabs, m.styles.ActiveTab.Render("Search"))
	}

	return m.styles.TabBar.Width(m.width).Render(lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...))
}

func (m BrowseModel) renderBanner() string {
	// Search mode: show input or result header
	if m.searchMode || m.searchResult != nil {
		var b strings.Builder
		if m.searchInput.Focused() {
			// Show text input
			b.WriteString(" ")
			b.WriteString(m.styles.SearchInput.Render(m.searchInput.View()))
			b.WriteString("\n\n")
			if m.searchLoading {
				b.WriteString(m.styles.Muted.Render("  Searching..."))
				b.WriteString("\n")
			}
			if m.searchErr != "" {
				b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("  " + m.searchErr))
				b.WriteString("\n")
			}
		} else if m.searchResult != nil {
			searchStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
			b.WriteString(fmt.Sprintf(" %s  %s\n\n",
				searchStyle.Render("SEARCH"),
				m.styles.Muted.Render(m.searchResult.Target.ID)))
		}
		return b.String()
	}

	switch m.activeTab {
	case tabBoard:
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

	case tabWork:
		workStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		return fmt.Sprintf(" %s  %s\n\n",
			workStyle.Render("WORK"),
			m.styles.Muted.Render("tasks + bugs + designs"))

	case tabKnowledge:
		kbStyle := lipgloss.NewStyle().Bold(true).Foreground(TypeColor("knowledge"))
		return fmt.Sprintf(" %s\n\n", kbStyle.Render("KNOWLEDGE"))

	case tabMemories:
		memStyle := lipgloss.NewStyle().Bold(true).Foreground(TypeColor("memory"))
		return fmt.Sprintf(" %s\n\n", memStyle.Render("MEMORIES"))

	case tabMessages:
		msgStyle := lipgloss.NewStyle().Bold(true).Foreground(TypeColor("message"))
		return fmt.Sprintf(" %s  %s\n\n",
			msgStyle.Render("MESSAGES"),
			m.styles.Muted.Render("messages + handoffs"))
	}

	return ""
}

func (m BrowseModel) renderTree() string {
	if m.searchMode && m.searchInput.Focused() {
		if m.searchLoading {
			return ""
		}
		if m.searchErr != "" {
			return ""
		}
		// Input is showing in banner; tree below is empty until results
		return ""
	}
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
		// Pick style based on group ID
		groupStyle := m.styles.GroupTitle
		switch {
		case node.ID == "group:board:needs-review" || node.ID == "group:status:needs-review":
			groupStyle = m.styles.GroupNeedsReview
		case node.ID == "group:board:blocked":
			groupStyle = m.styles.GroupBlocked
		}
		groupTitle := groupStyle.Render(title)
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

		// Highlight search target
		if m.searchResult != nil && node.ID == m.searchResult.Target.ID {
			title = m.styles.SearchHighlight.Render(title)
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

// setDetailContent wraps raw output to the detail pane width and sets it on the viewport.
func (m *BrowseModel) setDetailContent() {
	w := m.width - m.treeWidth - 5 // border chars + padding
	if w < 20 {
		w = 20
	}
	m.detailVP.SetContent(wordWrap(m.detailRaw, w))
}

func (m BrowseModel) renderDetailPane() string {
	if m.detailLoading {
		return RenderLoading(m.detailID, m.styles)
	}
	if m.detailRaw == "" {
		return RenderEmpty(m.styles)
	}

	return m.detailVP.View()
}

func (m BrowseModel) renderStatusBar() string {
	s := m.styles

	var help []struct{ key, desc string }
	if m.searchMode && m.searchInput.Focused() {
		help = []struct{ key, desc string }{
			{"enter", "search"},
			{"esc", "cancel"},
		}
	} else if m.searchResult != nil {
		help = []struct{ key, desc string }{
			{"j/k", "nav"},
			{"h/l", "tree"},
			{"tab", "pane"},
			{"esc", "back"},
			{"s", "new search"},
		}
	} else {
		help = []struct{ key, desc string }{
			{"j/k", "nav"},
			{"h/l", "tree"},
			{"1-5", "tabs"},
			{"[/]", "cycle"},
			{"s", "search"},
			{"r", "refresh"},
			{"c", "closed"},
			{"tab", "pane"},
			{"q", "quit"},
		}
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
	right = append(right, s.StatusBar.Render("v0.3.0"))
	pos := strings.Join(right, s.StatusBar.Render("  "))

	gap := m.width - lipgloss.Width(bar) - lipgloss.Width(pos) - 2
	if gap < 0 {
		gap = 0
	}

	return s.StatusBar.Width(m.width).Render(
		bar + strings.Repeat(" ", gap) + pos,
	)
}
