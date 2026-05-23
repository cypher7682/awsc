package components

import (
	"testing"

	"github.com/rivo/tview"
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

func TestOmnibox_ProfileAutocomplete(t *testing.T) {
	o := NewOmnibox()
	o.SetProfiles([]string{"default", "staging", "production", "dev"})
	o.SetRegions([]string{"us-east-1", "us-west-2", "eu-west-1"})

	// Typing "profile=" should list all profiles
	results := o.commandAutocomplete("profile=")
	if len(results) != 4 {
		t.Errorf("expected 4 profile suggestions, got %d: %v", len(results), results)
	}

	// Typing "profile=st" should filter to staging
	results = o.commandAutocomplete("profile=st")
	if len(results) != 1 || results[0] != "profile=staging" {
		t.Errorf("expected [profile=staging], got %v", results)
	}

	// Typing "region=us" should match us-east-1 and us-west-2
	results = o.commandAutocomplete("region=us")
	if len(results) != 2 {
		t.Errorf("expected 2 region suggestions, got %d: %v", len(results), results)
	}
}

func TestSortableTable_New(t *testing.T) {
	st := NewSortableTable(SortableTableConfig{
		Title: "Test",
		Columns: []Column{
			{Title: "NAME", Field: "name", Expansion: 2},
			{Title: "VALUE", Field: "value", Expansion: 1},
		},
	})
	if st == nil {
		t.Fatal("expected non-nil table")
	}
	if st.SortColumn() != "name" {
		t.Errorf("expected default sort column 'name', got '%s'", st.SortColumn())
	}
	if st.SortDirection() != SortAsc {
		t.Errorf("expected SortAsc, got %v", st.SortDirection())
	}
}

func TestSortableTable_SortRows(t *testing.T) {
	st := NewSortableTable(SortableTableConfig{
		Title: "Test",
		Columns: []Column{
			{Title: "NAME", Field: "name", Expansion: 1},
			{Title: "AGE", Field: "age", Expansion: 1},
		},
	})

	rows := []Row{
		{ID: "3", Cells: []string{"charlie", "30"}},
		{ID: "1", Cells: []string{"alice", "25"}},
		{ID: "2", Cells: []string{"bob", "35"}},
	}
	st.SetRows(rows)
	st.SortRows(func(row Row) string {
		return row.Cells[0] // sort by name
	})

	// Should be alice, bob, charlie
	if st.rows[0].ID != "1" {
		t.Errorf("expected first row ID '1' (alice), got '%s'", st.rows[0].ID)
	}
	if st.rows[1].ID != "2" {
		t.Errorf("expected second row ID '2' (bob), got '%s'", st.rows[1].ID)
	}
	if st.rows[2].ID != "3" {
		t.Errorf("expected third row ID '3' (charlie), got '%s'", st.rows[2].ID)
	}
}

func TestSortableTable_ToggleDirection(t *testing.T) {
	st := NewSortableTable(SortableTableConfig{
		Title: "Test",
		Columns: []Column{
			{Title: "NAME", Field: "name", Expansion: 1},
		},
	})

	rows := []Row{
		{ID: "1", Cells: []string{"alice"}},
		{ID: "2", Cells: []string{"bob"}},
		{ID: "3", Cells: []string{"charlie"}},
	}
	st.SetRows(rows)
	st.SortRows(func(row Row) string { return row.Cells[0] })

	if st.rows[0].ID != "1" {
		t.Fatalf("ascending: expected first='1', got '%s'", st.rows[0].ID)
	}

	// Toggle to descending
	st.sortDir = SortDesc
	st.SortRows(func(row Row) string { return row.Cells[0] })

	if st.rows[0].ID != "3" {
		t.Fatalf("descending: expected first='3', got '%s'", st.rows[0].ID)
	}
}

func TestSortableTable_CycleColumn(t *testing.T) {
	st := NewSortableTable(SortableTableConfig{
		Title: "Test",
		Columns: []Column{
			{Title: "A", Field: "a", Expansion: 1},
			{Title: "B", Field: "b", Expansion: 1},
			{Title: "C", Field: "c", Expansion: 1},
		},
	})

	if st.SortColumn() != "a" {
		t.Errorf("expected 'a', got '%s'", st.SortColumn())
	}
	// Simulate pressing 's' by manipulating sortCol
	st.sortCol = (st.sortCol + 1) % len(st.columns)
	if st.SortColumn() != "b" {
		t.Errorf("expected 'b', got '%s'", st.SortColumn())
	}
	st.sortCol = (st.sortCol + 1) % len(st.columns)
	if st.SortColumn() != "c" {
		t.Errorf("expected 'c', got '%s'", st.SortColumn())
	}
	st.sortCol = (st.sortCol + 1) % len(st.columns)
	if st.SortColumn() != "a" {
		t.Errorf("expected wrap to 'a', got '%s'", st.SortColumn())
	}
}

func TestCompletionList_ShowHide(t *testing.T) {
	cl := NewCompletionList()

	if cl.Visible() {
		t.Error("expected not visible initially")
	}

	cl.Show([]string{"region=eu-west-1", "region=eu-west-2", "region=eu-central-1"})
	if !cl.Visible() {
		t.Error("expected visible after Show")
	}
	if cl.DesiredHeight(10) < 3 {
		t.Errorf("expected height >= 3, got %d", cl.DesiredHeight(10))
	}

	cl.Hide()
	if cl.Visible() {
		t.Error("expected not visible after Hide")
	}
}

func TestCompletionList_Navigation(t *testing.T) {
	cl := NewCompletionList()
	cl.Show([]string{"alpha", "beta", "charlie"})

	// Initially at 0
	cl.MoveDown()
	// Now at 1 (beta)
	cl.MoveDown()
	// Now at 2 (charlie)

	var picked string
	cl.SetOnPick(func(text string) { picked = text })
	cl.Accept()

	if picked != "charlie" {
		t.Errorf("expected 'charlie', got '%s'", picked)
	}
	if cl.Visible() {
		t.Error("expected hidden after Accept")
	}
}

func TestCompletionList_WrapAround(t *testing.T) {
	cl := NewCompletionList()
	cl.Show([]string{"alpha", "beta", "charlie"})

	// Move up from 0 should wrap to bottom
	cl.MoveUp()
	var picked string
	cl.SetOnPick(func(text string) { picked = text })
	cl.Accept()
	if picked != "charlie" {
		t.Errorf("expected wrap to 'charlie', got '%s'", picked)
	}
}

func TestCompletionList_EmptyShowHides(t *testing.T) {
	cl := NewCompletionList()
	cl.Show([]string{"alpha"})
	if !cl.Visible() {
		t.Error("expected visible")
	}
	// Showing empty list should hide
	cl.Show([]string{})
	if cl.Visible() {
		t.Error("expected hidden after empty Show")
	}
}

func TestOmnibox_GetCompletions(t *testing.T) {
	o := NewOmnibox()
	o.SetProfiles([]string{"default", "staging", "production", "vng-dev", "vng-prod"})
	o.SetRegions([]string{"us-east-1", "us-west-2", "eu-west-1", "eu-west-2", "eu-central-1"})

	// region=eu should match eu-* regions
	items := o.GetCompletions("region=eu")
	if len(items) != 3 {
		t.Errorf("expected 3 eu regions, got %d: %v", len(items), items)
	}

	// profile=vng should match vng-dev, vng-prod
	items = o.GetCompletions("profile=vng")
	if len(items) != 2 {
		t.Errorf("expected 2 vng profiles, got %d: %v", len(items), items)
	}

	// Typing just "ec2" should return nil (not a popup scenario)
	items = o.GetCompletions("ec2")
	if items != nil {
		t.Errorf("expected nil for non-popup command, got %v", items)
	}
}

func TestTabbedView_Navigation(t *testing.T) {
	pages := []TabPage{
		{Name: "Page1", Content: tview.NewTextView()},
		{Name: "Page2", Content: tview.NewTextView()},
		{Name: "Page3", Content: tview.NewTextView()},
	}
	tv := NewTabbedView(pages)

	if tv.CurrentPage() != 0 {
		t.Errorf("expected page 0, got %d", tv.CurrentPage())
	}
	if tv.CurrentPageName() != "Page1" {
		t.Errorf("expected 'Page1', got '%s'", tv.CurrentPageName())
	}

	tv.Next()
	if tv.CurrentPage() != 1 {
		t.Errorf("expected page 1, got %d", tv.CurrentPage())
	}

	tv.Next()
	if tv.CurrentPage() != 2 {
		t.Errorf("expected page 2, got %d", tv.CurrentPage())
	}

	// Wrap around
	tv.Next()
	if tv.CurrentPage() != 0 {
		t.Errorf("expected wrap to page 0, got %d", tv.CurrentPage())
	}

	// Wrap backwards
	tv.Prev()
	if tv.CurrentPage() != 2 {
		t.Errorf("expected wrap to page 2, got %d", tv.CurrentPage())
	}
}

func TestTabbedView_SwitchTo(t *testing.T) {
	pages := []TabPage{
		{Name: "A", Content: tview.NewTextView()},
		{Name: "B", Content: tview.NewTextView()},
		{Name: "C", Content: tview.NewTextView()},
	}
	tv := NewTabbedView(pages)

	tv.SwitchTo(2)
	if tv.CurrentPageName() != "C" {
		t.Errorf("expected 'C', got '%s'", tv.CurrentPageName())
	}

	// Invalid index should be ignored
	tv.SwitchTo(99)
	if tv.CurrentPageName() != "C" {
		t.Errorf("expected still 'C', got '%s'", tv.CurrentPageName())
	}
}
