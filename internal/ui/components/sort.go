package components

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SortDirection represents ascending or descending sort order.
type SortDirection int

const (
	SortAsc  SortDirection = iota
	SortDesc
)

// String returns the display string for a sort direction.
func (d SortDirection) String() string {
	if d == SortAsc {
		return "asc"
	}
	return "desc"
}

// Symbol returns an arrow indicator for the sort direction.
func (d SortDirection) Symbol() string {
	if d == SortAsc {
		return "\u2191" // ↑
	}
	return "\u2193" // ↓
}

// Column defines a table column.
type Column struct {
	Title     string // Display header, e.g. "NAME"
	Field     string // Sort key identifier, e.g. "name"
	Expansion int    // tview expansion weight (0 = auto)
}

// Row represents a single rendered table row.
type Row struct {
	// Cells is the text content for each column.
	Cells []string
	// Colors is the color for each column cell. If nil or shorter than Cells,
	// tcell.ColorWhite is used as default.
	Colors []tcell.Color
	// ID is an opaque identifier the view can use to look up the backing data.
	ID string
}

// StatusReporter is called by SortableTable to push status messages.
type StatusReporter func(text string)

// SortableTable is a reusable, sortable table widget.
// Views provide column definitions and inject row data; the table handles
// rendering, header sort indicators, and s/d key bindings.
type SortableTable struct {
	Table    *tview.Table
	columns  []Column
	sortCol  int
	sortDir  SortDirection
	rows     []Row
	title    string
	onStatus StatusReporter

	// extraInput is an optional handler for view-specific keys.
	// Return nil to consume the event, or the event to pass it through.
	extraInput func(event *tcell.EventKey) *tcell.EventKey
}

// SortableTableConfig holds configuration for creating a SortableTable.
type SortableTableConfig struct {
	Title    string
	Columns  []Column
	OnStatus StatusReporter
}

// NewSortableTable creates a new sortable table with the given config.
func NewSortableTable(cfg SortableTableConfig) *SortableTable {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetSelectable(true, false)
	table.SetTitle(fmt.Sprintf(" %s ", cfg.Title))
	table.SetBorder(true)
	table.SetBorderColor(tcell.ColorDodgerBlue)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	st := &SortableTable{
		Table:    table,
		columns:  cfg.Columns,
		sortCol:  0,
		sortDir:  SortAsc,
		title:    cfg.Title,
		onStatus: cfg.OnStatus,
	}

	table.SetInputCapture(st.handleInput)
	return st
}

// SetExtraInput sets an additional key handler for view-specific shortcuts.
// It is called AFTER the sort keys are checked. Return nil to consume,
// or return the event to let it pass through.
func (st *SortableTable) SetExtraInput(fn func(event *tcell.EventKey) *tcell.EventKey) {
	st.extraInput = fn
}

// SetSelectedFunc sets the handler for when a row is selected (Enter).
func (st *SortableTable) SetSelectedFunc(fn func(row int, id string)) {
	st.Table.SetSelectedFunc(func(row, _ int) {
		if row <= 0 {
			return
		}
		idx := row - 1
		if idx >= len(st.rows) {
			return
		}
		fn(idx, st.rows[idx].ID)
	})
}

// SetRows replaces the table data and re-renders.
func (st *SortableTable) SetRows(rows []Row) {
	st.rows = rows
	st.render()
}

// GetRowID returns the ID of the currently selected row, or "" if none.
func (st *SortableTable) GetRowID() string {
	row, _ := st.Table.GetSelection()
	if row <= 0 {
		return ""
	}
	idx := row - 1
	if idx >= len(st.rows) {
		return ""
	}
	return st.rows[idx].ID
}

// GetSelectedIndex returns the 0-based index into the rows slice, or -1.
func (st *SortableTable) GetSelectedIndex() int {
	row, _ := st.Table.GetSelection()
	if row <= 0 {
		return -1
	}
	idx := row - 1
	if idx >= len(st.rows) {
		return -1
	}
	return idx
}

// SortColumn returns the current sort column field name.
func (st *SortableTable) SortColumn() string {
	if len(st.columns) == 0 {
		return ""
	}
	return st.columns[st.sortCol].Field
}

// SortDirection returns the current sort direction.
func (st *SortableTable) SortDirection() SortDirection {
	return st.sortDir
}

// SortLabel returns a display string like "name ↑".
func (st *SortableTable) SortLabel() string {
	return st.SortColumn() + " " + st.sortDir.Symbol()
}

// render redraws the table with current data and sort.
func (st *SortableTable) render() {
	st.Table.Clear()

	// Header row
	for col, c := range st.columns {
		label := c.Title
		if col == st.sortCol {
			label = c.Title + " " + st.sortDir.Symbol()
		}
		cell := tview.NewTableCell(label).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(c.Expansion)
		st.Table.SetCell(0, col, cell)
	}

	// Data rows
	for rowIdx, row := range st.rows {
		for col, text := range row.Cells {
			color := tcell.ColorWhite
			if col < len(row.Colors) {
				color = row.Colors[col]
			}
			expansion := 1
			if col < len(st.columns) {
				expansion = st.columns[col].Expansion
			}
			if expansion == 0 {
				expansion = 1
			}
			cell := tview.NewTableCell(text).
				SetTextColor(color).
				SetExpansion(expansion)
			st.Table.SetCell(rowIdx+1, col, cell)
		}
	}

	st.Table.SetTitle(fmt.Sprintf(" %s (%d) ", st.title, len(st.rows)))
}

// SetTitle overrides the base title (count is still appended on render).
func (st *SortableTable) SetTitle(title string) {
	st.title = title
	st.Table.SetTitle(fmt.Sprintf(" %s (%d) ", st.title, len(st.rows)))
}

// SortRows sorts the rows slice in-place using the given key function,
// respecting the current sort direction, then re-renders.
func (st *SortableTable) SortRows(keyFn func(row Row) string) {
	dir := st.sortDir
	sort.SliceStable(st.rows, func(i, j int) bool {
		ki := keyFn(st.rows[i])
		kj := keyFn(st.rows[j])
		if dir == SortAsc {
			return ki < kj
		}
		return ki > kj
	})
	st.render()
}

// handleInput processes sort keys (s/d) and delegates the rest.
func (st *SortableTable) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 's':
		st.sortCol = (st.sortCol + 1) % len(st.columns)
		if st.onStatus != nil {
			st.onStatus(fmt.Sprintf("[dodgerblue]Sort: %s", st.SortLabel()))
		}
		st.render()
		return nil
	case 'd':
		if st.sortDir == SortAsc {
			st.sortDir = SortDesc
		} else {
			st.sortDir = SortAsc
		}
		if st.onStatus != nil {
			st.onStatus(fmt.Sprintf("[dodgerblue]Sort: %s", st.SortLabel()))
		}
		st.render()
		return nil
	}

	if st.extraInput != nil {
		return st.extraInput(event)
	}
	return event
}
