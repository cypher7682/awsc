package services

import (
	"context"
	"testing"

	"github.com/tpriestnall/awsc/internal/navigation"
)

type mockNavigator struct {
	lastRoute  navigation.Route
	lastStatus string
}

func (m *mockNavigator) Navigate(route navigation.Route) {
	m.lastRoute = route
}

func (m *mockNavigator) SetStatus(text string) {
	m.lastStatus = text
}

func TestNewView(t *testing.T) {
	nav := &mockNavigator{}
	v := NewView(nav)

	if v.Name() != "services" {
		t.Errorf("expected name 'services', got '%s'", v.Name())
	}
}

func TestView_Refresh(t *testing.T) {
	nav := &mockNavigator{}
	v := NewView(nav)

	err := v.Refresh(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Table should have rows
	rowCount := v.table.GetRowCount()
	if rowCount == 0 {
		t.Error("expected table to have rows after refresh")
	}
}

func TestView_Shortcuts(t *testing.T) {
	nav := &mockNavigator{}
	v := NewView(nav)

	shortcuts := v.Shortcuts()
	if len(shortcuts) == 0 {
		t.Error("expected at least one shortcut")
	}
}

func TestView_FilterFields(t *testing.T) {
	nav := &mockNavigator{}
	v := NewView(nav)

	fields := v.FilterFields()
	if len(fields) == 0 {
		t.Error("expected at least one filter field")
	}
}

func TestView_Render(t *testing.T) {
	nav := &mockNavigator{}
	v := NewView(nav)

	primitive := v.Render()
	if primitive == nil {
		t.Error("expected non-nil primitive from Render")
	}
}
