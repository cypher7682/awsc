package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CompletionList is a popup list that appears above the omnibox for
// selecting from a set of completions. Up/Down navigate, Enter selects,
// typing refines, Escape dismisses.
type CompletionList struct {
	list    *tview.List
	visible bool
	items   []string // current filtered items
	onPick  func(text string)
}

// NewCompletionList creates a new completion popup.
func NewCompletionList() *CompletionList {
	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetHighlightFullLine(true)
	list.SetBorder(true)
	list.SetBorderColor(tcell.ColorDodgerBlue)
	list.SetBackgroundColor(tcell.ColorDarkSlateGray)
	list.SetMainTextColor(tcell.ColorWhite)
	list.SetSelectedTextColor(tcell.ColorWhite)
	list.SetSelectedBackgroundColor(tcell.ColorDodgerBlue)
	list.SetTitle(" completions ")
	list.SetTitleColor(tcell.ColorGray)

	return &CompletionList{
		list: list,
	}
}

// SetOnPick sets the callback fired when the user selects an item.
func (c *CompletionList) SetOnPick(fn func(text string)) {
	c.onPick = fn
}

// Widget returns the underlying tview primitive for layout embedding.
func (c *CompletionList) Widget() tview.Primitive {
	return c.list
}

// Visible returns whether the completion list is currently showing.
func (c *CompletionList) Visible() bool {
	return c.visible
}

// Show populates the list with items and makes it visible.
// If items is empty, the list is hidden instead.
func (c *CompletionList) Show(items []string) {
	c.items = items
	if len(items) == 0 {
		c.Hide()
		return
	}

	c.list.Clear()
	for _, item := range items {
		c.list.AddItem(item, "", 0, nil)
	}
	c.list.SetCurrentItem(0)
	c.visible = true
}

// Hide dismisses the completion list.
func (c *CompletionList) Hide() {
	c.visible = false
	c.items = nil
	c.list.Clear()
}

// MoveUp moves the selection up one item.
func (c *CompletionList) MoveUp() {
	if !c.visible || c.list.GetItemCount() == 0 {
		return
	}
	curr := c.list.GetCurrentItem()
	if curr > 0 {
		c.list.SetCurrentItem(curr - 1)
	} else {
		// Wrap to bottom
		c.list.SetCurrentItem(c.list.GetItemCount() - 1)
	}
}

// MoveDown moves the selection down one item.
func (c *CompletionList) MoveDown() {
	if !c.visible || c.list.GetItemCount() == 0 {
		return
	}
	curr := c.list.GetCurrentItem()
	if curr < c.list.GetItemCount()-1 {
		c.list.SetCurrentItem(curr + 1)
	} else {
		// Wrap to top
		c.list.SetCurrentItem(0)
	}
}

// Accept selects the current item, calls onPick, and hides the list.
func (c *CompletionList) Accept() bool {
	if !c.visible || c.list.GetItemCount() == 0 {
		return false
	}
	idx := c.list.GetCurrentItem()
	if idx < 0 || idx >= len(c.items) {
		return false
	}
	text := c.items[idx]
	c.Hide()
	if c.onPick != nil {
		c.onPick(text)
	}
	return true
}

// DesiredHeight returns the height the list wants (capped at maxHeight).
func (c *CompletionList) DesiredHeight(maxHeight int) int {
	if !c.visible {
		return 0
	}
	h := len(c.items) + 2 // +2 for border
	if h > maxHeight {
		h = maxHeight
	}
	if h < 3 {
		h = 3
	}
	return h
}
