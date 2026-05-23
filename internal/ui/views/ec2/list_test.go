package ec2view

import (
	"testing"

	"github.com/tpriestnall/awsc/internal/aws/ec2"
)

func TestMatchesFilter_SimpleTextSearch(t *testing.T) {
	v := &ListView{}
	inst := ec2.Instance{
		InstanceID: "i-abc123",
		Name:       "web-server-prod",
		State:      "running",
		Type:       "t3.micro",
		PrivateIP:  "10.0.1.100",
		PublicIP:   "54.1.2.3",
	}

	tests := []struct {
		filter   string
		expected bool
	}{
		{"web", true},
		{"abc123", true},
		{"running", true},
		{"t3.micro", true},
		{"10.0.1", true},
		{"54.1", true},
		{"nonexistent", false},
		{"stopped", false},
	}

	for _, tt := range tests {
		got := v.matchesFilter(inst, tt.filter)
		if got != tt.expected {
			t.Errorf("matchesFilter(%q): expected %v, got %v", tt.filter, tt.expected, got)
		}
	}
}

func TestMatchesFilter_FieldOperatorValue(t *testing.T) {
	v := &ListView{}
	inst := ec2.Instance{
		InstanceID: "i-abc123",
		Name:       "web-server-prod",
		State:      "running",
		Type:       "t3.micro",
		PrivateIP:  "10.0.1.100",
		PublicIP:   "54.1.2.3",
		VPCID:      "vpc-xyz",
		AZ:         "us-east-1a",
		Tags:       map[string]string{"env": "production", "team": "platform"},
	}

	tests := []struct {
		filter   string
		expected bool
	}{
		{"name = web-server-prod", true},
		{"name = wrong-name", false},
		{"name contains web", true},
		{"name contains api", false},
		{"name starts_with web", true},
		{"name starts_with api", false},
		{"name ends_with prod", true},
		{"name ends_with dev", false},
		{"state = running", true},
		{"state != running", false},
		{"state = stopped", false},
		{"type = t3.micro", true},
		{"vpc = vpc-xyz", true},
		{"az = us-east-1a", true},
		{"tag:env = production", true},
		{"tag:env = staging", false},
		{"tag:team contains plat", true},
	}

	for _, tt := range tests {
		got := v.matchesFilter(inst, tt.filter)
		if got != tt.expected {
			t.Errorf("matchesFilter(%q): expected %v, got %v", tt.filter, tt.expected, got)
		}
	}
}

func TestOrDash(t *testing.T) {
	if orDash("") != "-" {
		t.Error("expected '-' for empty string")
	}
	if orDash("hello") != "hello" {
		t.Error("expected 'hello' for non-empty string")
	}
}

func TestStateColor(t *testing.T) {
	// Just verify it doesn't panic for various states
	states := []string{"running", "stopped", "terminated", "shutting-down", "pending", "stopping", "unknown"}
	for _, s := range states {
		_ = stateColor(s)
	}
}

func TestListView_Name(t *testing.T) {
	v := &ListView{}
	if v.Name() != "ec2" {
		t.Errorf("expected 'ec2', got '%s'", v.Name())
	}
}

func TestListView_FilterFields(t *testing.T) {
	v := &ListView{}
	fields := v.FilterFields()
	if len(fields) == 0 {
		t.Error("expected filter fields")
	}
	// Check some expected fields
	expected := map[string]bool{
		"instance_id": false,
		"name":        false,
		"state":       false,
		"type":        false,
	}
	for _, f := range fields {
		if _, ok := expected[f]; ok {
			expected[f] = true
		}
	}
	for field, found := range expected {
		if !found {
			t.Errorf("expected field '%s' in filter fields", field)
		}
	}
}

func TestListView_Shortcuts(t *testing.T) {
	v := &ListView{}
	shortcuts := v.Shortcuts()
	if len(shortcuts) == 0 {
		t.Error("expected shortcuts")
	}
	// Should have terminate(Del), reboot(r), stop(x), start(a), sort-by(s), sort-dir(d), refresh(R)
	keys := make(map[string]bool)
	for _, s := range shortcuts {
		keys[s.Key] = true
	}
	expectedKeys := []string{"Del", "r", "x", "a", "s", "d", "R"}
	for _, k := range expectedKeys {
		if !keys[k] {
			t.Errorf("expected shortcut key '%s'", k)
		}
	}
}
