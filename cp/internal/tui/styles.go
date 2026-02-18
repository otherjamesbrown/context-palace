package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Styles holds all TUI styling
type Styles struct {
	// Tab bar
	ActiveTab   lipgloss.Style
	InactiveTab lipgloss.Style
	TabBar      lipgloss.Style

	// Panes
	ActivePane lipgloss.Style
	DimPane    lipgloss.Style

	// Tree
	CursorLine  lipgloss.Style
	NormalLine  lipgloss.Style
	TreePrefix  lipgloss.Style
	ExpandIcon  lipgloss.Style
	ShardID     lipgloss.Style
	ChildCount  lipgloss.Style
	GroupTitle   lipgloss.Style

	// Status indicators
	StatusOpen       lipgloss.Style
	StatusClosed     lipgloss.Style
	StatusInProgress lipgloss.Style

	// Detail pane
	DetailTitle    lipgloss.Style
	DetailLabel    lipgloss.Style
	DetailValue    lipgloss.Style
	DetailContent  lipgloss.Style
	ChildrenHeader lipgloss.Style
	ChildID        lipgloss.Style
	ChildTrigger   lipgloss.Style
	UsageHeader    lipgloss.Style

	// Status bar
	StatusBar   lipgloss.Style
	StatusKey   lipgloss.Style

	// Loading / empty
	Muted lipgloss.Style
}

// DefaultStyles returns the default style set
func DefaultStyles() Styles {
	return Styles{
		ActiveTab: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1),
		InactiveTab: lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Padding(0, 1),
		TabBar: lipgloss.NewStyle().
			MarginBottom(0),

		ActivePane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")),
		DimPane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")),

		CursorLine: lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("15")).
			Bold(true),
		NormalLine: lipgloss.NewStyle(),
		TreePrefix: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		ExpandIcon: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")),
		ShardID: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")),
		ChildCount: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		GroupTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("248")),

		StatusOpen: lipgloss.NewStyle().
			Foreground(lipgloss.Color("78")),
		StatusClosed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")),
		StatusInProgress: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")),

		DetailTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")),
		DetailLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")),
		DetailValue: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		DetailContent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		ChildrenHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("78")),
		ChildID: lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true),
		ChildTrigger: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true),
		UsageHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("141")),

		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("235")),
		StatusKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Background(lipgloss.Color("235")).
			Bold(true),

		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true),
	}
}

// TypeIcon returns a single-char icon for a shard type
func TypeIcon(shardType string) string {
	switch shardType {
	case "memory":
		return "M"
	case "task":
		return "T"
	case "knowledge":
		return "K"
	case "epic":
		return "E"
	case "bug":
		return "B"
	case "backlog":
		return "b"
	case "message":
		return "m"
	case "session":
		return "S"
	case "note":
		return "n"
	default:
		return "*"
	}
}

// TypeColor returns a color for a shard type
func TypeColor(shardType string) lipgloss.Color {
	switch shardType {
	case "memory":
		return lipgloss.Color("141") // purple
	case "task":
		return lipgloss.Color("214") // orange
	case "knowledge":
		return lipgloss.Color("78") // green
	case "epic":
		return lipgloss.Color("39") // blue
	case "bug":
		return lipgloss.Color("196") // red
	case "backlog":
		return lipgloss.Color("245") // gray
	case "message":
		return lipgloss.Color("220") // yellow
	case "session":
		return lipgloss.Color("117") // light blue
	case "board":
		return lipgloss.Color("44") // cyan
	default:
		return lipgloss.Color("252") // white
	}
}
