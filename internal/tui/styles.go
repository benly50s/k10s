package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary   = lipgloss.Color("#7C3AED")
	colorSecondary = lipgloss.Color("#6D28D9")
	colorMuted     = lipgloss.Color("#6B7280")
	colorSuccess   = lipgloss.Color("#10B981")
	colorWarning   = lipgloss.Color("#F59E0B")
	colorOIDC      = lipgloss.Color("#3B82F6")
	colorSelected  = lipgloss.Color("#FFFFFF")
	colorNormal    = lipgloss.Color("#D1D5DB")

	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 1)

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSecondary)

	StyleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSelected)

	StyleNormal = lipgloss.NewStyle().
			Foreground(colorNormal)

	StyleDimmed = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleOIDCBadge = lipgloss.NewStyle().
			Foreground(colorOIDC).
			Bold(true)

	StyleHelp = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess)

	StyleWarning = lipgloss.NewStyle().
			Foreground(colorWarning)

	StyleServerURL = lipgloss.NewStyle().
			Foreground(colorMuted)
)
