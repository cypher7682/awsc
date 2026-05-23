// Package components provides reusable TUI widgets for awsc.
package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HeaderHeight is the fixed height of the header bar.
const HeaderHeight = 4

// Shortcut represents a keyboard shortcut displayed in the header.
type Shortcut struct {
	Key   string
	Label string
}

// Header is the top bar showing context info (left) and shortcuts (right).
type Header struct {
	flex      *tview.Flex
	infoPane  *tview.TextView
	shortcuts *tview.TextView

	profile    string
	region     string
	resource   string
	shortcutList []Shortcut
}

// NewHeader creates a new header component.
func NewHeader() *Header {
	infoPane := tview.NewTextView()
	infoPane.SetDynamicColors(true)
	infoPane.SetBackgroundColor(tcell.ColorDarkSlateGray)

	shortcutsPane := tview.NewTextView()
	shortcutsPane.SetDynamicColors(true)
	shortcutsPane.SetBackgroundColor(tcell.ColorDarkSlateGray)

	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.AddItem(infoPane, 0, 1, false)
	flex.AddItem(shortcutsPane, 0, 1, false)

	h := &Header{
		flex:      flex,
		infoPane:  infoPane,
		shortcuts: shortcutsPane,
		shortcutList: []Shortcut{
			{Key: "?", Label: "help"},
			{Key: ":", Label: "command"},
			{Key: "/", Label: "filter"},
			{Key: "Esc", Label: "back"},
			{Key: "q", Label: "quit"},
		},
	}
	h.render()
	return h
}

// Widget returns the root primitive for embedding in layouts.
func (h *Header) Widget() tview.Primitive {
	return h.flex
}

// SetContext updates the profile and region display.
func (h *Header) SetContext(profile, region string) {
	h.profile = profile
	h.region = region
	h.render()
}

// SetResource updates the current resource display.
func (h *Header) SetResource(resource string) {
	h.resource = resource
	h.render()
}

// SetShortcuts sets the context-specific shortcuts.
func (h *Header) SetShortcuts(sc []Shortcut) {
	h.shortcutList = sc
	h.render()
}

// render redraws both panes.
func (h *Header) render() {
	h.renderInfo()
	h.renderShortcuts()
}

// renderInfo draws the left info pane.
func (h *Header) renderInfo() {
	var b strings.Builder

	b.WriteString(" [::b]awsc[::-]\n")

	if h.profile != "" {
		b.WriteString(fmt.Sprintf(" [gray]profile: [white]%s[-]\n", h.profile))
	} else {
		b.WriteString(" [gray]profile: [white]-[-]\n")
	}

	if h.region != "" {
		b.WriteString(fmt.Sprintf(" [gray]region:  [dodgerblue]%s[-]\n", h.region))
	} else {
		b.WriteString(" [gray]region:  [white]-[-]\n")
	}

	if h.resource != "" {
		b.WriteString(fmt.Sprintf(" [gray]view:    [white]%s[-]", h.resource))
	} else {
		b.WriteString(" [gray]view:    [white]-[-]")
	}

	h.infoPane.SetText(b.String())
}

// renderShortcuts draws the right shortcuts pane as a compact table.
func (h *Header) renderShortcuts() {
	if len(h.shortcutList) == 0 {
		h.shortcuts.SetText("")
		return
	}

	// Lay shortcuts out in 2 columns to fill the 4 available lines.
	rows := HeaderHeight
	cols := 2

	// Build cell grid: fill column-first.
	total := len(h.shortcutList)
	cells := make([]string, rows*cols)
	for i := range cells {
		cells[i] = ""
	}
	for i, sc := range h.shortcutList {
		col := i / rows
		row := i % rows
		if col >= cols {
			break
		}
		cells[row*cols+col] = fmt.Sprintf("[gold]<%s>[lightgray] %s", sc.Key, sc.Label)
	}

	// If there are more shortcuts than fit in 2 columns, add extras to last row.
	if total > rows*cols {
		var extra []string
		for i := rows * cols; i < total; i++ {
			extra = append(extra, fmt.Sprintf("[gold]<%s>[lightgray] %s", h.shortcutList[i].Key, h.shortcutList[i].Label))
		}
		cells[(rows-1)*cols+cols-1] += " " + strings.Join(extra, " ")
	}

	// Render rows with fixed-width columns.
	var b strings.Builder
	for row := 0; row < rows; row++ {
		if row > 0 {
			b.WriteString("\n")
		}
		for col := 0; col < cols; col++ {
			cell := cells[row*cols+col]
			if cell != "" {
				b.WriteString(fmt.Sprintf(" %-24s", cell))
			}
		}
	}

	h.shortcuts.SetText(b.String())
}
