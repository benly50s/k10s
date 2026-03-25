package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary   = lipgloss.Color("#06B6D4") // Cyan-500
	colorSecondary = lipgloss.Color("#0891B2") // Cyan-600
	colorMuted     = lipgloss.Color("#6B7280")
	colorSuccess   = lipgloss.Color("#10B981")
	colorWarning   = lipgloss.Color("#F59E0B")
	colorOIDC      = lipgloss.Color("#3B82F6")
	colorSelected  = lipgloss.Color("#FFFFFF")
	colorNormal    = lipgloss.Color("#D1D5DB")
	colorProd      = lipgloss.Color("#EF4444") // Red
	colorNonProd   = lipgloss.Color("#10B981") // Green

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

	// 단축키 강조용 (보라색, 볼드)
	StyleHelpKey = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// 설명 텍스트 (기존 muted 유지)
	StyleHelpDesc = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess)

	StyleWarning = lipgloss.NewStyle().
			Foreground(colorWarning)

	StyleProdBadge = lipgloss.NewStyle().
			Foreground(colorProd).
			Bold(true)

	StyleNonProdBadge = lipgloss.NewStyle().
			Foreground(colorNonProd).
			Bold(true)

	StyleServerURL = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleFavBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24")).
			Bold(true)
)
