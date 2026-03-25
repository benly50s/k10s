package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/benly/k10s/internal/auth"
	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/k8s"
	"github.com/benly/k10s/internal/portforward"
	"github.com/benly/k10s/internal/profile"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// pfCreateStep tracks the current sub-step of the create flow
type pfCreateStep int

const (
	pfStepPreset pfCreateStep = iota
	pfStepOIDC
	pfStepNamespace
	pfStepType
	pfStepResource
	pfStepPort
	pfStepExecuting
)

// async message types for resource fetching
type pfNamespacesMsg struct {
	Items []string
	Err   error
}

type pfResourcesMsg struct {
	Items []string
	Err   error
}

type pfPortsMsg struct {
	Ports []k8s.ResourcePort
	Err   error
}

type pfStartedMsg struct {
	Handle *k8s.PortForwardHandle
	Err    error
}

type pfOIDCDoneMsg struct {
	Err error
}

// PortForwardCreateModel is the multi-step port-forward creation flow.
type PortForwardCreateModel struct {
	profile  profile.Profile
	manager  *portforward.Manager
	cfg      *config.K10sConfig
	presets  []config.PortForwardPreset
	history  []config.PortForwardHistoryEntry
	presetCursor int
	step     pfCreateStep
	loading  bool
	spinner  spinner.Model
	keys     KeyMap
	errMsg   string

	// namespace selection
	namespaces []string
	nsFiltered []string
	nsHistory  []string // recently used namespace names
	nsFilter   textinput.Model
	nsCursor   int

	// type selection
	resourceTypes []string
	typeCursor    int

	// resource selection
	selectedNS       string
	selectedType     string // svc, pod, deployment
	resources        []string
	resFiltered      []string
	resFilter        textinput.Model
	resCursor        int

	// port input
	selectedResource string
	availPorts       []k8s.ResourcePort
	portInput        textinput.Model
	portHint         string

	cancelled bool
	done      bool
}

// NewPortForwardCreateModel creates the PF creation flow.
func NewPortForwardCreateModel(p profile.Profile, mgr *portforward.Manager, cfg *config.K10sConfig) PortForwardCreateModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StyleSelected

	nsFilter := textinput.New()
	nsFilter.Placeholder = "filter..."
	nsFilter.CharLimit = 64

	resFilter := textinput.New()
	resFilter.Placeholder = "filter..."
	resFilter.CharLimit = 64

	portIn := textinput.New()
	portIn.Placeholder = "local:remote (예: 8080:80)"
	portIn.CharLimit = 11
	portIn.Focus()

	var presets []config.PortForwardPreset
	if cfg != nil {
		presets = cfg.GetPresetsForProfile(p.Name)
	}

	var history []config.PortForwardHistoryEntry
	if cfg != nil {
		history = cfg.GetPFHistoryForProfile(p.Name)
	}

	var nsHistory []string
	if cfg != nil {
		for _, h := range cfg.GetPodLogNSHistoryForProfile(p.Name) {
			nsHistory = append(nsHistory, h.Namespace)
		}
	}

	step := pfStepNamespace
	if len(presets) > 0 || len(history) > 0 {
		step = pfStepPreset
	} else if p.OIDC {
		step = pfStepOIDC
	}

	return PortForwardCreateModel{
		profile:       p,
		manager:       mgr,
		cfg:           cfg,
		presets:       presets,
		history:       history,
		step:          step,
		loading:       step != pfStepPreset,
		spinner:       s,
		keys:          DefaultKeyMap(),
		nsFilter:      nsFilter,
		nsHistory:     nsHistory,
		resFilter:     resFilter,
		portInput:     portIn,
		resourceTypes: []string{"svc", "pod", "deployment"},
	}
}

// Init starts OIDC refresh (if needed) then namespace fetch.
func (m PortForwardCreateModel) Init() tea.Cmd {
	if m.step == pfStepPreset {
		return nil // preset selection is synchronous
	}
	if m.step == pfStepOIDC {
		return tea.Batch(m.spinner.Tick, m.refreshOIDC())
	}
	return tea.Batch(m.spinner.Tick, m.fetchNamespaces())
}

func (m PortForwardCreateModel) refreshOIDC() tea.Cmd {
	kubecfg := m.profile.FilePath
	ctx := m.profile.Context
	return func() tea.Msg {
		err := auth.RefreshOIDC(kubecfg, ctx)
		return pfOIDCDoneMsg{Err: err}
	}
}

func (m PortForwardCreateModel) fetchNamespaces() tea.Cmd {
	return func() tea.Msg {
		ns, err := k8s.FetchNamespaces(m.profile.FilePath, m.profile.Context)
		return pfNamespacesMsg{Items: ns, Err: err}
	}
}

func (m PortForwardCreateModel) fetchResources() tea.Cmd {
	resType := m.selectedType
	ns := m.selectedNS
	kubecfg := m.profile.FilePath
	ctx := m.profile.Context
	return func() tea.Msg {
		var items []string
		var err error
		switch resType {
		case "svc":
			items, err = k8s.FetchServices(kubecfg, ctx, ns)
		case "pod":
			items, err = k8s.FetchPods(kubecfg, ctx, ns)
		case "deployment":
			items, err = k8s.FetchDeployments(kubecfg, ctx, ns)
		}
		return pfResourcesMsg{Items: items, Err: err}
	}
}

func (m PortForwardCreateModel) fetchPorts() tea.Cmd {
	kubecfg := m.profile.FilePath
	ctx := m.profile.Context
	ns := m.selectedNS
	resType := m.selectedType
	name := m.selectedResource
	return func() tea.Msg {
		var ports []k8s.ResourcePort
		var err error
		switch resType {
		case "svc":
			ports, err = k8s.FetchServicePorts(kubecfg, ctx, ns, name)
		case "pod":
			ports, err = k8s.FetchPodPorts(kubecfg, ctx, ns, name)
		case "deployment":
			ports, err = k8s.FetchDeploymentPorts(kubecfg, ctx, ns, name)
		}
		return pfPortsMsg{Ports: ports, Err: err}
	}
}

func (m PortForwardCreateModel) startPortForward(localPort, remotePort int) tea.Cmd {
	kubecfg := m.profile.FilePath
	ctx := m.profile.Context
	ns := m.selectedNS
	resType := m.selectedType
	name := m.selectedResource
	return func() tea.Msg {
		handle, err := k8s.StartPortForward(kubecfg, ctx, ns, resType, name, localPort, remotePort)
		return pfStartedMsg{Handle: handle, Err: err}
	}
}

// Update handles messages for the create flow.
func (m PortForwardCreateModel) Update(msg tea.Msg) (PortForwardCreateModel, tea.Cmd) {
	// Handle spinner
	if tickMsg, ok := msg.(spinner.TickMsg); ok && m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(tickMsg)
		return m, cmd
	}

	switch m.step {
	case pfStepPreset:
		return m.updatePreset(msg)
	case pfStepOIDC:
		return m.updateOIDC(msg)
	case pfStepNamespace:
		return m.updateNamespace(msg)
	case pfStepType:
		return m.updateType(msg)
	case pfStepResource:
		return m.updateResource(msg)
	case pfStepPort:
		return m.updatePort(msg)
	case pfStepExecuting:
		return m.updateExecuting(msg)
	}
	return m, nil
}

func (m PortForwardCreateModel) updatePreset(msg tea.Msg) (PortForwardCreateModel, tea.Cmd) {
	// Items: presets + history + 1 (custom)
	totalItems := len(m.presets) + len(m.history) + 1

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
			if m.presetCursor > 0 {
				m.presetCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.presetCursor < totalItems-1 {
				m.presetCursor++
			}
		case key.Matches(msg, m.keys.Delete):
			// Delete preset or history item
			if m.presetCursor < len(m.presets) && m.cfg != nil {
				preset := m.presets[m.presetCursor]
				m.cfg.RemovePreset(preset.Name)
				_ = config.Save(m.cfg)
				m.presets = m.cfg.GetPresetsForProfile(m.profile.Name)
			} else if m.presetCursor >= len(m.presets) && m.presetCursor < len(m.presets)+len(m.history) && m.cfg != nil {
				h := m.history[m.presetCursor-len(m.presets)]
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
			}
			newTotal := len(m.presets) + len(m.history) + 1
			if m.presetCursor >= newTotal {
				m.presetCursor = max(0, newTotal-1)
			}
			// If no presets/history left, skip to namespace step
			if len(m.presets) == 0 && len(m.history) == 0 {
				if m.profile.OIDC {
					m.step = pfStepOIDC
					m.loading = true
					return m, tea.Batch(m.spinner.Tick, m.refreshOIDC())
				}
				m.step = pfStepNamespace
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.fetchNamespaces())
			}

		case key.Matches(msg, m.keys.Enter):
			if m.presetCursor < len(m.presets) {
				// Launch preset directly
				preset := m.presets[m.presetCursor]
				m.selectedNS = preset.Namespace
				m.selectedType = preset.ResourceType
				m.selectedResource = preset.ResourceName
				m.portInput.SetValue(fmt.Sprintf("%d:%d", preset.LocalPort, preset.RemotePort))
				m.step = pfStepExecuting
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.startPortForward(preset.LocalPort, preset.RemotePort))
			}
			if m.presetCursor >= len(m.presets) && m.presetCursor < len(m.presets)+len(m.history) {
				// Launch from history entry
				h := m.history[m.presetCursor-len(m.presets)]
				m.selectedNS = h.Namespace
				m.selectedType = h.ResourceType
				m.selectedResource = h.ResourceName
				m.portInput.SetValue(fmt.Sprintf("%d:%d", h.LocalPort, h.RemotePort))
				m.step = pfStepExecuting
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.startPortForward(h.LocalPort, h.RemotePort))
			}
			// "커스텀 생성" selected → proceed to OIDC or namespace
			if m.profile.OIDC {
				m.step = pfStepOIDC
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.refreshOIDC())
			}
			m.step = pfStepNamespace
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchNamespaces())
		}
	}
	return m, nil
}

func (m PortForwardCreateModel) updateOIDC(msg tea.Msg) (PortForwardCreateModel, tea.Cmd) {
	if oidcMsg, ok := msg.(pfOIDCDoneMsg); ok {
		if oidcMsg.Err != nil {
			m.loading = false
			m.errMsg = fmt.Sprintf("OIDC 인증 실패: %v", oidcMsg.Err)
			m.step = pfStepNamespace // show error in namespace step
			return m, nil
		}
		// OIDC done, proceed to namespace fetch
		m.step = pfStepNamespace
		return m, m.fetchNamespaces()
	}
	return m, nil
}

func (m PortForwardCreateModel) updateNamespace(msg tea.Msg) (PortForwardCreateModel, tea.Cmd) {
	// Handle async load
	if nsMsg, ok := msg.(pfNamespacesMsg); ok {
		m.loading = false
		if nsMsg.Err != nil {
			m.errMsg = nsMsg.Err.Error()
			return m, nil
		}
		m.namespaces = nsMsg.Items
		m.nsFiltered = m.sortNamespacesWithHistory(nsMsg.Items)
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Error state — back only
		if m.errMsg != "" {
			switch {
			case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Quit):
				m.cancelled = true
			}
			return m, nil
		}

		if m.nsFilter.Focused() {
			switch msg.String() {
			case "esc":
				m.nsFilter.Blur()
				m.nsFilter.SetValue("")
				m.nsFiltered = m.namespaces
				return m, nil
			case "enter":
				if m.nsCursor < len(m.nsFiltered) {
					m.selectedNS = m.nsFiltered[m.nsCursor]
					m.step = pfStepType
				}
				return m, nil
			case "up", "ctrl+p":
				if m.nsCursor > 0 {
					m.nsCursor--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.nsCursor < len(m.nsFiltered)-1 {
					m.nsCursor++
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.nsFilter, cmd = m.nsFilter.Update(msg)
			m.applyNSFilter()
			return m, cmd
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.cancelled = true
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.nsCursor > 0 {
				m.nsCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.nsCursor < len(m.nsFiltered)-1 {
				m.nsCursor++
			}
		case key.Matches(msg, m.keys.Enter):
			if m.nsCursor < len(m.nsFiltered) {
				m.selectedNS = m.nsFiltered[m.nsCursor]
				m.step = pfStepType
			}
		case key.Matches(msg, m.keys.Search):
			m.nsFilter.Focus()
			return m, textinput.Blink
		case key.Matches(msg, m.keys.Delete):
			if m.cfg != nil && m.nsCursor < len(m.nsFiltered) {
				ns := m.nsFiltered[m.nsCursor]
				histSet := make(map[string]bool, len(m.nsHistory))
				for _, h := range m.nsHistory {
					histSet[h] = true
				}
				if histSet[ns] {
					m.cfg.RemovePodLogNSHistory(m.profile.Name, ns)
					_ = config.Save(m.cfg)
					m.nsHistory = nil
					for _, h := range m.cfg.GetPodLogNSHistoryForProfile(m.profile.Name) {
						m.nsHistory = append(m.nsHistory, h.Namespace)
					}
					m.nsFiltered = m.sortNamespacesWithHistory(m.namespaces)
					if m.nsCursor >= len(m.nsFiltered) {
						m.nsCursor = max(0, len(m.nsFiltered)-1)
					}
				}
			}
		}
	}
	return m, nil
}

func (m *PortForwardCreateModel) applyNSFilter() {
	q := strings.ToLower(m.nsFilter.Value())
	if q == "" {
		m.nsFiltered = m.sortNamespacesWithHistory(m.namespaces)
		return
	}
	var out []string
	for _, ns := range m.namespaces {
		if strings.Contains(strings.ToLower(ns), q) {
			out = append(out, ns)
		}
	}
	m.nsFiltered = out
	if m.nsCursor >= len(m.nsFiltered) {
		m.nsCursor = max(0, len(m.nsFiltered)-1)
	}
}

// sortNamespacesWithHistory returns namespaces with recently used ones first.
func (m *PortForwardCreateModel) sortNamespacesWithHistory(all []string) []string {
	if len(m.nsHistory) == 0 {
		return all
	}
	histSet := make(map[string]bool, len(m.nsHistory))
	for _, h := range m.nsHistory {
		histSet[h] = true
	}
	var recent, rest []string
	for _, h := range m.nsHistory {
		for _, ns := range all {
			if ns == h {
				recent = append(recent, ns)
				break
			}
		}
	}
	for _, ns := range all {
		if !histSet[ns] {
			rest = append(rest, ns)
		}
	}
	return append(recent, rest...)
}

func (m PortForwardCreateModel) updateType(msg tea.Msg) (PortForwardCreateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.step = pfStepNamespace
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.typeCursor > 0 {
				m.typeCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.typeCursor < len(m.resourceTypes)-1 {
				m.typeCursor++
			}
		case key.Matches(msg, m.keys.Enter):
			m.selectedType = m.resourceTypes[m.typeCursor]
			m.step = pfStepResource
			m.loading = true
			m.errMsg = ""
			return m, tea.Batch(m.spinner.Tick, m.fetchResources())
		case msg.String() == "1", msg.String() == "2", msg.String() == "3":
			idx := int(msg.String()[0] - '1')
			if idx < len(m.resourceTypes) {
				m.selectedType = m.resourceTypes[idx]
				m.step = pfStepResource
				m.loading = true
				m.errMsg = ""
				return m, tea.Batch(m.spinner.Tick, m.fetchResources())
			}
		}
	}
	return m, nil
}

func (m PortForwardCreateModel) updateResource(msg tea.Msg) (PortForwardCreateModel, tea.Cmd) {
	// Handle async load
	if resMsg, ok := msg.(pfResourcesMsg); ok {
		m.loading = false
		if resMsg.Err != nil {
			m.errMsg = resMsg.Err.Error()
			return m, nil
		}
		m.resources = resMsg.Items
		m.resFiltered = resMsg.Items
		m.resCursor = 0
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.errMsg != "" {
			switch {
			case key.Matches(msg, m.keys.Back):
				m.step = pfStepType
				m.errMsg = ""
			case key.Matches(msg, m.keys.Quit):
				m.cancelled = true
				return m, tea.Quit
			}
			return m, nil
		}

		if m.resFilter.Focused() {
			switch msg.String() {
			case "esc":
				m.resFilter.Blur()
				m.resFilter.SetValue("")
				m.resFiltered = m.resources
				return m, nil
			case "enter":
				if m.resCursor < len(m.resFiltered) {
					m.selectedResource = m.resFiltered[m.resCursor]
					m.step = pfStepPort
					m.loading = true
					m.errMsg = ""
					return m, tea.Batch(m.spinner.Tick, m.fetchPorts())
				}
				return m, nil
			case "up", "ctrl+p":
				if m.resCursor > 0 {
					m.resCursor--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.resCursor < len(m.resFiltered)-1 {
					m.resCursor++
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.resFilter, cmd = m.resFilter.Update(msg)
			m.applyResFilter()
			return m, cmd
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.step = pfStepType
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.resCursor > 0 {
				m.resCursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.resCursor < len(m.resFiltered)-1 {
				m.resCursor++
			}
		case key.Matches(msg, m.keys.Enter):
			if m.resCursor < len(m.resFiltered) {
				m.selectedResource = m.resFiltered[m.resCursor]
				m.step = pfStepPort
				m.loading = true
				m.errMsg = ""
				return m, tea.Batch(m.spinner.Tick, m.fetchPorts())
			}
		case key.Matches(msg, m.keys.Search):
			m.resFilter.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m *PortForwardCreateModel) applyResFilter() {
	q := strings.ToLower(m.resFilter.Value())
	if q == "" {
		m.resFiltered = m.resources
		return
	}
	var out []string
	for _, r := range m.resources {
		if strings.Contains(strings.ToLower(r), q) {
			out = append(out, r)
		}
	}
	m.resFiltered = out
	if m.resCursor >= len(m.resFiltered) {
		m.resCursor = max(0, len(m.resFiltered)-1)
	}
}

func (m PortForwardCreateModel) updatePort(msg tea.Msg) (PortForwardCreateModel, tea.Cmd) {
	// Handle async port fetch
	if portsMsg, ok := msg.(pfPortsMsg); ok {
		m.loading = false
		if portsMsg.Err != nil {
			m.portHint = "포트 자동감지 실패 — 직접 입력하세요"
		} else if len(portsMsg.Ports) > 0 {
			var hints []string
			for _, p := range portsMsg.Ports {
				if p.Name != "" {
					hints = append(hints, fmt.Sprintf("%s:%d", p.Name, p.Port))
				} else {
					hints = append(hints, fmt.Sprintf("%d", p.Port))
				}
			}
			m.portHint = "감지된 포트: " + strings.Join(hints, ", ")
			// Auto-fill with first port
			if len(portsMsg.Ports) > 0 {
				p := portsMsg.Ports[0].Port
				m.portInput.SetValue(fmt.Sprintf("%d:%d", p, p))
			}
		}
		m.availPorts = portsMsg.Ports
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			m.step = pfStepResource
			m.errMsg = ""
			m.portHint = ""
			return m, nil
		case "enter":
			local, remote, err := parsePorts(m.portInput.Value())
			if err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.errMsg = ""
			m.step = pfStepExecuting
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.startPortForward(local, remote))
		}
		var cmd tea.Cmd
		m.portInput, cmd = m.portInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m PortForwardCreateModel) updateExecuting(msg tea.Msg) (PortForwardCreateModel, tea.Cmd) {
	if startMsg, ok := msg.(pfStartedMsg); ok {
		m.loading = false
		if startMsg.Err != nil {
			m.errMsg = startMsg.Err.Error()
			m.step = pfStepPort
			return m, nil
		}
		// Parse ports again to register
		local, remote, _ := parsePorts(m.portInput.Value())
		m.manager.Add(portforward.Entry{
			Profile:      m.profile.Name,
			Namespace:    m.selectedNS,
			ResourceType: m.selectedType,
			ResourceName: m.selectedResource,
			LocalPort:    local,
			RemotePort:   remote,
			Handle:       startMsg.Handle,
			StartedAt:    time.Now(),
		})
		if m.cfg != nil {
			m.cfg.AddPFHistory(config.PortForwardHistoryEntry{
				Profile:      m.profile.Name,
				Namespace:    m.selectedNS,
				ResourceType: m.selectedType,
				ResourceName: m.selectedResource,
				LocalPort:    local,
				RemotePort:   remote,
			})
			m.cfg.AddPodLogNSHistory(config.PodLogNSHistoryEntry{
				Profile:   m.profile.Name,
				Namespace: m.selectedNS,
			})
			_ = config.Save(m.cfg)
		}
		m.done = true
		return m, nil
	}
	return m, nil
}

// View renders the current step.
func (m PortForwardCreateModel) View() string {
	breadcrumb := fmt.Sprintf("k10s - %s  ›  Port Forward  ›  새로 만들기", m.profile.Name)
	title := StyleTitle.Render(breadcrumb)

	if m.loading {
		stepLabel := ""
		switch m.step {
		case pfStepOIDC:
			stepLabel = "OIDC 인증 중..."
		case pfStepNamespace:
			stepLabel = "네임스페이스 조회 중..."
		case pfStepResource:
			stepLabel = "리소스 조회 중..."
		case pfStepPort:
			stepLabel = "포트 조회 중..."
		case pfStepExecuting:
			stepLabel = "포트포워드 시작 중..."
		}
		return title + "\n\n  " + m.spinner.View() + " " + stepLabel + "\n"
	}

	switch m.step {
	case pfStepPreset:
		return m.viewPreset(title)
	case pfStepNamespace:
		return m.viewNamespace(title)
	case pfStepType:
		return m.viewType(title)
	case pfStepResource:
		return m.viewResource(title)
	case pfStepPort:
		return m.viewPort(title)
	}
	return title + "\n"
}

func (m PortForwardCreateModel) viewPreset(title string) string {
	content := "\n"
	content += StyleNormal.Render("  프리셋 또는 커스텀 생성:") + "\n\n"

	for i, p := range m.presets {
		line := fmt.Sprintf("[%s]  %s/%s  %d→%d  (%s)",
			p.Name, p.ResourceType, p.ResourceName,
			p.LocalPort, p.RemotePort, p.Namespace)
		if i == m.presetCursor {
			content += "  " + StyleSelected.Render("> "+line) + "\n"
		} else {
			content += "  " + StyleNormal.Render("  "+line) + "\n"
		}
	}

	// History section
	if len(m.history) > 0 {
		content += "\n" + StyleNormal.Render("  History:") + "\n\n"
		for i, h := range m.history {
			idx := len(m.presets) + i
			line := fmt.Sprintf("%s/%s  %d→%d  (%s)",
				h.ResourceType, h.ResourceName,
				h.LocalPort, h.RemotePort, h.Namespace)
			if idx == m.presetCursor {
				content += "  " + StyleSelected.Render("> "+line) + "\n"
			} else {
				content += "  " + StyleDimmed.Render("  "+line) + "\n"
			}
		}
	}

	// Custom create option
	customLabel := "[+] 커스텀 생성"
	if m.presetCursor == len(m.presets)+len(m.history) {
		content += "  " + StyleSelected.Render("> "+customLabel) + "\n"
	} else {
		content += "  " + StyleNormal.Render("  "+customLabel) + "\n"
	}

	content += "\n"
	help := renderHelp(
		"↑↓/jk", "move",
		"enter", "선택",
		"ctrl+d", "삭제",
		"←/esc", "back",
		"q", "quit",
	)
	return title + "\n" + content + help
}

func (m PortForwardCreateModel) viewNamespace(title string) string {
	content := "\n"

	if m.errMsg != "" {
		content += StyleWarning.Render("  Error: "+m.errMsg) + "\n"
		help := StyleHelp.Render("  [←/esc] back   [q] quit")
		return title + "\n" + content + help
	}

	content += StyleNormal.Render("  Select Namespace:") + "\n\n"

	if m.nsFilter.Focused() {
		content += "  / " + m.nsFilter.View() + "\n\n"
	} else {
		content += StyleHelp.Render("  Press / to filter") + "\n\n"
	}

	// Build set of recent namespaces for section headers
	histSet := make(map[string]bool, len(m.nsHistory))
	for _, h := range m.nsHistory {
		histSet[h] = true
	}
	showedRecentHeader := false
	showedAllHeader := false

	for i, ns := range m.nsFiltered {
		if !m.nsFilter.Focused() && m.nsFilter.Value() == "" && len(m.nsHistory) > 0 {
			if !showedRecentHeader && histSet[ns] {
				content += StyleDimmed.Render("  Recent:") + "\n"
				showedRecentHeader = true
			}
			if !showedAllHeader && !histSet[ns] {
				if showedRecentHeader {
					content += "\n" + StyleDimmed.Render("  All:") + "\n"
				}
				showedAllHeader = true
			}
		}

		cursor := "  "
		if i == m.nsCursor {
			cursor = "> "
			content += "  " + StyleSelected.Render(cursor+ns) + "\n"
		} else {
			content += "  " + StyleNormal.Render(cursor+ns) + "\n"
		}
	}

	content += "\n"
	help := renderHelp(
		"↑↓/jk", "move",
		"/", "filter",
		"enter", "select",
		"ctrl+d", "히스토리 삭제",
		"←/esc", "back",
		"q", "quit",
	)
	return title + "\n" + content + help
}

func (m PortForwardCreateModel) viewType(title string) string {
	content := "\n"
	content += StyleDimmed.Render(fmt.Sprintf("  Namespace: %s", m.selectedNS)) + "\n\n"
	content += StyleNormal.Render("  Select Resource Type:") + "\n\n"

	typeLabels := map[string]string{
		"svc":        "Service",
		"pod":        "Pod",
		"deployment": "Deployment",
	}

	for i, rt := range m.resourceTypes {
		cursor := "  "
		label := fmt.Sprintf("%d. %s", i+1, typeLabels[rt])
		if i == m.typeCursor {
			cursor = "> "
			content += "  " + StyleSelected.Render(cursor+label) + "\n"
		} else {
			content += "  " + StyleNormal.Render(cursor+label) + "\n"
		}
	}

	content += "\n"
	help := renderHelp(
		"↑↓/jk", "move",
		"1-3", "바로 선택",
		"enter", "select",
		"←/esc", "back",
		"q", "quit",
	)
	return title + "\n" + content + help
}

func (m PortForwardCreateModel) viewResource(title string) string {
	content := "\n"
	content += StyleDimmed.Render(fmt.Sprintf("  Namespace: %s  ›  Type: %s", m.selectedNS, m.selectedType)) + "\n\n"

	if m.errMsg != "" {
		content += StyleWarning.Render("  Error: "+m.errMsg) + "\n"
		help := renderHelp("←/esc", "back", "q", "quit")
		return title + "\n" + content + help
	}

	if len(m.resFiltered) == 0 {
		content += StyleDimmed.Render("  리소스 없음") + "\n"
		help := renderHelp("←/esc", "back", "q", "quit")
		return title + "\n" + content + help
	}

	content += StyleNormal.Render("  Select Resource:") + "\n\n"

	if m.resFilter.Focused() {
		content += "  / " + m.resFilter.View() + "\n\n"
	} else {
		content += StyleHelp.Render("  Press / to filter") + "\n\n"
	}

	for i, r := range m.resFiltered {
		cursor := "  "
		if i == m.resCursor {
			cursor = "> "
			content += "  " + StyleSelected.Render(cursor+r) + "\n"
		} else {
			content += "  " + StyleNormal.Render(cursor+r) + "\n"
		}
	}

	content += "\n"
	help := renderHelp(
		"↑↓/jk", "move",
		"/", "filter",
		"enter", "select",
		"←/esc", "back",
		"q", "quit",
	)
	return title + "\n" + content + help
}

func (m PortForwardCreateModel) viewPort(title string) string {
	content := "\n"
	content += StyleDimmed.Render(fmt.Sprintf("  %s  ›  %s  ›  %s/%s",
		m.selectedNS, m.selectedType, m.selectedType, m.selectedResource)) + "\n\n"

	content += StyleNormal.Render("  포트 입력 (local:remote):") + "\n\n"

	if m.portHint != "" {
		content += StyleDimmed.Render("  "+m.portHint) + "\n\n"
	}

	content += "  " + m.portInput.View() + "\n\n"

	if m.errMsg != "" {
		content += StyleWarning.Render("  "+m.errMsg) + "\n\n"
	}

	help := renderHelp(
		"enter", "시작",
		"esc", "back",
	)
	return title + "\n" + content + help
}

// parsePorts parses "local:remote" string into two port numbers.
func parsePorts(s string) (local int, remote int, err error) {
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("형식: local:remote (예: 8080:80)")
	}
	local, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || local < 1 || local > 65535 {
		return 0, 0, fmt.Errorf("유효하지 않은 로컬 포트: %s", parts[0])
	}
	remote, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || remote < 1 || remote > 65535 {
		return 0, 0, fmt.Errorf("유효하지 않은 리모트 포트: %s", parts[1])
	}
	return local, remote, nil
}

// Cancelled returns true if user backed out.
func (m PortForwardCreateModel) Cancelled() bool {
	return m.cancelled
}

// Done returns true if port-forward was successfully created.
func (m PortForwardCreateModel) Done() bool {
	return m.done
}
