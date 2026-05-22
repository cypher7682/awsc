// Package components provides reusable TUI widgets for awsc.
package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Shortcut represents a keyboard shortcut displayed in the header.
type Shortcut struct {
	Key   string
	Label string
}

// Header is the top bar showing profile, region, and keyboard shortcuts.
type Header struct {
	*tview.TextView
	profile   string
	region    string
	resource  string
	shortcuts []Shortcut
}

// NewHeader creates a new header component.
func NewHeader() *Header {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(tcell.ColorDarkSlateGray)

	h := &Header{
		TextView: tv,
		shortcuts: []Shortcut{
			{Key: "?", Label: "help"},
			{Key: ":", Label: "command"},
			{Key: "/", Label: "filter"},
			{Key: "Esc", Label: "back"},
			{Key: "q", Label: "quit"},
		},
	}
	h.Render()
	return h
}

// SetContext updates the profile and region display.
func (h *Header) SetContext(profile, region string) {
	h.profile = profile
	h.region = region
	h.Render()
}

// SetResource updates the current resource display.
func (h *Header) SetResource(resource string) {
	h.resource = resource
	h.Render()
}

// SetShortcuts sets the context-specific shortcuts.
func (h *Header) SetShortcuts(shortcuts []Shortcut) {
	h.shortcuts = shortcuts
	h.Render()
}

// Render redraws the header content.
func (h *Header) Render() {
	var b strings.Builder

	// Left side: app name + context
	b.WriteString("[::b]awsc[::-] ")
	if h.profile != "" {
		b.WriteString(fmt.Sprintf("[gray]profile:[white]%s ", h.profile))
	}
	if h.region != "" {
		b.WriteString(fmt.Sprintf("[gray]region:[dodgerblue]%s ", h.region))
	}
	if h.resource != "" {
		b.WriteString(fmt.Sprintf("[gray]>[white] %s", h.resource))
	}

	// Right side: shortcuts
	b.WriteString("  ")
	for i, sc := range h.shortcuts {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(fmt.Sprintf("[gold]<%s>[lightgray] %s", sc.Key, sc.Label))
	}

	h.SetText(b.String())
}
