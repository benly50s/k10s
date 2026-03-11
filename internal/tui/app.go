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
	StateExecuting
	StateExit
	StateError
)

// ExecuteMsg is sent when the user has chosen an action and we need to exit TUI
type ExecuteMsg struct {
	Profile profile.Profile
	Action  Action
}

// AppModel is the top-level bubbletea model
type AppModel struct {
	state       AppState
	clusterList ClusterListModel
	actionMenu  ActionMenuModel
	profiles    []profile.Profile
	result      *ExecuteMsg
	err         error
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
	case StateExecuting, StateExit, StateError:
		return m, tea.Quit
	}
	return m, nil
}

func (m AppModel) updateClusterSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.result = &ExecuteMsg{Profile: *selected, Action: ActionK9s}
			m.state = StateExit
			return m, tea.Quit
		case "argocd":
			m.result = &ExecuteMsg{Profile: *selected, Action: ActionArgoCD}
			m.state = StateExit
			return m, tea.Quit
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
		m.result = &ExecuteMsg{
			Profile: m.actionMenu.profile,
			Action:  action,
		}
		m.state = StateExit
		return m, tea.Quit
	}

	return m, cmd
}

// View renders the current state
func (m AppModel) View() string {
	switch m.state {
	case StateClusterSelect:
		return m.clusterList.View()
	case StateActionSelect:
		return m.actionMenu.View()
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
