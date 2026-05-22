package ecrview

import (
	"testing"
)

func TestListView_Name(t *testing.T) {
	v := &ListView{}
	if v.Name() != "ecr" {
		t.Errorf("expected 'ecr', got '%s'", v.Name())
	}
}

func TestListView_Shortcuts(t *testing.T) {
	v := &ListView{}
	shortcuts := v.Shortcuts()
	if len(shortcuts) == 0 {
		t.Error("expected shortcuts")
	}
}

func TestListView_FilterFields(t *testing.T) {
	v := &ListView{}
	fields := v.FilterFields()
	if len(fields) == 0 {
		t.Error("expected filter fields")
	}
	// Should include "name"
	found := false
	for _, f := range fields {
		if f == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'name' in filter fields")
	}
}

func TestImageView_Name(t *testing.T) {
	v := &ImageView{}
	if v.Name() != "ecr-detail" {
		t.Errorf("expected 'ecr-detail', got '%s'", v.Name())
	}
}

func TestImageView_Shortcuts(t *testing.T) {
	v := &ImageView{}
	shortcuts := v.Shortcuts()
	if len(shortcuts) == 0 {
		t.Error("expected shortcuts")
	}
}

func TestImageView_FilterFields(t *testing.T) {
	v := &ImageView{}
	fields := v.FilterFields()
	if len(fields) == 0 {
		t.Error("expected filter fields")
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
