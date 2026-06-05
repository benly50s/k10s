package tui

import (
	"fmt"
	"strings"
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
	Preset  config.PortForwardPreset
	SetName string
	Handle  *k8s.PortForwardHandle
	Err     error
}

const (
	tabActive = iota
	tabSets
	tabPresets
	tabHistory
	tabCount
)

// PortForwardManagerModel is the port-forward management screen.
type PortForwardManagerModel struct {
	profile    profile.Profile
	manager    *portforward.Manager
	cfg        *config.K10sConfig
	profileMap map[string]string // kubeconfigPath -> profileName for discovery
	entries    []portforward.Entry
	presets    []config.PortForwardPreset
	history    []config.PortForwardHistoryEntry
	sets       []config.PortForwardSet
	activeTab  int
	cursors    [tabCount]int // cursors for each tab
	keys       KeyMap
	cancelled  bool
	wantsNew   bool
	statusMsg  string
	width      int
	height     int

	// Filter mode
	filter     textinput.Model
	filtering  bool
	// Preset save mode
	saving    bool
	savingSet bool
	saveInput textinput.Model
	saveIdx   int // index of entry being saved as preset

	// Preset launch mode
	launching bool
	spinner   spinner.Model

	// Auto-launch properties from router
	AutoLaunchSet    string
	AutoLaunchPreset string
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

	fi := textinput.New()
	fi.Placeholder = "filter..."
	fi.CharLimit = 64
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

	var sets []config.PortForwardSet
	if cfg != nil {
		sets = cfg.Global.PortForwardSets
	}

	return PortForwardManagerModel{
		profile:    p,
		manager:    mgr,
		cfg:        cfg,
		profileMap: profileMap,
		entries:    mgr.List(),
		presets:    presets,
		history:    history,
		sets:       sets,
		keys:       DefaultKeyMap(),
		saveInput:  si,
		spinner:    sp,
		filter:     fi,
		activeTab:  tabActive,
	}
}

// pfRefreshInterval is how often we auto-check port-forward liveness.
const pfRefreshInterval = 5 * time.Second

// Init initializes the model and starts the auto-refresh ticker.
func (m PortForwardManagerModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	if m.AutoLaunchSet != "" {
		for _, set := range m.sets {
			if set.Name == m.AutoLaunchSet {
				for _, item := range set.Forwards {
					if item.Profile == m.profile.Name {
						preset := config.PortForwardPreset{
							Profile:      item.Profile,
							Namespace:    item.Namespace,
							ResourceType: item.ResourceType,
							ResourceName: item.ResourceName,
							LocalPort:    item.LocalPort,
							RemotePort:   item.RemotePort,
						}
						cmds = append(cmds, m.launchPreset(preset, set.Name))
					}
				}
				break
			}
		}
		cmds = append(cmds, m.spinner.Tick)
	} else if m.AutoLaunchPreset != "" {
		for _, preset := range m.presets {
			if preset.Name == m.AutoLaunchPreset {
				cmds = append(cmds, m.launchPreset(preset, ""))
				cmds = append(cmds, m.spinner.Tick)
				break
			}
		}
	}

	if !k8s.DemoMode {
		cmds = append(cmds, tea.Tick(pfRefreshInterval, func(t time.Time) tea.Msg {
			return pfRefreshTickMsg(t)
		}))
	}

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// totalRows returns the number of navigable rows in the active tab.
func (m PortForwardManagerModel) totalRows() int {
	count := 0
	switch m.activeTab {
	case tabActive:
		count = len(m.filteredEntries())
	case tabSets:
		count = len(m.filteredSets())
	case tabPresets:
		count = len(m.filteredPresets())
	case tabHistory:
		count = len(m.filteredHistory())
	}
	return count + 1 // +1 for "[+] 새 포트포워드" row
}
// filtered list helpers
func (m PortForwardManagerModel) filteredSets() []config.PortForwardSet {
	if !m.filtering || strings.TrimSpace(m.filter.Value()) == "" {
		return m.sets
	}
	q := strings.ToLower(m.filter.Value())
	var out []config.PortForwardSet
	for _, s := range m.sets {
		if strings.Contains(strings.ToLower(s.Name), q) {
			out = append(out, s)
		}
	}
	return out
}

func (m PortForwardManagerModel) filteredEntries() []portforward.Entry {
	if !m.filtering || strings.TrimSpace(m.filter.Value()) == "" {
		return m.entries
	}
	q := strings.ToLower(m.filter.Value())
	var out []portforward.Entry
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.ResourceName), q) || strings.Contains(strings.ToLower(e.Namespace), q) {
			out = append(out, e)
		}
	}
	return out
}

func (m PortForwardManagerModel) filteredPresets() []config.PortForwardPreset {
	if !m.filtering || strings.TrimSpace(m.filter.Value()) == "" {
		return m.presets
	}
	q := strings.ToLower(m.filter.Value())
	var out []config.PortForwardPreset
	for _, p := range m.presets {
		if strings.Contains(strings.ToLower(p.ResourceName), q) || strings.Contains(strings.ToLower(p.Namespace), q) || strings.Contains(strings.ToLower(p.Name), q) {
			out = append(out, p)
		}
	}
	return out
}

func (m PortForwardManagerModel) filteredHistory() []config.PortForwardHistoryEntry {
	if !m.filtering || strings.TrimSpace(m.filter.Value()) == "" {
		return m.history
	}
	q := strings.ToLower(m.filter.Value())
	var out []config.PortForwardHistoryEntry
	for _, h := range m.history {
		if strings.Contains(strings.ToLower(h.ResourceName), q) || strings.Contains(strings.ToLower(h.Namespace), q) {
			out = append(out, h)
		}
	}
	return out
}

// Update handles messages.
func (m PortForwardManagerModel) Update(msg tea.Msg) (PortForwardManagerModel, tea.Cmd) {
	// Handle window resize
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	// Handle auto-refresh tick
	if _, ok := msg.(pfRefreshTickMsg); ok {
		if k8s.DemoMode {
			return m, nil
		}
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
			SetName:      startMsg.SetName,
		})
		m.entries = m.manager.List()
		m.statusMsg = fmt.Sprintf("Started: %s/%s :%d",
			startMsg.Preset.ResourceType, startMsg.Preset.ResourceName, startMsg.Preset.LocalPort)
		return m, nil
	}

	// Save mode: text input for preset/set name
	if m.saving || m.savingSet {
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
			if m.filtering {
				m.filtering = false
				m.filter.SetValue("")
				m.filter.Blur()
				return m, nil
			}
			m.cancelled = true
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if m.cursors[m.activeTab] > 0 {
				m.cursors[m.activeTab]--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursors[m.activeTab] < m.totalRows()-1 {
				m.cursors[m.activeTab]++
			}

		case msg.String() == "tab", msg.String() == "right":
			if !m.filtering {
				m.activeTab = (m.activeTab + 1) % tabCount
				return m, nil
			}

		case msg.String() == "shift+tab", msg.String() == "left":
			if !m.filtering {
				m.activeTab = (m.activeTab - 1 + tabCount) % tabCount
				return m, nil
			}

		case key.Matches(msg, m.keys.Enter):
			return m.handleEnter()

		case msg.String() == "n":
			m.wantsNew = true
			return m, nil

		case msg.String() == "/":
			if !m.filtering {
				m.filtering = true
				m.filter.Focus()
				return m, textinput.Blink
			}

		case msg.String() == "p":
			// Save current entry as preset (Only in tabActive)
			if m.activeTab == tabActive {
				idx := m.cursors[m.activeTab]
				entries := m.filteredEntries()
				if idx >= 0 && idx < len(entries) && m.cfg != nil {
					m.saving = true
					m.savingSet = false
					m.saveIdx = idx
					m.saveInput.Placeholder = "프리셋 이름..."
					m.saveInput.SetValue("")
					m.saveInput.Focus()
					return m, textinput.Blink
				}
			}

		case msg.String() == "s":
			// Save all active entries as a new PF Set
			if len(m.entries) > 0 && m.cfg != nil {
				m.savingSet = true
				m.saving = false
				m.saveInput.Placeholder = "세트 이름..."
				m.saveInput.SetValue("")
				m.saveInput.Focus()
				return m, textinput.Blink
			} else {
				m.statusMsg = "저장할 활성 포트포워드가 없습니다."
				return m, nil
			}

		case msg.String() == "r":
			removed := m.refreshEntries()
			if removed > 0 {
				m.statusMsg = fmt.Sprintf("%d개 끊어진 포트포워드 제거됨", removed)
			} else {
				m.statusMsg = "모든 포트포워드 정상"
			}
			return m, nil

		case key.Matches(msg, m.keys.Delete):
			return m.handleDelete()

		case msg.String() == "D":
			return m.handleClearHistory()
		}

		if m.filtering {
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			if m.cursors[m.activeTab] >= m.totalRows() {
				m.cursors[m.activeTab] = max(0, m.totalRows()-1)
			}
			return m, cmd
		}
	}
	return m, nil
}

// refreshEntries runs Cleanup, re-discovers external processes, and updates the local entries list.
func (m *PortForwardManagerModel) refreshEntries() int {
	removed := m.manager.Cleanup()
	m.manager.DiscoverExternal(m.profileMap)
	m.entries = m.manager.List()
	if m.cursors[m.activeTab] >= m.totalRows() {
		m.cursors[m.activeTab] = max(0, m.totalRows()-1)
	}
	return removed
}

func (m PortForwardManagerModel) handleEnter() (PortForwardManagerModel, tea.Cmd) {
	if m.cursors[m.activeTab] == m.totalRows()-1 {
		m.wantsNew = true
		return m, nil
	}

	switch m.activeTab {
	case tabSets:
		sets := m.filteredSets()
		if m.cursors[tabSets] >= 0 && m.cursors[tabSets] < len(sets) {
			set := sets[m.cursors[tabSets]]
			activeCount := 0
			for _, e := range m.entries {
				if e.SetName == set.Name && !e.External {
					activeCount++
				}
			}
			if activeCount > 0 {
				removed := m.manager.StopSet(set.Name)
				m.entries = m.manager.List()
				m.statusMsg = fmt.Sprintf("세트 '%s' 종료됨 (PF %d개)", set.Name, removed)
				return m, nil
			} else {
				var cmds []tea.Cmd
				for _, item := range set.Forwards {
					preset := config.PortForwardPreset{
						Profile:      item.Profile,
						Namespace:    item.Namespace,
						ResourceType: item.ResourceType,
						ResourceName: item.ResourceName,
						LocalPort:    item.LocalPort,
						RemotePort:   item.RemotePort,
					}
					cmds = append(cmds, m.launchPreset(preset, set.Name))
				}
				m.launching = true
				m.statusMsg = ""
				return m, tea.Batch(append(cmds, m.spinner.Tick)...)
			}
		}

	case tabActive:
		entries := m.filteredEntries()
		if m.cursors[tabActive] >= 0 && m.cursors[tabActive] < len(entries) {
			e := entries[m.cursors[tabActive]]
			if e.External {
				return m, nil
			}
			if e.Handle != nil && e.Handle.IsAlive() {
				return m, nil
			}
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
			return m, tea.Batch(m.spinner.Tick, m.launchPreset(preset, e.SetName))
		}

	case tabPresets:
		presets := m.filteredPresets()
		if m.cursors[tabPresets] >= 0 && m.cursors[tabPresets] < len(presets) {
			preset := presets[m.cursors[tabPresets]]
			m.launching = true
			m.statusMsg = ""
			return m, tea.Batch(m.spinner.Tick, m.launchPreset(preset, ""))
		}

	case tabHistory:
		history := m.filteredHistory()
		if m.cursors[tabHistory] >= 0 && m.cursors[tabHistory] < len(history) {
			h := history[m.cursors[tabHistory]]
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
			return m, tea.Batch(m.spinner.Tick, m.launchPreset(preset, ""))
		}
	}
	return m, nil
}

func (m PortForwardManagerModel) handleDelete() (PortForwardManagerModel, tea.Cmd) {
	switch m.activeTab {
	case tabSets:
		sets := m.filteredSets()
		if m.cursors[tabSets] >= 0 && m.cursors[tabSets] < len(sets) {
			set := sets[m.cursors[tabSets]]
			if m.cfg != nil {
				m.cfg.RemovePortForwardSet(set.Name)
				_ = config.Save(m.cfg)
				m.sets = m.cfg.Global.PortForwardSets
				m.statusMsg = fmt.Sprintf("세트 '%s' 삭제됨", set.Name)
			}
		}

	case tabActive:
		entries := m.filteredEntries()
		if m.cursors[tabActive] >= 0 && m.cursors[tabActive] < len(entries) {
			e := entries[m.cursors[tabActive]]
			if e.External {
				m.statusMsg = "외부 세션 포트포워드는 해당 세션에서 종료해주세요"
			} else {
				_ = m.manager.Remove(e.ID)
				m.entries = m.manager.List()
				m.statusMsg = fmt.Sprintf("종료됨: %s/%s :%d", e.ResourceType, e.ResourceName, e.LocalPort)
			}
		}

	case tabPresets:
		presets := m.filteredPresets()
		if m.cursors[tabPresets] >= 0 && m.cursors[tabPresets] < len(presets) {
			preset := presets[m.cursors[tabPresets]]
			if m.cfg != nil {
				m.cfg.RemovePreset(preset.Name)
				_ = config.Save(m.cfg)
				m.presets = m.cfg.GetPresetsForProfile(m.profile.Name)
				m.statusMsg = fmt.Sprintf("프리셋 '%s' 삭제됨", preset.Name)
			}
		}

	case tabHistory:
		history := m.filteredHistory()
		if m.cursors[tabHistory] >= 0 && m.cursors[tabHistory] < len(history) {
			h := history[m.cursors[tabHistory]]
			if m.cfg != nil {
				m.cfg.RemovePFHistory(h)
				_ = config.Save(m.cfg)
				m.history = m.cfg.GetPFHistoryForProfile(m.profile.Name)
				m.statusMsg = "히스토리 삭제됨"
			}
		}
	}

	if m.cursors[m.activeTab] >= m.totalRows() {
		m.cursors[m.activeTab] = max(0, m.totalRows()-1)
	}
	return m, nil
}

func (m PortForwardManagerModel) handleClearHistory() (PortForwardManagerModel, tea.Cmd) {
	if m.activeTab == tabHistory && m.cfg != nil {
		m.cfg.ClearPFHistoryForProfile(m.profile.Name)
		_ = config.Save(m.cfg)
		m.history = m.cfg.GetPFHistoryForProfile(m.profile.Name)
		m.cursors[tabHistory] = 0
		m.statusMsg = "히스토리 전체 삭제됨"
	}
	return m, nil
}

func (m PortForwardManagerModel) updateSaveMode(msg tea.Msg) (PortForwardManagerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.saving = false
			m.savingSet = false
			m.saveInput.SetValue("")
			m.saveInput.Blur()
			return m, nil
		case "enter":
			name := m.saveInput.Value()
			if name != "" {
				if m.savingSet {
					var items []config.PortForwardSetItem
					seenPorts := make(map[int]bool)
					for _, e := range m.entries {
						if seenPorts[e.LocalPort] {
							continue
						}
						seenPorts[e.LocalPort] = true
						items = append(items, config.PortForwardSetItem{
							Profile:      m.profile.Name,
							Namespace:    e.Namespace,
							ResourceType: e.ResourceType,
							ResourceName: e.ResourceName,
							LocalPort:    e.LocalPort,
							RemotePort:   e.RemotePort,
						})
					}
					m.cfg.AddPortForwardSet(config.PortForwardSet{
						Name:     name,
						Forwards: items,
					})
					_ = config.Save(m.cfg)
					m.sets = m.cfg.Global.PortForwardSets
					m.statusMsg = fmt.Sprintf("세트 저장 완료: %s", name)
				} else if m.saving && m.saveIdx < len(m.entries) {
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
			}
			m.saving = false
			m.savingSet = false
			m.saveInput.SetValue("")
			m.saveInput.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

func (m PortForwardManagerModel) launchPreset(preset config.PortForwardPreset, setName string) tea.Cmd {
	p := m.profile
	return func() tea.Msg {
		handle, err := k8s.StartPortForward(
			p.FilePath, p.Context,
			preset.Namespace, preset.ResourceType, preset.ResourceName,
			preset.LocalPort, preset.RemotePort,
		)
		return pfMgrPresetStartedMsg{Preset: preset, SetName: setName, Handle: handle, Err: err}
	}
}

// View renders the port-forward manager screen.
func (m PortForwardManagerModel) View() string {
	titleStr := fmt.Sprintf("k10s - %s  ›  Port Forward", m.profile.Name)
	title := StyleTitle.Render(padLine(titleStr, m.width-2))

	if m.launching {
		return title + "\n\n  " + m.spinner.View() + " 프리셋 포트포워드 시작 중...\n"
	}

	if m.saving || m.savingSet {
		var b strings.Builder
		if m.savingSet {
			b.WriteString(StyleNormal.Render(fmt.Sprintf("  세트로 저장할 포트포워드 갯수: %d", len(m.entries))) + "\n\n")
		} else {
			e := m.entries[m.saveIdx]
			b.WriteString(StyleNormal.Render(fmt.Sprintf("  프리셋으로 저장: %s/%s %d→%d",
				e.ResourceType, e.ResourceName, e.LocalPort, e.RemotePort)) + "\n\n")
		}
		b.WriteString("  이름: " + m.saveInput.View() + "\n\n")
		b.WriteString(StyleHelp.Render("  [enter] 저장   [esc] 취소"))

		modal := StyleModal.Render(padLine(b.String(), m.width-8))
		return title + "\n\n" + modal
	}

	var mainContent strings.Builder
	mainContent.WriteString("\n")

	// Render Tabs
	mainContent.WriteString(m.viewTabs())
	mainContent.WriteString("\n")

	if m.filtering {
		mainContent.WriteString("  / " + m.filter.View() + "\n\n")
	} else {
		mainContent.WriteString(StyleHelp.Render("  Press / to filter") + "\n\n")
	}

	// Content based on tab
	var listLines []string
	cursor := m.cursors[m.activeTab]

	// Determine available height for the list (rough estimate)
	listHeight := m.height - 15
	if listHeight < 5 {
		listHeight = 10
	}

	switch m.activeTab {
	case tabActive:
		entries := m.filteredEntries()
		if len(entries) == 0 {
			listLines = append(listLines, StyleDimmed.Render("  활성 포트포워드 없음"))
		} else {
			for i, e := range entries {
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

				if i == cursor {
					listLines = append(listLines, "  "+StyleSelected.Render("> "+line))
				} else {
					listLines = append(listLines, "  "+StyleNormal.Render("  "+line))
				}
			}
		}
	case tabSets:
		sets := m.filteredSets()
		if len(sets) == 0 {
			listLines = append(listLines, StyleDimmed.Render("  저장된 세트 없음"))
		} else {
			for i, s := range sets {
				activeCount := 0
				for _, e := range m.entries {
					if e.SetName == s.Name && !e.External {
						activeCount++
					}
				}
				status := " "
				if activeCount == len(s.Forwards) && len(s.Forwards) > 0 {
					status = "✅"
				} else if activeCount > 0 {
					status = "◐"
				}
				line := fmt.Sprintf("[%s] %s  (%d items)", status, s.Name, len(s.Forwards))
				if i == cursor {
					listLines = append(listLines, "  "+StyleSelected.Render("> "+line))
				} else {
					listLines = append(listLines, "  "+StyleNormal.Render("  "+line))
				}
			}
		}
	case tabPresets:
		presets := m.filteredPresets()
		if len(presets) == 0 {
			listLines = append(listLines, StyleDimmed.Render("  저장된 프리셋 없음"))
		} else {
			for i, p := range presets {
				line := fmt.Sprintf("[%s]  %s/%s  %d→%d  (%s)",
					p.Name, p.ResourceType, p.ResourceName,
					p.LocalPort, p.RemotePort, p.Namespace)
				if i == cursor {
					listLines = append(listLines, "  "+StyleSelected.Render("> "+line))
				} else {
					listLines = append(listLines, "  "+StyleDimmed.Render("  "+line))
				}
			}
		}
	case tabHistory:
		history := m.filteredHistory()
		if len(history) == 0 {
			listLines = append(listLines, StyleDimmed.Render("  히스토리 없음"))
		} else {
			for i, h := range history {
				line := fmt.Sprintf("%s/%s  %d→%d  (%s)",
					h.ResourceType, h.ResourceName,
					h.LocalPort, h.RemotePort, h.Namespace)
				if i == cursor {
					listLines = append(listLines, "  "+StyleSelected.Render("> "+line))
				} else {
					listLines = append(listLines, "  "+StyleDimmed.Render("  "+line))
				}
			}
		}
	}

	// Add "New Port Forward" row to the end of the list
	{
		newLabel := "[+] 새 포트포워드"
		if cursor == m.totalRows()-1 {
			listLines = append(listLines, "  "+StyleSelected.Render("> "+newLabel))
		} else {
			listLines = append(listLines, "  "+StyleNormal.Render("  "+newLabel))
		}
	}

	// Pagination & Full Width Padding
	paginated := paginateList(listLines, cursor, listHeight)
	var paddedLines []string
	for _, l := range paginated {
		paddedLines = append(paddedLines, padLine(l, m.width-4))
	}
	
	mainContent.WriteString(StyleActiveBox.Render(strings.Join(paddedLines, "\n")) + "\n\n")

	if m.statusMsg != "" {
		mainContent.WriteString(StyleWarning.Render("  "+m.statusMsg) + "\n\n")
	}

	help := renderHelp(
		"↑↓/jk", "move",
		"tab/←→", "탭 이동",
		"/", "filter",
		"enter", "실행/토글",
		"n", "새로 만들기",
		"p", "프리셋 저장",
		"s", "세트 저장",
		"r", "새로고침",
		"ctrl+d", "삭제",
		"D", "히스토리 비우기",
		"←/esc", "back",
		"q", "quit",
	)

	return title + "\n" + mainContent.String() + help
}

func (m PortForwardManagerModel) viewTabs() string {
	tabs := []string{"실행 중", "세트", "프리셋", "히스토리"}
	var renderedTabs []string
	for i, t := range tabs {
		label := fmt.Sprintf(" %s ", t)
		if i == tabActive && len(m.entries) > 0 {
			label = fmt.Sprintf(" %s (%d) ", t, len(m.entries))
		}
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, StyleTabActive.Render(label))
		} else {
			renderedTabs = append(renderedTabs, StyleTabInactive.Render(label))
		}
	}
	return "  " + strings.Join(renderedTabs, " ") + "\n"
}

// Cancelled returns true if the user pressed back.
func (m PortForwardManagerModel) Cancelled() bool {
	return m.cancelled
}

// WantsCreate returns true if the user wants to create a new port-forward.
func (m PortForwardManagerModel) WantsCreate() bool {
	return m.wantsNew
}
