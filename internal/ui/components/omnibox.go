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

	switch mode {
	case OmniboxModeCommand:
		o.InputField.SetLabel("[dodgerblue::b]: [-::-]")
		o.InputField.SetPlaceholder("ec2, ecr, services, region=us-east-1, quit")
		o.InputField.SetAutocompleteFunc(o.commandAutocomplete)
	case OmniboxModeFilter:
		o.InputField.SetLabel("[dodgerblue::b]/ [-::-]")
		o.InputField.SetPlaceholder("name contains web, state = running, ...")
		o.InputField.SetAutocompleteFunc(o.filterAutocomplete)
	case OmniboxModeConfirm:
		o.InputField.SetLabel("[red::b]Confirm (y/N): [-::-]")
		o.InputField.SetPlaceholder("")
		o.InputField.SetAutocompleteFunc(nil)
	}
}

// Deactivate returns the omnibox to idle mode.
func (o *Omnibox) Deactivate() {
	o.mode = OmniboxModeIdle
	o.InputField.SetText("")
	o.InputField.SetAutocompleteFunc(nil)
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

// commandAutocomplete provides autocomplete for commands.
func (o *Omnibox) commandAutocomplete(currentText string) []string {
	if currentText == "" {
		return nil
	}

	commands := []string{
		"ec2", "ecr", "services", "sg", "vpc", "subnet",
		"region", "profile", "quit", "help",
	}

	var matches []string
	lower := strings.ToLower(currentText)
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, lower) {
			matches = append(matches, cmd)
		}
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
