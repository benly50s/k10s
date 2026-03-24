package tui

import (
	"fmt"

	"github.com/benly/k10s/internal/portforward"
	"github.com/benly/k10s/internal/profile"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// PortForwardManagerModel is the port-forward management screen.
type PortForwardManagerModel struct {
	profile    profile.Profile
	manager    *portforward.Manager
	entries    []portforward.Entry
	cursor     int
	keys       KeyMap
	cancelled  bool
	wantsNew   bool
	statusMsg  string
}

// NewPortForwardManagerModel creates the PF management screen.
func NewPortForwardManagerModel(p profile.Profile, mgr *portforward.Manager) PortForwardManagerModel {
	return PortForwardManagerModel{
		profile: p,
		manager: mgr,
		entries: mgr.List(),
		keys:    DefaultKeyMap(),
	}
}

// Init initializes the model.
func (m PortForwardManagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m PortForwardManagerModel) Update(msg tea.Msg) (PortForwardManagerModel, tea.Cmd) {
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
			total := len(m.entries) + 1 // +1 for "new" row
			if m.cursor < total-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.Enter):
			if m.cursor == len(m.entries) {
				// "New port-forward" selected
				m.wantsNew = true
				return m, nil
			}

		case msg.String() == "n":
			m.wantsNew = true
			return m, nil

		case msg.String() == "d", msg.String() == "delete":
			if m.cursor < len(m.entries) {
				entry := m.entries[m.cursor]
				_ = m.manager.Remove(entry.ID)
				m.entries = m.manager.List()
				m.statusMsg = fmt.Sprintf("Stopped: %s/%s :%d", entry.ResourceType, entry.ResourceName, entry.LocalPort)
				// Keep cursor in bounds
				total := len(m.entries) + 1
				if m.cursor >= total {
					m.cursor = total - 1
				}
			}
		}
	}
	return m, nil
}

// View renders the port-forward manager screen.
func (m PortForwardManagerModel) View() string {
	title := StyleTitle.Render(fmt.Sprintf("k10s - %s  ›  Port Forward", m.profile.Name))

	content := "\n"

	if len(m.entries) == 0 {
		content += StyleDimmed.Render("  활성 포트포워드 없음") + "\n\n"
	} else {
		content += StyleNormal.Render("  Active Port-Forwards:") + "\n\n"
		for i, e := range m.entries {
			line := fmt.Sprintf("%s/%s  localhost:%d → %d  (%s)",
				e.ResourceType, e.ResourceName,
				e.LocalPort, e.RemotePort, e.Namespace)

			cursor := "  "
			if i == m.cursor {
				cursor = "> "
				content += "  " + StyleSelected.Render(cursor+line) + "\n"
			} else {
				content += "  " + StyleNormal.Render(cursor+line) + "\n"
			}
		}
		content += "\n"
	}

	// "New port-forward" row
	newLabel := "[+] 새 포트포워드"
	if m.cursor == len(m.entries) {
		content += "  " + StyleSelected.Render("> "+newLabel) + "\n"
	} else {
		content += "  " + StyleNormal.Render("  "+newLabel) + "\n"
	}

	content += "\n"

	if m.statusMsg != "" {
		content += StyleWarning.Render("  "+m.statusMsg) + "\n\n"
	}

	help := StyleHelp.Render("  [↑↓] move   [enter/n] 새로 만들기   [d] 중지/삭제   [←/esc] back   [q] quit")
	return title + "\n" + content + help
}

// Cancelled returns true if the user pressed back.
func (m PortForwardManagerModel) Cancelled() bool {
	return m.cancelled
}

// WantsCreate returns true if the user wants to create a new port-forward.
func (m PortForwardManagerModel) WantsCreate() bool {
	return m.wantsNew
}
