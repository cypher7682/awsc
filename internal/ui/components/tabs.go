package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TabPage represents a single page in a TabbedView.
type TabPage struct {
	Name    string
	Content tview.Primitive
}

// TabbedView provides a multi-page view with left/right tab navigation.
// It renders a tab bar at the top showing all page names with the active
// page highlighted, and the active page content below.
type TabbedView struct {
	flex    *tview.Flex
	tabBar  *tview.TextView
	content *tview.Pages
	pages   []TabPage
	current int

	// extraInput handles view-specific keys beyond tab navigation.
	extraInput func(event *tcell.EventKey) *tcell.EventKey
}

// NewTabbedView creates a new tabbed view with the given pages.
func NewTabbedView(pages []TabPage) *TabbedView {
	tabBar := tview.NewTextView()
	tabBar.SetDynamicColors(true)
	tabBar.SetBackgroundColor(tcell.ColorDarkSlateGray)
	tabBar.SetTextAlign(tview.AlignLeft)

	content := tview.NewPages()
	for _, p := range pages {
		content.AddPage(p.Name, p.Content, true, false)
	}
	if len(pages) > 0 {
		content.SwitchToPage(pages[0].Name)
	}

	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(tabBar, 1, 0, false)
	flex.AddItem(content, 0, 1, true)

	tv := &TabbedView{
		flex:    flex,
		tabBar:  tabBar,
		content: content,
		pages:   pages,
		current: 0,
	}

	tv.renderTabBar()
	flex.SetInputCapture(tv.handleInput)

	return tv
}

// Widget returns the root primitive for embedding in layouts.
func (tv *TabbedView) Widget() tview.Primitive {
	return tv.flex
}

// SetExtraInput sets a handler for keys not consumed by tab navigation.
func (tv *TabbedView) SetExtraInput(fn func(event *tcell.EventKey) *tcell.EventKey) {
	tv.extraInput = fn
}

// CurrentPage returns the index of the active page.
func (tv *TabbedView) CurrentPage() int {
	return tv.current
}

// CurrentPageName returns the name of the active page.
func (tv *TabbedView) CurrentPageName() string {
	if tv.current < len(tv.pages) {
		return tv.pages[tv.current].Name
	}
	return ""
}

// SwitchTo activates the page at the given index.
func (tv *TabbedView) SwitchTo(idx int) {
	if idx < 0 || idx >= len(tv.pages) {
		return
	}
	tv.current = idx
	tv.content.SwitchToPage(tv.pages[idx].Name)
	tv.renderTabBar()
}

// Next moves to the next page (wraps around).
func (tv *TabbedView) Next() {
	tv.SwitchTo((tv.current + 1) % len(tv.pages))
}

// Prev moves to the previous page (wraps around).
func (tv *TabbedView) Prev() {
	idx := tv.current - 1
	if idx < 0 {
		idx = len(tv.pages) - 1
	}
	tv.SwitchTo(idx)
}

// renderTabBar redraws the tab indicator bar.
func (tv *TabbedView) renderTabBar() {
	var parts []string
	for i, p := range tv.pages {
		if i == tv.current {
			parts = append(parts, fmt.Sprintf(" [black:dodgerblue:b] %s [-:-:-] ", p.Name))
		} else {
			parts = append(parts, fmt.Sprintf(" [gray::] %s [-::-] ", p.Name))
		}
	}
	tv.tabBar.SetText(strings.Join(parts, "[darkgray]|[-]"))
}

// handleInput processes left/right for tab switching, delegates rest.
func (tv *TabbedView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyLeft:
		tv.Prev()
		return nil
	case tcell.KeyRight:
		tv.Next()
		return nil
	}

	if tv.extraInput != nil {
		return tv.extraInput(event)
	}
	return event
}
