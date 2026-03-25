package tui

import (
	"strings"
	"testing"
)

func TestRenderHelp(t *testing.T) {
	tests := []struct {
		name     string
		pairs    []string
		expected []string // substrings that must be in the output
	}{
		{
			name:     "single pair",
			pairs:    []string{"enter", "select"},
			expected: []string{"[enter]", "select"},
		},
		{
			name:     "multiple pairs",
			pairs:    []string{"↑↓", "move", "enter", "select", "q", "quit"},
			expected: []string{"[↑↓]", "move", "[enter]", "select", "[q]", "quit"},
		},
		{
			name:     "odd number of args (missing desc)",
			pairs:    []string{"esc", "back", "enter"},
			expected: []string{"[esc]", "back", "[enter]"}, // "enter" has no description, should not crash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderHelp(tt.pairs...)
			
			// Just verify that the keys and descriptions are present in the rendered string
			// (ignoring exact lipgloss ANSI escape codes which may vary)
			for _, exp := range tt.expected {
				if !strings.Contains(got, exp) {
					t.Errorf("renderHelp() = %q, want it to contain %q", got, exp)
				}
			}
		})
	}
}
