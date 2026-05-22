// Package theme provides consistent styling for the awsc TUI.
package theme

import "github.com/gdamore/tcell/v2"

// Colors defines the color palette for the application.
var Colors = struct {
	// Primary colors
	Background    tcell.Color
	Foreground    tcell.Color
	Border        tcell.Color
	BorderFocused tcell.Color

	// Header
	HeaderBg      tcell.Color
	HeaderFg      tcell.Color
	ShortcutKey   tcell.Color
	ShortcutLabel tcell.Color

	// Table
	TableHeader   tcell.Color
	TableSelected tcell.Color
	TableSelectedFg tcell.Color

	// Status indicators
	Running    tcell.Color
	Stopped    tcell.Color
	Terminated tcell.Color
	Pending    tcell.Color
	Warning    tcell.Color

	// Omnibox
	OmniboxBg     tcell.Color
	OmniboxFg     tcell.Color
	OmniboxPrompt tcell.Color
	OmniboxHint   tcell.Color

	// Info
	InfoLabel tcell.Color
	InfoValue tcell.Color
}{
	Background:    tcell.ColorDefault,
	Foreground:    tcell.ColorWhite,
	Border:        tcell.ColorGray,
	BorderFocused: tcell.ColorDodgerBlue,

	HeaderBg:      tcell.ColorDarkSlateGray,
	HeaderFg:      tcell.ColorWhite,
	ShortcutKey:   tcell.ColorGold,
	ShortcutLabel: tcell.ColorLightGray,

	TableHeader:     tcell.ColorDodgerBlue,
	TableSelected:   tcell.ColorDodgerBlue,
	TableSelectedFg: tcell.ColorWhite,

	Running:    tcell.ColorGreen,
	Stopped:    tcell.ColorRed,
	Terminated: tcell.ColorDarkGray,
	Pending:    tcell.ColorYellow,
	Warning:    tcell.ColorOrange,

	OmniboxBg:     tcell.ColorDarkSlateGray,
	OmniboxFg:     tcell.ColorWhite,
	OmniboxPrompt: tcell.ColorDodgerBlue,
	OmniboxHint:   tcell.ColorGray,

	InfoLabel: tcell.ColorDodgerBlue,
	InfoValue: tcell.ColorWhite,
}

// StateColor returns the appropriate color for an instance state.
func StateColor(state string) tcell.Color {
	switch state {
	case "running":
		return Colors.Running
	case "stopped":
		return Colors.Stopped
	case "terminated", "shutting-down":
		return Colors.Terminated
	case "pending", "stopping":
		return Colors.Pending
	default:
		return Colors.Foreground
	}
}
