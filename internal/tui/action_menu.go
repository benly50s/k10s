package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/benly/k10s/internal/profile"
)

// Action represents an action the user can take on a cluster
type Action int

const (
	ActionNone Action = iota
	ActionK9s
	ActionArgoCD
	ActionPortForward
)

// actionOption is a menu item
type actionOption struct {
	action  Action
	label   string
	enabled bool
}

// ActionMenuModel is the action selection screen
type ActionMenuModel struct {
	profile   profile.Profile
	options   []actionOption
	cursor    int
	keys      KeyMap
	selected  Action
	cancelled bool
}

// NewActionMenuModel creates a new action menu for the given profile
func NewActionMenuModel(p profile.Profile) ActionMenuModel {
	hasArgocd := p.Argocd != nil

	options := []actionOption{
		{action: ActionK9s, label: "k9s 열기", enabled: true},
		{action: ActionArgoCD, label: "ArgoCD 접속 (포트포워딩 + 로그인 + 브라우저)", enabled: hasArgocd},
		{action: ActionPortForward, label: "포트포워딩만", enabled: hasArgocd},
	}

	return ActionMenuModel{
		profile:  p,
		options:  options,
		cursor:   0,
		keys:     DefaultKeyMap(),
		selected: ActionNone,
	}
}

// Init initializes the action menu
func (m ActionMenuModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m ActionMenuModel) Update(msg tea.Msg) (ActionMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			m.cancelled = true
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.Enter):
			opt := m.options[m.cursor]
			if opt.enabled {
				m.selected = opt.action
			}
		}
	}
	return m, nil
}

// View renders the action menu
func (m ActionMenuModel) View() string {
	title := StyleTitle.Render(fmt.Sprintf("k10s - %s", m.profile.Name))

	content := "\n  Select action:\n\n"

	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		var label string
		if !opt.enabled {
			label = StyleDimmed.Render(cursor + opt.label + " (no config)")
		} else if i == m.cursor {
			label = StyleSelected.Render(cursor + opt.label)
		} else {
			label = StyleNormal.Render(cursor + opt.label)
		}

		content += "  " + label + "\n"
	}

	content += "\n"
	help := StyleHelp.Render("  [←/esc] back   [↑↓] move   [enter] run   [q] quit")

	return title + "\n" + content + help
}

// Selected returns the chosen action (ActionNone if none selected)
func (m ActionMenuModel) Selected() Action {
	return m.selected
}

// Cancelled returns true if the user pressed back/esc
func (m ActionMenuModel) Cancelled() bool {
	return m.cancelled
}
