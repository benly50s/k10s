package tui

import (
	"fmt"
	"time"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/k8s"
	"github.com/benly/k10s/internal/portforward"
	"github.com/benly/k10s/internal/profile"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// pfRefreshTickMsg is sent periodically to refresh port-forward status.
type pfRefreshTickMsg time.Time

// pfMgrPresetStartedMsg is sent when a preset port-forward finishes starting
type pfMgrPresetStartedMsg struct {
	Preset config.PortForwardPreset
	Handle *k8s.PortForwardHandle
	Err    error
}

// PortForwardManagerModel is the port-forward management screen.
type PortForwardManagerModel struct {
	profile    profile.Profile
	manager    *portforward.Manager
	cfg        *config.K10sConfig
	profileMap map[string]string // kubeconfigPath -> profileName for discovery
	entries    []portforward.Entry
	presets    []config.PortForwardPreset
	history    []config.PortForwardHistoryEntry
	cursor     int
	keys       KeyMap
	cancelled  bool
	wantsNew   bool
	statusMsg  string

	// Preset save mode
	saving    bool
	saveInput textinput.Model
	saveIdx   int // index of entry being saved as preset

	// Preset launch mode
	launching bool
	spinner   spinner.Model
}

// NewPortForwardManagerModel creates the PF management screen.
func NewPortForwardManagerModel(p profile.Profile, mgr *portforward.Manager, cfg *config.K10sConfig, profiles []profile.Profile) PortForwardManagerModel {
	// Build profile map for external process discovery
	profileMap := make(map[string]string, len(profiles))
	for _, prof := range profiles {
		profileMap[prof.FilePath] = prof.Name
	}
	mgr.DiscoverExternal(profileMap)
	si := textinput.New()
	si.Placeholder = "프리셋 이름..."
	si.CharLimit = 32

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleSelected

	var presets []config.PortForwardPreset
	if cfg != nil {
		presets = cfg.GetPresetsForProfile(p.Name)
	}

	var history []config.PortForwardHistoryEntry
	if cfg != nil {
		history = cfg.GetPFHistoryForProfile(p.Name)
	}

	return PortForwardManagerModel{
		profile:    p,
		manager:    mgr,
		cfg:        cfg,
		profileMap: profileMap,
		entries:    mgr.List(),
		presets:    presets,
		history:    history,
		keys:       DefaultKeyMap(),
		saveInput:  si,
		spinner:    sp,
	}
}

// pfRefreshInterval is how often we auto-check port-forward liveness.
const pfRefreshInterval = 5 * time.Second

// Init initializes the model and starts the auto-refresh ticker.
func (m PortForwardManagerModel) Init() tea.Cmd {
	return tea.Tick(pfRefreshInterval, func(t time.Time) tea.Msg {
		return pfRefreshTickMsg(t)
	})
}

// totalRows returns the number of navigable rows.
func (m PortForwardManagerModel) totalRows() int {
	return len(m.entries) + len(m.presets) + len(m.history) + 1 // entries + presets + history + "new"
}

// Update handles messages.
func (m PortForwardManagerModel) Update(msg tea.Msg) (PortForwardManagerModel, tea.Cmd) {
	// Handle auto-refresh tick
	if _, ok := msg.(pfRefreshTickMsg); ok {
		m.refreshEntries()
		return m, tea.Tick(pfRefreshInterval, func(t time.Time) tea.Msg {
			return pfRefreshTickMsg(t)
		})
	}

	// Handle spinner tick
	if tickMsg, ok := msg.(spinner.TickMsg); ok && m.launching {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(tickMsg)
		return m, cmd
	}

	// Handle preset launch result
	if startMsg, ok := msg.(pfMgrPresetStartedMsg); ok {
		m.launching = false
		if startMsg.Err != nil {
			m.statusMsg = fmt.Sprintf("프리셋 실패: %v", startMsg.Err)
			return m, nil
		}
		m.manager.Add(portforward.Entry{
			Profile:      m.profile.Name,
			Namespace:    startMsg.Preset.Namespace,
			ResourceType: startMsg.Preset.ResourceType,
			ResourceName: startMsg.Preset.ResourceName,
			LocalPort:    startMsg.Preset.LocalPort,
			RemotePort:   startMsg.Preset.RemotePort,
			Handle:       startMsg.Handle,
		})
		m.entries = m.manager.List()
		m.statusMsg = fmt.Sprintf("Started: %s/%s :%d (프리셋: %s)",
			startMsg.Preset.ResourceType, startMsg.Preset.ResourceName,
			startMsg.Preset.LocalPort, startMsg.Preset.Name)
		return m, nil
	}

	// Save mode: text input for preset name
	if m.saving {
		return m.updateSaveMode(msg)
	}

	if m.launching {
		return m, nil
	}

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
			if m.cursor < m.totalRows()-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.Enter):
			return m.handleEnter()

		case msg.String() == "n":
			m.wantsNew = true
			return m, nil

		case msg.String() == "s":
			// Save current entry as preset
			if m.cursor < len(m.entries) && m.cfg != nil {
				m.saving = true
				m.saveIdx = m.cursor
				m.saveInput.Focus()
				return m, textinput.Blink
			}

		case msg.String() == "r":
			removed := m.refreshEntries()
			if removed > 0 {
				m.statusMsg = fmt.Sprintf("%d개 끊어진 포트포워드 제거됨", removed)
			} else {
				m.statusMsg = "모든 포트포워드 정상"
			}
			return m, nil

		case msg.String() == "d", msg.String() == "delete":
			return m.handleDelete()

		case msg.String() == "D":
			return m.handleClearHistory()
		}
	}
	return m, nil
}

// refreshEntries runs Cleanup, re-discovers external processes, and updates the local entries list.
func (m *PortForwardManagerModel) refreshEntries() int {
	removed := m.manager.Cleanup()
	m.manager.DiscoverExternal(m.profileMap)
	m.entries = m.manager.List()
	if m.cursor >= m.totalRows() {
		m.cursor = max(0, m.totalRows()-1)
	}
	return removed
}

func (m PortForwardManagerModel) handleEnter() (PortForwardManagerModel, tea.Cmd) {
	presetsEnd := len(m.entries) + len(m.presets)
	historyEnd := presetsEnd + len(m.history)
	newRowIdx := historyEnd

	if m.cursor == newRowIdx {
		// "New" row
		m.wantsNew = true
		return m, nil
	}
	// Active entry — reconnect if dead
	if m.cursor < len(m.entries) {
		e := m.entries[m.cursor]
		if e.External {
			return m, nil // external entries are read-only
		}
		if e.Handle != nil && e.Handle.IsAlive() {
			return m, nil // already running, nothing to do
		}
		// Remove the dead entry and relaunch
		_ = m.manager.Remove(e.ID)
		m.entries = m.manager.List()
		preset := config.PortForwardPreset{
			Profile:      e.Profile,
			Namespace:    e.Namespace,
			ResourceType: e.ResourceType,
			ResourceName: e.ResourceName,
			LocalPort:    e.LocalPort,
			RemotePort:   e.RemotePort,
		}
		m.launching = true
		m.statusMsg = ""
		return m, tea.Batch(m.spinner.Tick, m.launchPreset(preset))
	}
	if m.cursor >= len(m.entries) && m.cursor < presetsEnd {
		// Preset row — launch it
		presetIdx := m.cursor - len(m.entries)
		preset := m.presets[presetIdx]
		m.launching = true
		m.statusMsg = ""
		return m, tea.Batch(m.spinner.Tick, m.launchPreset(preset))
	}
	if m.cursor >= presetsEnd && m.cursor < historyEnd {
		// History row — launch it
		histIdx := m.cursor - len(m.entries) - len(m.presets)
		h := m.history[histIdx]
		preset := config.PortForwardPreset{
			Profile:      h.Profile,
			Namespace:    h.Namespace,
			ResourceType: h.ResourceType,
			ResourceName: h.ResourceName,
			LocalPort:    h.LocalPort,
			RemotePort:   h.RemotePort,
		}
		m.launching = true
		m.statusMsg = ""
		return m, tea.Batch(m.spinner.Tick, m.launchPreset(preset))
	}
	return m, nil
}

func (m PortForwardManagerModel) handleDelete() (PortForwardManagerModel, tea.Cmd) {
	presetsEnd := len(m.entries) + len(m.presets)
	historyEnd := presetsEnd + len(m.history)

	if m.cursor < len(m.entries) {
		// Active entry
		entry := m.entries[m.cursor]
		if entry.External {
			m.statusMsg = "외부 세션 포트포워드는 해당 세션에서 종료해주세요"
			return m, nil
		}
		_ = m.manager.Remove(entry.ID)
		m.entries = m.manager.List()
		m.statusMsg = fmt.Sprintf("Stopped: %s/%s :%d", entry.ResourceType, entry.ResourceName, entry.LocalPort)
	} else if m.cursor >= len(m.entries) && m.cursor < presetsEnd {
		// Preset
		presetIdx := m.cursor - len(m.entries)
		preset := m.presets[presetIdx]
		m.cfg.RemovePreset(preset.Name)
		_ = config.Save(m.cfg)
		m.presets = m.cfg.GetPresetsForProfile(m.profile.Name)
		m.statusMsg = fmt.Sprintf("프리셋 삭제: %s", preset.Name)
	} else if m.cursor >= presetsEnd && m.cursor < historyEnd {
		// History entry
		histIdx := m.cursor - presetsEnd
		h := m.history[histIdx]
		m.cfg.RemovePFHistory(config.PortForwardHistoryEntry{
			Profile:      h.Profile,
			Namespace:    h.Namespace,
			ResourceType: h.ResourceType,
			ResourceName: h.ResourceName,
			LocalPort:    h.LocalPort,
			RemotePort:   h.RemotePort,
		})
		_ = config.Save(m.cfg)
		m.history = m.cfg.GetPFHistoryForProfile(m.profile.Name)
		m.statusMsg = fmt.Sprintf("히스토리 삭제: %s/%s :%d", h.ResourceType, h.ResourceName, h.LocalPort)
	}
	if m.cursor >= m.totalRows() {
		m.cursor = max(0, m.totalRows()-1)
	}
	return m, nil
}

func (m PortForwardManagerModel) handleClearHistory() (PortForwardManagerModel, tea.Cmd) {
	if m.cfg == nil || len(m.history) == 0 {
		return m, nil
	}
	m.cfg.ClearPFHistoryForProfile(m.profile.Name)
	m.cfg.ClearPodLogNSHistoryForProfile(m.profile.Name)
	_ = config.Save(m.cfg)
	m.history = nil
	m.statusMsg = "히스토리 전체 삭제"
	if m.cursor >= m.totalRows() {
		m.cursor = max(0, m.totalRows()-1)
	}
	return m, nil
}

func (m PortForwardManagerModel) updateSaveMode(msg tea.Msg) (PortForwardManagerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.saving = false
			m.saveInput.SetValue("")
			m.saveInput.Blur()
			return m, nil
		case "enter":
			name := m.saveInput.Value()
			if name != "" && m.saveIdx < len(m.entries) {
				e := m.entries[m.saveIdx]
				m.cfg.AddPreset(config.PortForwardPreset{
					Name:         name,
					Profile:      m.profile.Name,
					Namespace:    e.Namespace,
					ResourceType: e.ResourceType,
					ResourceName: e.ResourceName,
					LocalPort:    e.LocalPort,
					RemotePort:   e.RemotePort,
				})
				_ = config.Save(m.cfg)
				m.presets = m.cfg.GetPresetsForProfile(m.profile.Name)
				m.statusMsg = fmt.Sprintf("프리셋 저장: %s", name)
			}
			m.saving = false
			m.saveInput.SetValue("")
			m.saveInput.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

func (m PortForwardManagerModel) launchPreset(preset config.PortForwardPreset) tea.Cmd {
	p := m.profile
	return func() tea.Msg {
		handle, err := k8s.StartPortForward(
			p.FilePath, p.Context,
			preset.Namespace, preset.ResourceType, preset.ResourceName,
			preset.LocalPort, preset.RemotePort,
		)
		return pfMgrPresetStartedMsg{Preset: preset, Handle: handle, Err: err}
	}
}

// View renders the port-forward manager screen.
func (m PortForwardManagerModel) View() string {
	title := StyleTitle.Render(fmt.Sprintf("k10s - %s  ›  Port Forward", m.profile.Name))

	if m.launching {
		return title + "\n\n  " + m.spinner.View() + " 프리셋 포트포워드 시작 중...\n"
	}

	if m.saving {
		content := "\n"
		e := m.entries[m.saveIdx]
		content += StyleNormal.Render(fmt.Sprintf("  프리셋으로 저장: %s/%s %d→%d",
			e.ResourceType, e.ResourceName, e.LocalPort, e.RemotePort)) + "\n\n"
		content += "  이름: " + m.saveInput.View() + "\n\n"
		help := StyleHelp.Render("  [enter] 저장   [esc] 취소")
		return title + "\n" + content + help
	}

	content := "\n"
	rowIdx := 0

	// Active port-forwards section
	if len(m.entries) == 0 {
		content += StyleDimmed.Render("  활성 포트포워드 없음") + "\n\n"
	} else {
		content += StyleNormal.Render("  Active Port-Forwards:") + "\n\n"
		for _, e := range m.entries {
			status := "●"
			if e.Handle == nil || !e.Handle.IsAlive() {
				status = "○"
			}
			extTag := ""
			if e.External {
				extTag = " [ext]"
			}
			line := fmt.Sprintf("%s %s/%s  localhost:%d → %d  (%s)%s",
				status, e.ResourceType, e.ResourceName,
				e.LocalPort, e.RemotePort, e.Namespace, extTag)

			if rowIdx == m.cursor {
				content += "  " + StyleSelected.Render("> "+line) + "\n"
			} else {
				content += "  " + StyleNormal.Render("  "+line) + "\n"
			}
			rowIdx++
		}
		content += "\n"
	}

	// Presets section
	if len(m.presets) > 0 {
		content += StyleNormal.Render("  Presets:") + "\n\n"
		for _, p := range m.presets {
			line := fmt.Sprintf("[%s]  %s/%s  %d→%d  (%s)",
				p.Name, p.ResourceType, p.ResourceName,
				p.LocalPort, p.RemotePort, p.Namespace)

			if rowIdx == m.cursor {
				content += "  " + StyleSelected.Render("> "+line) + "\n"
			} else {
				content += "  " + StyleDimmed.Render("  "+line) + "\n"
			}
			rowIdx++
		}
		content += "\n"
	}

	// History section
	if len(m.history) > 0 {
		content += StyleNormal.Render("  History:") + "\n\n"
		for _, h := range m.history {
			line := fmt.Sprintf("%s/%s  %d→%d  (%s)",
				h.ResourceType, h.ResourceName,
				h.LocalPort, h.RemotePort, h.Namespace)
			if rowIdx == m.cursor {
				content += "  " + StyleSelected.Render("> "+line) + "\n"
			} else {
				content += "  " + StyleDimmed.Render("  "+line) + "\n"
			}
			rowIdx++
		}
		content += "\n"
	}

	// "New port-forward" row
	newLabel := "[+] 새 포트포워드"
	if rowIdx == m.cursor {
		content += "  " + StyleSelected.Render("> "+newLabel) + "\n"
	} else {
		content += "  " + StyleNormal.Render("  "+newLabel) + "\n"
	}

	content += "\n"

	if m.statusMsg != "" {
		content += StyleWarning.Render("  "+m.statusMsg) + "\n\n"
	}

	help := StyleHelp.Render("  [↑↓] move   [enter] 실행/재연결   [n] 새로 만들기   [s] 프리셋 저장   [d] 중지/삭제   [D] 히스토리 전체삭제   [r] 새로고침   [←/esc] back   [q] quit")
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
