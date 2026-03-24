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

// Remove stops and removes a port-forward by ID
func (m *Manager) Remove(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.entries {
		if e.ID == id {
			e.Handle.Stop()
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

// StopAll stops and removes all active port-forwards
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		e.Handle.Stop()
	}
	m.entries = nil
}
