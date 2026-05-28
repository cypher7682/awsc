// Package components provides reusable UI components.
package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PickerOption represents a single option in the picker.
type PickerOption struct {
	Key         string // Short key (e.g., "s" for start)
	Label       string // Display label
	Description string // Optional description
	Disabled    bool   // Greyed out if true
	Color       tcell.Color
}

// Picker is a modal popup for selecting from a list of options.
type Picker struct {
	*tview.Box
	options    []PickerOption
	selected   int
	title      string
	onSelect   func(option PickerOption)
	onCancel   func()
	visible    bool
	width      int
	height     int
}

// NewPicker creates a new picker with the given title and options.
func NewPicker(title string, options []PickerOption) *Picker {
	p := &Picker{
		Box:      tview.NewBox(),
		options:  options,
		title:    title,
		selected: 0,
		width:    40,
		height:   len(options) + 4, // title + border + options + help line
	}

	// Find first non-disabled option
	for i, opt := range options {
		if !opt.Disabled {
			p.selected = i
			break
		}
	}

	p.SetBorder(true)
	p.SetBorderColor(tcell.ColorDodgerBlue)
	p.SetTitle(" " + title + " ")
	p.SetTitleColor(tcell.ColorWhite)
	p.SetBackgroundColor(tcell.ColorBlack)

	return p
}

// SetOnSelect sets the callback when an option is selected.
func (p *Picker) SetOnSelect(fn func(option PickerOption)) *Picker {
	p.onSelect = fn
	return p
}

// SetOnCancel sets the callback when picker is cancelled.
func (p *Picker) SetOnCancel(fn func()) *Picker {
	p.onCancel = fn
	return p
}

// Show makes the picker visible.
func (p *Picker) Show() {
	p.visible = true
}

// Hide makes the picker invisible.
func (p *Picker) Hide() {
	p.visible = false
}

// IsVisible returns whether the picker is visible.
func (p *Picker) IsVisible() bool {
	return p.visible
}

// Draw renders the picker.
func (p *Picker) Draw(screen tcell.Screen) {
	if !p.visible {
		return
	}

	// Calculate centered position
	screenWidth, screenHeight := screen.Size()
	x := (screenWidth - p.width) / 2
	y := (screenHeight - p.height) / 2

	p.SetRect(x, y, p.width, p.height)
	p.Box.DrawForSubclass(screen, p)

	// Draw options
	innerX := x + 2
	innerY := y + 1

	for i, opt := range p.options {
		style := tcell.StyleDefault.Background(tcell.ColorBlack)

		if opt.Disabled {
			style = style.Foreground(tcell.ColorDarkGray)
		} else if i == p.selected {
			style = style.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite)
		} else if opt.Color != 0 {
			style = style.Foreground(opt.Color)
		} else {
			style = style.Foreground(tcell.ColorWhite)
		}

		// Draw key hint
		keyStyle := style
		if !opt.Disabled && i != p.selected {
			keyStyle = style.Foreground(tcell.ColorYellow)
		}

		line := ""
		if opt.Key != "" {
			line = "[" + opt.Key + "] "
		}
		line += opt.Label

		// Pad to width
		for len(line) < p.width-4 {
			line += " "
		}

		// Draw character by character
		for j, r := range line {
			if j < 4 && opt.Key != "" { // Key hint portion
				screen.SetContent(innerX+j, innerY+i, r, nil, keyStyle)
			} else {
				screen.SetContent(innerX+j, innerY+i, r, nil, style)
			}
		}
	}

	// Draw help line at bottom
	helpY := y + p.height - 2
	helpText := "↑/↓:navigate  Enter:select  Esc:cancel"
	helpStyle := tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)
	for i, r := range helpText {
		if innerX+i < x+p.width-1 {
			screen.SetContent(innerX+i, helpY, r, nil, helpStyle)
		}
	}
}

// InputHandler handles keyboard input.
func (p *Picker) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return p.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if !p.visible {
			return
		}

		switch event.Key() {
		case tcell.KeyUp:
			p.moveSelection(-1)
		case tcell.KeyDown:
			p.moveSelection(1)
		case tcell.KeyEnter:
			if p.selected >= 0 && p.selected < len(p.options) {
				opt := p.options[p.selected]
				if !opt.Disabled && p.onSelect != nil {
					p.Hide()
					p.onSelect(opt)
				}
			}
		case tcell.KeyEscape:
			p.Hide()
			if p.onCancel != nil {
				p.onCancel()
			}
		case tcell.KeyRune:
			// Check for key shortcuts
			key := string(event.Rune())
			for i, opt := range p.options {
				if opt.Key == key && !opt.Disabled {
					p.selected = i
					if p.onSelect != nil {
						p.Hide()
						p.onSelect(opt)
					}
					return
				}
			}
			// Also handle j/k for vim navigation
			switch event.Rune() {
			case 'j':
				p.moveSelection(1)
			case 'k':
				p.moveSelection(-1)
			}
		}
	})
}

// moveSelection moves the selection by delta, skipping disabled options.
func (p *Picker) moveSelection(delta int) {
	if len(p.options) == 0 {
		return
	}

	start := p.selected
	for {
		p.selected += delta
		if p.selected < 0 {
			p.selected = len(p.options) - 1
		} else if p.selected >= len(p.options) {
			p.selected = 0
		}

		// Found a non-disabled option
		if !p.options[p.selected].Disabled {
			return
		}

		// Wrapped around without finding anything
		if p.selected == start {
			return
		}
	}
}

// MouseHandler handles mouse input.
func (p *Picker) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return p.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		if !p.visible {
			return false, nil
		}

		x, y := event.Position()
		rectX, rectY, width, height := p.GetRect()

		// Check if click is within picker bounds
		if x >= rectX && x < rectX+width && y >= rectY && y < rectY+height {
			// Calculate which option was clicked
			optionY := y - rectY - 1 // -1 for top border
			if optionY >= 0 && optionY < len(p.options) {
				if action == tview.MouseLeftClick {
					opt := p.options[optionY]
					if !opt.Disabled {
						p.selected = optionY
						if p.onSelect != nil {
							p.Hide()
							p.onSelect(opt)
						}
					}
				}
			}
			return true, nil
		}

		// Click outside - cancel
		if action == tview.MouseLeftClick {
			p.Hide()
			if p.onCancel != nil {
				p.onCancel()
			}
			return true, nil
		}

		return false, nil
	})
}
