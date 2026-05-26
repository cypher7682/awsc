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

	if len(h.shortcutList) != 1 {
		t.Errorf("expected 1 shortcut, got %d", len(h.shortcutList))
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

	// Fuzzy matching: "usw2" should match us-west-2
	items = o.GetCompletions("r=usw2")
	if len(items) != 1 || items[0] != "r=us-west-2" {
		t.Errorf("expected [r=us-west-2] for fuzzy 'usw2', got %v", items)
	}

	// Fuzzy matching: "euw1" should match eu-west-1
	items = o.GetCompletions("region=euw1")
	if len(items) != 1 || items[0] != "region=eu-west-1" {
		t.Errorf("expected [region=eu-west-1] for fuzzy 'euw1', got %v", items)
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

func TestSortableTable_MultiSelect(t *testing.T) {
	st := NewSortableTable(SortableTableConfig{
		Title:   "test",
		Columns: []Column{{Title: "A", Field: "a", Expansion: 1}},
	})

	// Not in select mode by default
	if st.SelectMode() {
		t.Error("expected select mode off by default")
	}
	if st.SelectedCount() != 0 {
		t.Error("expected 0 selected")
	}

	// Add rows
	st.SetRows([]Row{
		{ID: "1", Cells: []string{"Alpha"}},
		{ID: "2", Cells: []string{"Beta"}},
		{ID: "3", Cells: []string{"Charlie"}},
	})

	// Enable select mode
	st.SetSelectMode(true)
	if !st.SelectMode() {
		t.Error("expected select mode on")
	}

	// Select row 0 (table starts at row 1 for data)
	st.Table.Select(1, 0)
	st.ToggleSelected()
	if st.SelectedCount() != 1 {
		t.Errorf("expected 1 selected, got %d", st.SelectedCount())
	}
	ids := st.SelectedIDs()
	if len(ids) != 1 || ids[0] != "1" {
		t.Errorf("expected ['1'], got %v", ids)
	}

	// Select another
	st.Table.Select(3, 0)
	st.ToggleSelected()
	if st.SelectedCount() != 2 {
		t.Errorf("expected 2 selected, got %d", st.SelectedCount())
	}

	// Deselect first
	st.Table.Select(1, 0)
	st.ToggleSelected()
	if st.SelectedCount() != 1 {
		t.Errorf("expected 1 selected after deselect, got %d", st.SelectedCount())
	}
	ids = st.SelectedIDs()
	if len(ids) != 1 || ids[0] != "3" {
		t.Errorf("expected ['3'], got %v", ids)
	}

	// Clear all
	st.ClearSelected()
	if st.SelectedCount() != 0 {
		t.Error("expected 0 after clear")
	}

	// Disable select mode
	st.SetSelectMode(false)
	if st.SelectMode() {
		t.Error("expected select mode off")
	}
}

func TestSortableTable_OnSelectionChanged(t *testing.T) {
	st := NewSortableTable(SortableTableConfig{
		Title:   "test",
		Columns: []Column{{Title: "A", Field: "a", Expansion: 1}},
	})
	st.SetRows([]Row{
		{ID: "x", Cells: []string{"X"}},
		{ID: "y", Cells: []string{"Y"}},
	})
	st.SetSelectMode(true)

	var callbackIDs []string
	st.SetOnSelectionChanged(func(ids []string) {
		callbackIDs = ids
	})

	st.Table.Select(1, 0)
	st.ToggleSelected()
	if len(callbackIDs) != 1 || callbackIDs[0] != "x" {
		t.Errorf("expected callback with ['x'], got %v", callbackIDs)
	}
}

func TestChart_Creation(t *testing.T) {
	chart := NewChart("CPUUtilization", "Percent", "3h", 0)
	if chart == nil {
		t.Fatal("expected non-nil chart")
	}
}

func TestChart_SetData(t *testing.T) {
	chart := NewChart("Test", "Percent", "3h", 0)
	chart.SetHeight(4)

	// No data
	chart.SetData(nil)

	// Some data
	data := []ChartDatapoint{
		{Value: 10.0, Label: "14:00"},
		{Value: 50.0, Label: "14:05"},
		{Value: 30.0, Label: "14:10"},
		{Value: 80.0, Label: "14:15"},
		{Value: 20.0, Label: "14:20"},
	}
	chart.SetData(data) // should not panic
}

func TestChart_ConstantData(t *testing.T) {
	chart := NewChart("Flat", "Count", "3h", 0)
	chart.SetHeight(4)

	// All same values (edge case: valRange == 0)
	data := []ChartDatapoint{
		{Value: 42.0},
		{Value: 42.0},
		{Value: 42.0},
	}
	chart.SetData(data) // should not panic
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value    float64
		unit     string
		expected string
	}{
		{45.2, "Percent", "45.2%"},
		{0, "Percent", "0.0%"},
		{100, "Count", "100"},
		{1500, "Count", "1.5K"},
		{2_500_000, "Count", "2.5M"},
		{512, "Bytes", "512 B"},
	}
	for _, tt := range tests {
		got := formatValue(tt.value, tt.unit)
		if got != tt.expected {
			t.Errorf("formatValue(%.1f, %q) = %q, want %q", tt.value, tt.unit, got, tt.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    float64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.expected {
			t.Errorf("formatBytes(%.0f) = %q, want %q", tt.bytes, got, tt.expected)
		}
	}
}

func TestBrailleChar(t *testing.T) {
	// Empty grid should produce space
	grid := make([][]bool, 4)
	for i := range grid {
		grid[i] = make([]bool, 2)
	}
	ch := brailleChar(grid, 0, 0, 4, 2)
	if ch != ' ' {
		t.Errorf("expected space for empty grid, got %c (U+%04X)", ch, ch)
	}

	// Single dot top-left should be braille dot 0 (U+2801)
	grid[0][0] = true
	ch = brailleChar(grid, 0, 0, 4, 2)
	if ch != '\u2801' {
		t.Errorf("expected U+2801, got U+%04X", ch)
	}
}

func TestFuzzyScore(t *testing.T) {
	tests := []struct {
		query    string
		target   string
		wantPos  bool   // expect positive score (match)
		minScore int    // minimum expected score (0 = any positive)
		desc     string // description
	}{
		// Exact matches
		{"us-west-2", "us-west-2", true, 1000, "exact match"},
		{"US-WEST-2", "us-west-2", true, 1000, "exact match case insensitive"},

		// Prefix matches
		{"us", "us-west-2", true, 500, "prefix match"},
		{"us-w", "us-west-2", true, 500, "prefix match longer"},
		{"eu", "eu-west-1", true, 500, "prefix match eu"},

		// Contains matches
		{"west", "us-west-2", true, 200, "contains match"},
		{"east", "us-east-1", true, 200, "contains match east"},

		// Fuzzy matches (shortcuts)
		{"usw2", "us-west-2", true, 100, "fuzzy shortcut usw2"},
		{"use1", "us-east-1", true, 100, "fuzzy shortcut use1"},
		{"euw1", "eu-west-1", true, 100, "fuzzy shortcut euw1"},
		{"apne1", "ap-northeast-1", true, 100, "fuzzy shortcut apne1"},
		{"rtc-dev", "vng-api-rtc-main-dev", true, 100, "fuzzy mid-string rtc-dev"},
		{"api-dev", "vng-api-rtc-main-dev", true, 100, "fuzzy mid-string api-dev"},

		// Non-matches (first char must be at word boundary for fuzzy)
		{"us", "eu-west-1", false, 0, "us should not match eu-west-1 (u not at boundary)"},
		{"xyz", "us-west-2", false, 0, "no match"},
		{"abc", "def", false, 0, "completely different"},
	}

	for _, tt := range tests {
		score := FuzzyScore(tt.query, tt.target)
		if tt.wantPos {
			if score < 0 {
				t.Errorf("%s: FuzzyScore(%q, %q) = %d, want positive", tt.desc, tt.query, tt.target, score)
			} else if tt.minScore > 0 && score < tt.minScore {
				t.Errorf("%s: FuzzyScore(%q, %q) = %d, want >= %d", tt.desc, tt.query, tt.target, score, tt.minScore)
			}
		} else {
			if score >= 0 {
				t.Errorf("%s: FuzzyScore(%q, %q) = %d, want negative (no match)", tt.desc, tt.query, tt.target, score)
			}
		}
	}
}

func TestFuzzyFilter(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-northeast-1"}

	// usw should match us-west-2 first (prefix-ish), then us-east-1 (shares 'us')
	matches := FuzzyFilter("usw", regions)
	if len(matches) == 0 {
		t.Fatal("expected at least one match for 'usw'")
	}
	if matches[0] != "us-west-2" {
		t.Errorf("expected us-west-2 first, got %s", matches[0])
	}

	// euw1 should match eu-west-1
	matches = FuzzyFilter("euw1", regions)
	if len(matches) == 0 || matches[0] != "eu-west-1" {
		t.Errorf("expected eu-west-1 for 'euw1', got %v", matches)
	}

	// Empty query returns all sorted
	matches = FuzzyFilter("", regions)
	if len(matches) != 4 {
		t.Errorf("expected 4 results for empty query, got %d", len(matches))
	}
}

func TestFuzzyBest(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}

	// Exact prefix
	best := FuzzyBest("us-west", regions)
	if best != "us-west-2" {
		t.Errorf("expected us-west-2, got %s", best)
	}

	// Fuzzy shortcut
	best = FuzzyBest("usw2", regions)
	if best != "us-west-2" {
		t.Errorf("expected us-west-2 for 'usw2', got %s", best)
	}

	// No match
	best = FuzzyBest("xyz", regions)
	if best != "" {
		t.Errorf("expected empty for no match, got %s", best)
	}
}
