package tui

import (
	"fmt"
	"os"

	"github.com/benly/k10s/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

// AppState represents the current state of the TUI state machine
type AppState int

const (
	StateClusterSelect AppState = iota
	StateActionSelect
	StateNamespaceSelect
	StateExecuting
	StateDeleteConfirm
	StateExit
	StateError
)

// ExecuteMsg is sent when the user has chosen an action and we need to exit TUI
type ExecuteMsg struct {
	Profile   profile.Profile
	Action    Action
	Namespace string // empty = all namespaces
}

// AppModel is the top-level bubbletea model
type AppModel struct {
	state           AppState
	clusterList     ClusterListModel
	actionMenu      ActionMenuModel
	namespaceSelect NamespaceSelectModel
	profiles        []profile.Profile
	targetProfile   *profile.Profile
	result          *ExecuteMsg
	err             error
}

// NewAppModel creates the top-level application model
func NewAppModel(profiles []profile.Profile) AppModel {
	return AppModel{
		state:       StateClusterSelect,
		clusterList: NewClusterListModel(profiles),
		profiles:    profiles,
	}
}

// Init initializes the app model
func (m AppModel) Init() tea.Cmd {
	return m.clusterList.Init()
}

// Update handles messages and state transitions
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateClusterSelect:
		return m.updateClusterSelect(msg)
	case StateActionSelect:
		return m.updateActionSelect(msg)
	case StateNamespaceSelect:
		return m.updateNamespaceSelect(msg)
	case StateDeleteConfirm:
		return m.updateDeleteConfirm(msg)
	case StateExecuting, StateExit, StateError:
		return m, tea.Quit
	}
	return m, nil
}

func (m AppModel) updateClusterSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if deleteMsg, ok := msg.(DeletePromptMsg); ok {
		m.state = StateDeleteConfirm
		m.targetProfile = &deleteMsg.Profile
		return m, nil
	}

	var cmd tea.Cmd
	m.clusterList, cmd = m.clusterList.Update(msg)

	if m.clusterList.Quitting() {
		m.state = StateExit
		return m, tea.Quit
	}

	if selected := m.clusterList.Selected(); selected != nil {
		// Check default_action
		switch selected.DefaultAction {
		case "k9s":
			// Fast-path: skip action menu but still pick a namespace
			m.state = StateNamespaceSelect
			m.namespaceSelect = NewNamespaceSelectModel(*selected)
			return m, m.namespaceSelect.Init()
		default: // "select" or anything else
			m.state = StateActionSelect
			m.actionMenu = NewActionMenuModel(*selected)
			return m, m.actionMenu.Init()
		}
	}

	return m, cmd
}

func (m AppModel) updateActionSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.actionMenu, cmd = m.actionMenu.Update(msg)

	if m.actionMenu.Cancelled() {
		// Go back to cluster selection
		m.state = StateClusterSelect
		// Reset the selected item in cluster list
		m.clusterList = NewClusterListModel(m.profiles)
		return m, m.clusterList.Init()
	}

	if action := m.actionMenu.Selected(); action != ActionNone {
		if action == ActionK9s {
			// Go through namespace selection before executing
			m.state = StateNamespaceSelect
			m.namespaceSelect = NewNamespaceSelectModel(m.actionMenu.profile)
			return m, m.namespaceSelect.Init()
		}
		m.result = &ExecuteMsg{
			Profile: m.actionMenu.profile,
			Action:  action,
		}
		m.state = StateExit
		return m, tea.Quit
	}

	return m, cmd
}

func (m AppModel) updateNamespaceSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.namespaceSelect, cmd = m.namespaceSelect.Update(msg)

	if m.namespaceSelect.Cancelled() {
		// Go back to action menu (or cluster select if fast-path)
		if m.actionMenu.profile.Name != "" {
			m.state = StateActionSelect
		} else {
			m.state = StateClusterSelect
			m.clusterList = NewClusterListModel(m.profiles)
			return m, m.clusterList.Init()
		}
		return m, nil
	}

	if ns, done := m.namespaceSelect.Selected(); done {
		m.result = &ExecuteMsg{
			Profile:   m.namespaceSelect.profile,
			Action:    ActionK9s,
			Namespace: ns,
		}
		m.state = StateExit
		return m, tea.Quit
	}

	return m, cmd
}

func (m AppModel) updateDeleteConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			// Delete file
			if m.targetProfile != nil {
				if err := os.Remove(m.targetProfile.FilePath); err != nil {
					m.err = fmt.Errorf("failed to delete file %s: %w", m.targetProfile.FilePath, err)
					m.state = StateError
					return m, tea.Quit
				}
			}

			// Reload profiles after deletion
			// Since we just delete it from disk, we can just remove it from our slice.
			var newProfiles []profile.Profile
			for _, p := range m.profiles {
				if p.FilePath != m.targetProfile.FilePath {
					newProfiles = append(newProfiles, p)
				}
			}
			m.profiles = newProfiles
			m.clusterList = NewClusterListModel(m.profiles)
			
			m.state = StateClusterSelect
			m.targetProfile = nil
			return m, m.clusterList.Init()

		case "n", "N", "esc", "enter", "ctrl+c":
			m.state = StateClusterSelect
			m.targetProfile = nil
			return m, nil
		}
	}
	return m, nil
}

// View renders the current state
func (m AppModel) View() string {
	switch m.state {
	case StateClusterSelect:
		return m.clusterList.View()
	case StateActionSelect:
		return m.actionMenu.View()
	case StateNamespaceSelect:
		return m.namespaceSelect.View()
	case StateDeleteConfirm:
		return StyleWarning.Render(fmt.Sprintf("\n  Are you sure you want to delete profile '%s'?\n  File: %s\n  (y/N)", m.targetProfile.Name, m.targetProfile.FilePath))
	case StateError:
		if m.err != nil {
			return StyleWarning.Render(fmt.Sprintf("Error: %v\n", m.err))
		}
		return ""
	default:
		return ""
	}
}

// Result returns the execution result, or nil if the user quit
func (m AppModel) Result() *ExecuteMsg {
	return m.result
}

// Run starts the TUI and returns the user's selection
func Run(profiles []profile.Profile) (*ExecuteMsg, error) {
	model := NewAppModel(profiles)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		errModel := AppModel{state: StateError, err: err}
		fmt.Fprintln(os.Stderr, errModel.View())
		return nil, err
	}
	appModel, ok := finalModel.(AppModel)
	if !ok {
		return nil, nil
	}
	return appModel.Result(), nil
}
