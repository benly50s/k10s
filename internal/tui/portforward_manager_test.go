package tui

import (
	"testing"

	"github.com/benly/k10s/internal/portforward"
	"github.com/benly/k10s/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

func TestPortForwardManager_Update(t *testing.T) {
	// 1. Setup minimal model
	p := profile.Profile{Name: "test-cluster"}
	mgr := portforward.NewManager()
	
	model := NewPortForwardManagerModel(p, mgr, nil, nil)

	t.Run("Filter Mode Toggle", func(t *testing.T) {
		// When we type '/', filtering should become true
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
		newModel, _ := model.Update(msg)
		m := newModel

		if !m.filtering {
			t.Errorf("Expected filtering to be true after pressing '/'")
		}

		// When we press 'esc' in filter mode, filtering should turn off
		escMsg := tea.KeyMsg{Type: tea.KeyEsc}
		newModel, _ = m.Update(escMsg)
		m = newModel

		if m.filtering {
			t.Errorf("Expected filtering to be false after pressing 'esc'")
		}
	})

	t.Run("Quit Key", func(t *testing.T) {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
		newModel, _ := model.Update(msg)
		m := newModel

		if !m.Cancelled() {
			t.Errorf("Expected Cancelled() to be true after pressing 'q'")
		}
	})
	
	t.Run("Back Key (Left)", func(t *testing.T) {
		msg := tea.KeyMsg{Type: tea.KeyLeft}
		newModel, _ := model.Update(msg)
		m := newModel

		if !m.Cancelled() {
			t.Errorf("Expected Cancelled() to be true after pressing Left arrow")
		}
	})
}
