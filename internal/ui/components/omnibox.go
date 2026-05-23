package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// FilterSuggestion represents an autocomplete suggestion.
type FilterSuggestion struct {
	Field    string // e.g., "instance_id", "name", "security_group"
	Operator string // e.g., "=", "contains", "starts_with"
	Example  string // e.g., "i-abc123"
}

// OmniboxMode defines the current mode of the omnibox.
type OmniboxMode int

const (
	OmniboxModeIdle    OmniboxMode = iota // Showing status
	OmniboxModeCommand                    // Typing a :command
	OmniboxModeFilter                     // Typing a /filter
	OmniboxModeConfirm                    // Confirming an action
)

// OmniboxHandler handles omnibox events.
type OmniboxHandler interface {
	OnCommand(command string)
	OnFilter(filter string)
	OnConfirm(confirmed bool)
}

// Omnibox is the bottom bar that handles commands, filtering, and status.
// It uses a single InputField that's always rendered in the widget tree,
// switching between a read-only status display and an editable input mode.
type Omnibox struct {
	*tview.InputField
	mode        OmniboxMode
	handler     OmniboxHandler
	suggestions []FilterSuggestion
	allFields   []string
	statusText  string
	profiles    []string // available AWS profiles
	regions     []string // available AWS regions
}

// NewOmnibox creates a new omnibox component.
func NewOmnibox() *Omnibox {
	input := tview.NewInputField()
	input.SetBackgroundColor(tcell.ColorDarkSlateGray)
	input.SetFieldBackgroundColor(tcell.ColorDarkSlateGray)
	input.SetFieldTextColor(tcell.ColorWhite)
	input.SetLabelColor(tcell.ColorGray)
	input.SetPlaceholderTextColor(tcell.ColorDarkGray)

	o := &Omnibox{
		InputField: input,
		mode:       OmniboxModeIdle,
		allFields: []string{
			"instance_id", "name", "state", "type", "private_ip",
			"public_ip", "vpc_id", "subnet_id", "security_group",
			"key_name", "ami", "az", "platform", "tag:",
		},
	}

	o.SetStatus("Ready")
	return o
}

// SetProfiles sets the available AWS profiles for command autocomplete.
func (o *Omnibox) SetProfiles(profiles []string) {
	o.profiles = profiles
}

// SetRegions sets the available AWS regions for command autocomplete.
func (o *Omnibox) SetRegions(regions []string) {
	o.regions = regions
}

// SetHandler sets the event handler for omnibox actions.
func (o *Omnibox) SetHandler(handler OmniboxHandler) {
	o.handler = handler
}

// SetStatus updates the status text shown when idle.
func (o *Omnibox) SetStatus(text string) {
	o.statusText = text
	if o.mode == OmniboxModeIdle {
		o.InputField.SetLabel(fmt.Sprintf(" [gray]%s", text))
		o.InputField.SetText("")
		o.InputField.SetFieldWidth(0)
		o.InputField.SetPlaceholder("")
		o.InputField.SetAutocompleteFunc(nil)
	}
}

// SetFields sets the available filter fields for autocomplete.
func (o *Omnibox) SetFields(fields []string) {
	o.allFields = fields
}

// Activate switches the omnibox to input mode.
func (o *Omnibox) Activate(mode OmniboxMode) {
	o.mode = mode
	o.InputField.SetText("")
	o.InputField.SetFieldWidth(0) // 0 = fill available space
	o.InputField.SetAutocompleteFunc(nil) // we use our own popup

	switch mode {
	case OmniboxModeCommand:
		o.InputField.SetLabel("[dodgerblue::b]: [-::-]")
		o.InputField.SetPlaceholder("ec2, ecr, services, r=eu-west-1, p=nexmo-dev, quit")
	case OmniboxModeFilter:
		o.InputField.SetLabel("[dodgerblue::b]/ [-::-]")
		o.InputField.SetPlaceholder("name contains web, state = running, ...")
	case OmniboxModeConfirm:
		o.InputField.SetLabel("[red::b]Confirm (y/N): [-::-]")
		o.InputField.SetPlaceholder("")
	}
}

// Deactivate returns the omnibox to idle mode.
func (o *Omnibox) Deactivate() {
	o.mode = OmniboxModeIdle
	o.InputField.SetText("")
	o.InputField.SetPlaceholder("")
	// Restore status display
	o.InputField.SetLabel(fmt.Sprintf(" [gray]%s", o.statusText))
	o.InputField.SetFieldWidth(0)
}

// Mode returns the current omnibox mode.
func (o *Omnibox) Mode() OmniboxMode {
	return o.mode
}

// Input returns the input field for focus management.
func (o *Omnibox) Input() *tview.InputField {
	return o.InputField
}

// HandleInput processes the current input when Enter is pressed.
func (o *Omnibox) HandleInput() {
	text := strings.TrimSpace(o.InputField.GetText())
	if text == "" {
		// Empty submit in filter mode clears the filter
		if o.mode == OmniboxModeFilter && o.handler != nil {
			o.handler.OnFilter("")
		}
		o.Deactivate()
		return
	}

	switch o.mode {
	case OmniboxModeCommand:
		if o.handler != nil {
			o.handler.OnCommand(text)
		}
	case OmniboxModeFilter:
		if o.handler != nil {
			o.handler.OnFilter(text)
		}
	case OmniboxModeConfirm:
		confirmed := strings.ToLower(text) == "y" || strings.ToLower(text) == "yes"
		if o.handler != nil {
			o.handler.OnConfirm(confirmed)
		}
	}

	o.Deactivate()
}

// SetConfirmPrompt sets the omnibox to confirmation mode with a custom prompt.
func (o *Omnibox) SetConfirmPrompt(prompt string) {
	o.Activate(OmniboxModeConfirm)
	o.InputField.SetLabel(fmt.Sprintf("[red::b]%s (y/N): [-::-]", prompt))
}

// GetCompletions returns the popup completion items for the current text.
// Returns non-nil only for profile=/p= and region=/r= prefixes (where a popup makes sense).
func (o *Omnibox) GetCompletions(currentText string) []string {
	if currentText == "" {
		return nil
	}

	lower := strings.ToLower(currentText)

	// Profile completions (full or shorthand)
	var profilePrefix string
	var profileLabel string
	if strings.HasPrefix(lower, "profile=") {
		profilePrefix = strings.TrimPrefix(lower, "profile=")
		profileLabel = "profile="
	} else if strings.HasPrefix(lower, "p=") {
		profilePrefix = strings.TrimPrefix(lower, "p=")
		profileLabel = "p="
	}
	if profileLabel != "" {
		var matches []string
		for _, p := range o.profiles {
			if profilePrefix == "" || strings.Contains(strings.ToLower(p), profilePrefix) {
				matches = append(matches, profileLabel+p)
			}
		}
		sort.Strings(matches)
		return matches
	}

	// Region completions (full or shorthand)
	var regionPrefix string
	var regionLabel string
	if strings.HasPrefix(lower, "region=") {
		regionPrefix = strings.TrimPrefix(lower, "region=")
		regionLabel = "region="
	} else if strings.HasPrefix(lower, "r=") {
		regionPrefix = strings.TrimPrefix(lower, "r=")
		regionLabel = "r="
	}
	if regionLabel != "" {
		var matches []string
		for _, r := range o.regions {
			if regionPrefix == "" || strings.Contains(r, regionPrefix) {
				matches = append(matches, regionLabel+r)
			}
		}
		sort.Strings(matches)
		return matches
	}

	return nil
}

// commandAutocomplete provides autocomplete for commands.
func (o *Omnibox) commandAutocomplete(currentText string) []string {
	if currentText == "" {
		return nil
	}

	lower := strings.ToLower(currentText)

	// If user is typing "profile=..." or "p=...", suggest profiles
	var profilePrefix string
	var profileLabel string
	if strings.HasPrefix(lower, "profile=") {
		profilePrefix = strings.TrimPrefix(lower, "profile=")
		profileLabel = "profile="
	} else if strings.HasPrefix(lower, "p=") {
		profilePrefix = strings.TrimPrefix(lower, "p=")
		profileLabel = "p="
	}
	if profileLabel != "" {
		var matches []string
		for _, p := range o.profiles {
			if profilePrefix == "" || strings.HasPrefix(strings.ToLower(p), profilePrefix) {
				matches = append(matches, profileLabel+p)
			}
		}
		sort.Strings(matches)
		return matches
	}

	// If user is typing "region=..." or "r=...", suggest regions
	var regionPrefix string
	var regionLabel string
	if strings.HasPrefix(lower, "region=") {
		regionPrefix = strings.TrimPrefix(lower, "region=")
		regionLabel = "region="
	} else if strings.HasPrefix(lower, "r=") {
		regionPrefix = strings.TrimPrefix(lower, "r=")
		regionLabel = "r="
	}
	if regionLabel != "" {
		var matches []string
		for _, r := range o.regions {
			if regionPrefix == "" || strings.HasPrefix(r, regionPrefix) {
				matches = append(matches, regionLabel+r)
			}
		}
		sort.Strings(matches)
		return matches
	}

	// Base command suggestions
	commands := []string{
		"ec2", "ecr", "services", "sg", "vpc", "subnet",
		"region", "profile", "quit", "help",
	}

	var matches []string
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, lower) {
			matches = append(matches, cmd)
		}
	}

	// Also suggest "profile=", "p=", "region=", "r=" if partially matched
	if strings.HasPrefix("profile=", lower) && lower != "profile=" {
		matches = append(matches, "profile=")
	}
	if strings.HasPrefix("p=", lower) && lower != "p=" {
		matches = append(matches, "p=")
	}
	if strings.HasPrefix("region=", lower) && lower != "region=" {
		matches = append(matches, "region=")
	}
	if strings.HasPrefix("r=", lower) && lower != "r=" {
		matches = append(matches, "r=")
	}

	sort.Strings(matches)
	return matches
}

// filterAutocomplete provides autocomplete for filter expressions.
func (o *Omnibox) filterAutocomplete(currentText string) []string {
	if currentText == "" {
		return nil
	}

	parts := strings.Fields(currentText)
	lastPart := parts[len(parts)-1]
	lower := strings.ToLower(lastPart)

	// If we're at the field name stage
	if !strings.Contains(lastPart, "=") && !strings.Contains(lastPart, " ") {
		var matches []string
		for _, field := range o.allFields {
			if strings.HasPrefix(field, lower) {
				matches = append(matches, field)
			}
		}
		sort.Strings(matches)
		return matches
	}

	// If we have a field, suggest operators
	operators := []string{"=", "!=", "contains", "starts_with", "ends_with"}
	var matches []string
	for _, op := range operators {
		if strings.HasPrefix(op, lower) {
			matches = append(matches, op)
		}
	}
	return matches
}
