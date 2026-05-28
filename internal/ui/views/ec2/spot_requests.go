// Package ec2view provides EC2 views including spot instance requests.
package ec2view

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// spotRequestColumns defines the column layout for the spot requests table.
var spotRequestColumns = []components.Column{
	{Title: "REQUEST ID", Field: "id", Expansion: 1},
	{Title: "STATE", Field: "state", Expansion: 0},
	{Title: "STATUS", Field: "status", Expansion: 1},
	{Title: "INSTANCE ID", Field: "instance_id", Expansion: 1},
	{Title: "TYPE", Field: "type", Expansion: 0},
	{Title: "INSTANCE TYPE", Field: "instance_type", Expansion: 0},
	{Title: "AZ", Field: "az", Expansion: 0},
	{Title: "PRICE", Field: "price", Expansion: 0},
	{Title: "CREATED", Field: "created", Expansion: 1},
}

// SpotRequestsView displays the list of EC2 spot instance requests.
type SpotRequestsView struct {
	st        *components.SortableTable
	navigator Navigator

	mu       sync.RWMutex
	requests []ec2.SpotInstanceRequest
	filtered []ec2.SpotInstanceRequest
	filter   string
}

// NewSpotRequestsView creates a new spot requests list view.
func NewSpotRequestsView(navigator Navigator) *SpotRequestsView {
	v := &SpotRequestsView{
		navigator: navigator,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "EC2 Spot Instance Requests",
		Columns:  spotRequestColumns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)
	v.st.SetSelectedFunc(v.onSelect)

	return v
}

// Name returns the view identifier.
func (v *SpotRequestsView) Name() string {
	return "ec2/spot"
}

// Render returns the tview primitive.
func (v *SpotRequestsView) Render() tview.Primitive {
	return v.st.Table
}

// Refresh reloads spot request data from AWS.
func (v *SpotRequestsView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()
	requests, err := svc.ListSpotInstanceRequests(ctx)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.requests = requests
	v.applyFilter()
	v.mu.Unlock()

	v.rebuildRows()
	return nil
}

// Shortcuts returns spot requests view shortcuts.
func (v *SpotRequestsView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "details"},
		{Key: "i", Label: "go to instance"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *SpotRequestsView) FilterFields() []string {
	return []string{"id", "state", "status", "instance_id", "instance_type", "az", "type"}
}

// HandleFilter applies a filter expression.
func (v *SpotRequestsView) HandleFilter(expression string) {
	v.mu.Lock()
	v.filter = expression
	v.applyFilter()
	v.mu.Unlock()
	v.rebuildRows()
}

// applyFilter filters requests based on the current filter expression.
// Must be called with mu held.
func (v *SpotRequestsView) applyFilter() {
	if v.filter == "" {
		v.filtered = v.requests
		return
	}

	v.filtered = nil
	lower := strings.ToLower(v.filter)

	// Parse field:value filters
	var field, value string
	if idx := strings.Index(lower, ":"); idx > 0 {
		field = lower[:idx]
		value = lower[idx+1:]
	} else {
		value = lower
	}

	for _, req := range v.requests {
		match := false

		switch field {
		case "id":
			match = strings.Contains(strings.ToLower(req.RequestID), value)
		case "state":
			match = strings.Contains(strings.ToLower(req.State), value)
		case "status":
			match = strings.Contains(strings.ToLower(req.StatusCode), value)
		case "instance_id":
			match = strings.Contains(strings.ToLower(req.InstanceID), value)
		case "instance_type":
			match = strings.Contains(strings.ToLower(req.InstanceType), value)
		case "az":
			match = strings.Contains(strings.ToLower(req.AvailabilityZone), value)
		case "type":
			match = strings.Contains(strings.ToLower(req.Type), value)
		default:
			// No field specified - search across all text fields
			searchable := strings.ToLower(fmt.Sprintf("%s %s %s %s %s",
				req.RequestID,
				req.State,
				req.StatusCode,
				req.InstanceID,
				req.InstanceType,
			))
			match = strings.Contains(searchable, value)
		}

		if match {
			v.filtered = append(v.filtered, req)
		}
	}
}

// rebuildRows updates the table rows from filtered data.
func (v *SpotRequestsView) rebuildRows() {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rows := make([]components.Row, 0, len(v.filtered))
	for _, req := range v.filtered {
		// Format create time
		created := req.CreateTime.Format("2006-01-02 15:04")

		// Format instance ID
		instanceID := req.InstanceID
		if instanceID == "" {
			instanceID = "-"
		}

		// Format price
		price := req.SpotPrice
		if price == "" {
			price = "-"
		} else {
			price = "$" + price
		}

		// Color based on state
		var colors []tcell.Color
		switch req.State {
		case "active":
			colors = []tcell.Color{tcell.ColorDefault, tcell.ColorGreen}
		case "open":
			colors = []tcell.Color{tcell.ColorDefault, tcell.ColorYellow}
		case "failed", "cancelled":
			colors = []tcell.Color{tcell.ColorDefault, tcell.ColorRed}
		case "closed":
			colors = []tcell.Color{tcell.ColorDefault, tcell.ColorGray}
		}

		// Cells in column order
		rows = append(rows, components.Row{
			ID: req.RequestID,
			Cells: []string{
				req.RequestID,
				req.State,
				req.StatusCode,
				instanceID,
				req.Type,
				req.InstanceType,
				req.AvailabilityZone,
				price,
				created,
			},
			Colors: colors,
		})
	}

	v.st.SetRows(rows)
}

// onSelect handles selection of a spot request.
func (v *SpotRequestsView) onSelect(row int, id string) {
	v.navigator.Navigate(navigation.Route{
		Resource:   "ec2/spot-detail",
		ResourceID: id,
	})
}

// handleInput processes view-specific key events.
func (v *SpotRequestsView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		row, _ := v.st.Table.GetSelection()
		if row > 0 && row-1 < len(v.filtered) {
			v.mu.RLock()
			req := v.filtered[row-1]
			v.mu.RUnlock()
			v.onSelect(row-1, req.RequestID)
		}
		return nil
	}

	switch event.Rune() {
	case 'i':
		// Navigate to instance if fulfilled
		row, _ := v.st.Table.GetSelection()
		if row > 0 && row-1 < len(v.filtered) {
			v.mu.RLock()
			req := v.filtered[row-1]
			v.mu.RUnlock()
			if req.InstanceID != "" {
				v.navigator.Navigate(navigation.Route{
					Resource:   "ec2-detail",
					ResourceID: req.InstanceID,
				})
			} else {
				v.navigator.SetStatus("[yellow]No instance associated with this request")
			}
		}
		return nil

	case 'R':
		// Refresh
		v.navigator.SetStatus("[yellow]Refreshing spot requests...")
		go func() {
			ctx := v.navigator.Context()
			if err := v.Refresh(ctx); err != nil {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus(fmt.Sprintf("[red]Refresh failed: %s", err.Error()))
				})
			} else {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus("[green]Spot requests refreshed")
				})
			}
		}()
		return nil
	}
	return event
}
