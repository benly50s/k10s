package portforward

import (
	"fmt"
	"sync"
	"time"

	"github.com/benly/k10s/internal/k8s"
)

// Entry represents an active port-forward
type Entry struct {
	ID           int
	Profile      string // cluster name
	Namespace    string
	ResourceType string // svc, pod, deployment
	ResourceName string
	LocalPort    int
	RemotePort   int
	Handle       *k8s.PortForwardHandle
	StartedAt    time.Time
	External     bool // true if discovered from another session
}

// Label returns a human-readable label for the entry
func (e Entry) Label() string {
	return fmt.Sprintf("%s/%s  %d→%d  (%s/%s)",
		e.ResourceType, e.ResourceName,
		e.LocalPort, e.RemotePort,
		e.Profile, e.Namespace)
}

// Manager tracks active port-forward processes
type Manager struct {
	mu      sync.Mutex
	entries []Entry
	nextID  int
}

// NewManager creates a new port-forward manager
func NewManager() *Manager {
	return &Manager{nextID: 1}
}

// Add registers a new port-forward entry and returns its ID
func (m *Manager) Add(entry Entry) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry.ID = m.nextID
	m.nextID++
	m.entries = append(m.entries, entry)
	return entry.ID
}

// Remove stops and removes a port-forward by ID.
// External entries are only removed from tracking (not killed).
func (m *Manager) Remove(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.entries {
		if e.ID == id {
			if !e.External {
				e.Handle.Stop()
			}
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("port-forward #%d not found", id)
}

// List returns a copy of all active entries
func (m *Manager) List() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Entry, len(m.entries))
	copy(out, m.entries)
	return out
}

// Count returns the number of active port-forwards
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

// Cleanup removes entries whose port-forward process has died.
// Returns the number of removed entries.
func (m *Manager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	alive := m.entries[:0]
	removed := 0
	for _, e := range m.entries {
		if e.Handle != nil && e.Handle.IsAlive() {
			alive = append(alive, e)
		} else {
			removed++
		}
	}
	m.entries = alive
	return removed
}

// DiscoverExternal scans for kubectl port-forward processes from other sessions
// and adds them to the manager. It uses profile matching to set the Profile field.
// profileMap maps kubeconfigPath -> profileName for matching.
func (m *Manager) DiscoverExternal(profileMap map[string]string) int {
	m.mu.Lock()
	// Collect PIDs to exclude: our own children + already tracked external PIDs
	excludePIDs := make(map[int]bool)
	for _, e := range m.entries {
		if e.Handle != nil {
			if e.Handle.Cmd != nil && e.Handle.Cmd.Process != nil {
				excludePIDs[e.Handle.Cmd.Process.Pid] = true
			}
			if e.Handle.Process != nil {
				excludePIDs[e.Handle.Process.Pid] = true
			}
		}
	}
	// Collect local ports we already track to avoid duplicates
	knownPorts := make(map[int]bool)
	for _, e := range m.entries {
		knownPorts[e.LocalPort] = true
	}
	m.mu.Unlock()

	discovered := k8s.DiscoverPortForwards(excludePIDs)

	added := 0
	for _, d := range discovered {
		// Skip if we already track this port
		if knownPorts[d.LocalPort] {
			continue
		}

		profileName := ""
		if d.KubeconfigPath != "" {
			profileName = profileMap[d.KubeconfigPath]
		}

		m.Add(Entry{
			Profile:      profileName,
			Namespace:    d.Namespace,
			ResourceType: d.ResourceType,
			ResourceName: d.ResourceName,
			LocalPort:    d.LocalPort,
			RemotePort:   d.RemotePort,
			Handle:       d.Handle,
			StartedAt:    time.Now(),
			External:     true,
		})
		knownPorts[d.LocalPort] = true
		added++
	}
	return added
}

// StopAll stops and removes all active port-forwards.
// External (discovered) processes are NOT killed — only our own processes are stopped.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if !e.External {
			e.Handle.Stop()
		}
	}
	m.entries = nil
}
