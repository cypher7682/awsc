package navigation

import (
	"testing"
)

func TestNewStack(t *testing.T) {
	s := NewStack()
	if s.Current().Resource != "services" {
		t.Errorf("expected initial route 'services', got '%s'", s.Current().Resource)
	}
	if s.Depth() != 1 {
		t.Errorf("expected depth 1, got %d", s.Depth())
	}
}

func TestStack_Push(t *testing.T) {
	s := NewStack()
	s.Push(Route{Resource: "ec2"})

	if s.Current().Resource != "ec2" {
		t.Errorf("expected 'ec2', got '%s'", s.Current().Resource)
	}
	if s.Depth() != 2 {
		t.Errorf("expected depth 2, got %d", s.Depth())
	}
}

func TestStack_Back(t *testing.T) {
	s := NewStack()
	s.Push(Route{Resource: "ec2"})
	s.Push(Route{Resource: "ec2", ResourceID: "i-123"})

	ok := s.Back()
	if !ok {
		t.Error("expected Back to return true")
	}
	if s.Current().Resource != "ec2" {
		t.Errorf("expected 'ec2', got '%s'", s.Current().Resource)
	}

	ok = s.Back()
	if !ok {
		t.Error("expected Back to return true")
	}
	if s.Current().Resource != "services" {
		t.Errorf("expected 'services', got '%s'", s.Current().Resource)
	}

	ok = s.Back()
	if ok {
		t.Error("expected Back to return false at beginning")
	}
}

func TestStack_Forward(t *testing.T) {
	s := NewStack()
	s.Push(Route{Resource: "ec2"})
	s.Push(Route{Resource: "ecr"})
	s.Back()
	s.Back()

	ok := s.Forward()
	if !ok {
		t.Error("expected Forward to return true")
	}
	if s.Current().Resource != "ec2" {
		t.Errorf("expected 'ec2', got '%s'", s.Current().Resource)
	}

	ok = s.Forward()
	if !ok {
		t.Error("expected Forward to return true")
	}
	if s.Current().Resource != "ecr" {
		t.Errorf("expected 'ecr', got '%s'", s.Current().Resource)
	}

	ok = s.Forward()
	if ok {
		t.Error("expected Forward to return false at end")
	}
}

func TestStack_PushDiscardsForwardHistory(t *testing.T) {
	s := NewStack()
	s.Push(Route{Resource: "ec2"})
	s.Push(Route{Resource: "ecr"})
	s.Back() // now at ec2

	// Push a new route, should discard "ecr" from forward history
	s.Push(Route{Resource: "vpc"})

	if s.Current().Resource != "vpc" {
		t.Errorf("expected 'vpc', got '%s'", s.Current().Resource)
	}
	if s.CanGoForward() {
		t.Error("expected no forward history after push")
	}
	if s.Depth() != 3 {
		t.Errorf("expected depth 3, got %d", s.Depth())
	}
}

func TestStack_CanGoBack(t *testing.T) {
	s := NewStack()
	if s.CanGoBack() {
		t.Error("expected CanGoBack to be false initially")
	}
	s.Push(Route{Resource: "ec2"})
	if !s.CanGoBack() {
		t.Error("expected CanGoBack to be true after push")
	}
}

func TestStack_Breadcrumb(t *testing.T) {
	s := NewStack()
	s.Push(Route{Resource: "ec2"})
	s.Push(Route{Resource: "ec2", ResourceID: "i-123"})

	crumbs := s.Breadcrumb()
	expected := []string{"services", "ec2", "ec2/i-123"}
	if len(crumbs) != len(expected) {
		t.Fatalf("expected %d breadcrumbs, got %d", len(expected), len(crumbs))
	}
	for i, c := range crumbs {
		if c != expected[i] {
			t.Errorf("breadcrumb[%d]: expected '%s', got '%s'", i, expected[i], c)
		}
	}
}

func TestRoute_String(t *testing.T) {
	tests := []struct {
		route    Route
		expected string
	}{
		{Route{Resource: "ec2"}, "ec2"},
		{Route{Resource: "ec2", ResourceID: "i-123"}, "ec2/i-123"},
		{Route{Resource: "services"}, "services"},
	}

	for _, tt := range tests {
		got := tt.route.String()
		if got != tt.expected {
			t.Errorf("Route.String(): expected '%s', got '%s'", tt.expected, got)
		}
	}
}

func TestCommandRegistry_Resolve(t *testing.T) {
	cr := NewCommandRegistry()

	route, ok := cr.Resolve("ec2")
	if !ok {
		t.Error("expected ec2 command to resolve")
	}
	if route.Resource != "ec2" {
		t.Errorf("expected resource 'ec2', got '%s'", route.Resource)
	}

	route, ok = cr.Resolve("ecr")
	if !ok {
		t.Error("expected ecr command to resolve")
	}
	if route.Resource != "ecr" {
		t.Errorf("expected resource 'ecr', got '%s'", route.Resource)
	}

	_, ok = cr.Resolve("nonexistent")
	if ok {
		t.Error("expected nonexistent command to not resolve")
	}
}

func TestCommandRegistry_Register(t *testing.T) {
	cr := NewCommandRegistry()
	cr.Register("lambda", func(_ string) Route {
		return Route{Resource: "lambda"}
	})

	route, ok := cr.Resolve("lambda")
	if !ok {
		t.Error("expected lambda command to resolve")
	}
	if route.Resource != "lambda" {
		t.Errorf("expected resource 'lambda', got '%s'", route.Resource)
	}
}

func TestCommandRegistry_AvailableCommands(t *testing.T) {
	cr := NewCommandRegistry()
	cmds := cr.AvailableCommands()

	// Should have at least the default commands
	if len(cmds) < 5 {
		t.Errorf("expected at least 5 commands, got %d", len(cmds))
	}

	// Check that ec2 is in the list
	found := false
	for _, cmd := range cmds {
		if cmd == "ec2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'ec2' to be in available commands")
	}
}
