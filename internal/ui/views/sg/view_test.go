package sgview

import "testing"

func TestListView_Name(t *testing.T) {
	v := &ListView{}
	if v.Name() != "sg" {
		t.Errorf("expected 'sg', got '%s'", v.Name())
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
}
