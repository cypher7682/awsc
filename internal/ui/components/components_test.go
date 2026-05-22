package components

import (
	"testing"
)

func TestNewHeader(t *testing.T) {
	h := NewHeader()
	if h == nil {
		t.Fatal("expected non-nil header")
	}
	if h.profile != "" {
		t.Errorf("expected empty profile, got '%s'", h.profile)
	}
}

func TestHeader_SetContext(t *testing.T) {
	h := NewHeader()
	h.SetContext("production", "eu-west-1")

	if h.profile != "production" {
		t.Errorf("expected profile 'production', got '%s'", h.profile)
	}
	if h.region != "eu-west-1" {
		t.Errorf("expected region 'eu-west-1', got '%s'", h.region)
	}
}

func TestHeader_SetResource(t *testing.T) {
	h := NewHeader()
	h.SetResource("ec2")

	if h.resource != "ec2" {
		t.Errorf("expected resource 'ec2', got '%s'", h.resource)
	}
}

func TestHeader_SetShortcuts(t *testing.T) {
	h := NewHeader()
	shortcuts := []Shortcut{
		{Key: "x", Label: "test"},
	}
	h.SetShortcuts(shortcuts)

	if len(h.shortcuts) != 1 {
		t.Errorf("expected 1 shortcut, got %d", len(h.shortcuts))
	}
}

func TestNewOmnibox(t *testing.T) {
	o := NewOmnibox()
	if o == nil {
		t.Fatal("expected non-nil omnibox")
	}
	if o.Mode() != OmniboxModeIdle {
		t.Errorf("expected idle mode, got %d", o.Mode())
	}
}

func TestOmnibox_Activate(t *testing.T) {
	o := NewOmnibox()

	o.Activate(OmniboxModeCommand)
	if o.Mode() != OmniboxModeCommand {
		t.Errorf("expected command mode, got %d", o.Mode())
	}

	o.Deactivate()
	if o.Mode() != OmniboxModeIdle {
		t.Errorf("expected idle mode, got %d", o.Mode())
	}
}

func TestOmnibox_Modes(t *testing.T) {
	o := NewOmnibox()

	modes := []OmniboxMode{OmniboxModeCommand, OmniboxModeFilter, OmniboxModeConfirm}
	for _, mode := range modes {
		o.Activate(mode)
		if o.Mode() != mode {
			t.Errorf("expected mode %d, got %d", mode, o.Mode())
		}
		o.Deactivate()
	}
}

func TestOmnibox_SetFields(t *testing.T) {
	o := NewOmnibox()
	fields := []string{"name", "state", "type"}
	o.SetFields(fields)

	if len(o.allFields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(o.allFields))
	}
}

func TestOmnibox_CommandAutocomplete(t *testing.T) {
	o := NewOmnibox()

	results := o.commandAutocomplete("ec")
	if len(results) == 0 {
		t.Error("expected autocomplete results for 'ec'")
	}
	// Should include ec2 and ecr
	found := map[string]bool{"ec2": false, "ecr": false}
	for _, r := range results {
		if _, ok := found[r]; ok {
			found[r] = true
		}
	}
	for cmd, f := range found {
		if !f {
			t.Errorf("expected '%s' in autocomplete results", cmd)
		}
	}
}

func TestOmnibox_CommandAutocomplete_Empty(t *testing.T) {
	o := NewOmnibox()
	results := o.commandAutocomplete("")
	if results != nil {
		t.Error("expected nil results for empty input")
	}
}

func TestOmnibox_FilterAutocomplete(t *testing.T) {
	o := NewOmnibox()
	o.SetFields([]string{"name", "state", "namespace"})

	results := o.filterAutocomplete("na")
	if len(results) == 0 {
		t.Error("expected autocomplete results for 'na'")
	}
}
