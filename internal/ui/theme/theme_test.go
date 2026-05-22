package theme

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestStateColor(t *testing.T) {
	tests := []struct {
		state    string
		expected tcell.Color
	}{
		{"running", Colors.Running},
		{"stopped", Colors.Stopped},
		{"terminated", Colors.Terminated},
		{"shutting-down", Colors.Terminated},
		{"pending", Colors.Pending},
		{"stopping", Colors.Pending},
		{"unknown", Colors.Foreground},
	}

	for _, tt := range tests {
		got := StateColor(tt.state)
		if got != tt.expected {
			t.Errorf("StateColor(%q): expected %v, got %v", tt.state, tt.expected, got)
		}
	}
}
