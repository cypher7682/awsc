// Package ec2view provides the EC2 instance list and detail views.
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

// Navigator is the interface for views to navigate.
type Navigator interface {
	Navigate(route navigation.Route)
	EC2Service() *ec2.Service
	TviewApp() *tview.Application
	Context() context.Context
	ShowConfirm(prompt string)
	SetStatus(text string)
}

// ListView displays a list of EC2 instances.
type ListView struct {
	table     *tview.Table
	navigator Navigator

	mu        sync.RWMutex
	instances []ec2.Instance
	filtered  []ec2.Instance
	filter    string
	sortField string
	sortAsc   bool
}

// NewListView creates a new EC2 list view.
func NewListView(navigator Navigator) *ListView {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetSelectable(true, false)
	table.SetTitle(" EC2 Instances ")
	table.SetBorder(true)
	table.SetBorderColor(tcell.ColorDodgerBlue)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	v := &ListView{
		table:     table,
		navigator: navigator,
		sortField: "name",
		sortAsc:   true,
	}

	// Set up input handling
	table.SetInputCapture(v.handleInput)
	table.SetSelectedFunc(v.onSelect)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "ec2"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.table
}

// Refresh reloads instance data from AWS.
func (v *ListView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()
	instances, err := svc.ListInstancesRaw(ctx, nil)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.instances = instances
	v.applyFilter()
	v.mu.Unlock()

	v.renderTable()
	return nil
}

// Shortcuts returns EC2-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "details"},
		{Key: "t", Label: "terminate"},
		{Key: "r", Label: "reboot"},
		{Key: "s", Label: "stop"},
		{Key: "S", Label: "start"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields for EC2.
func (v *ListView) FilterFields() []string {
	return []string{
		"instance_id", "name", "state", "type", "private_ip",
		"public_ip", "vpc_id", "subnet_id", "security_group",
		"key_name", "ami", "az", "tag:",
	}
}

// HandleFilter applies a filter expression.
func (v *ListView) HandleFilter(expression string) {
	v.mu.Lock()
	v.filter = expression
	v.applyFilter()
	v.mu.Unlock()
	v.renderTable()
}

// applyFilter filters instances based on the current filter expression.
// Must be called with lock held.
func (v *ListView) applyFilter() {
	if v.filter == "" {
		v.filtered = v.instances
		return
	}

	v.filtered = nil
	lower := strings.ToLower(v.filter)

	for _, inst := range v.instances {
		if v.matchesFilter(inst, lower) {
			v.filtered = append(v.filtered, inst)
		}
	}
}

// matchesFilter checks if an instance matches the filter expression.
func (v *ListView) matchesFilter(inst ec2.Instance, filter string) bool {
	// Parse filter: "field operator value" or just "value" for text search
	parts := strings.SplitN(filter, " ", 3)

	// Simple text search if no operator
	if len(parts) < 3 {
		searchText := strings.ToLower(filter)
		return strings.Contains(strings.ToLower(inst.Name), searchText) ||
			strings.Contains(strings.ToLower(inst.InstanceID), searchText) ||
			strings.Contains(strings.ToLower(inst.State), searchText) ||
			strings.Contains(strings.ToLower(inst.Type), searchText) ||
			strings.Contains(strings.ToLower(inst.PrivateIP), searchText) ||
			strings.Contains(strings.ToLower(inst.PublicIP), searchText)
	}

	field := strings.ToLower(parts[0])
	operator := strings.ToLower(parts[1])
	value := strings.ToLower(parts[2])

	var fieldValue string
	switch field {
	case "instance_id", "id":
		fieldValue = strings.ToLower(inst.InstanceID)
	case "name":
		fieldValue = strings.ToLower(inst.Name)
	case "state":
		fieldValue = strings.ToLower(inst.State)
	case "type":
		fieldValue = strings.ToLower(inst.Type)
	case "private_ip":
		fieldValue = strings.ToLower(inst.PrivateIP)
	case "public_ip":
		fieldValue = strings.ToLower(inst.PublicIP)
	case "vpc_id", "vpc":
		fieldValue = strings.ToLower(inst.VPCID)
	case "subnet_id", "subnet":
		fieldValue = strings.ToLower(inst.SubnetID)
	case "az":
		fieldValue = strings.ToLower(inst.AZ)
	case "key_name", "key":
		fieldValue = strings.ToLower(inst.KeyName)
	default:
		// Check tags
		if strings.HasPrefix(field, "tag:") {
			tagKey := strings.TrimPrefix(field, "tag:")
			fieldValue = strings.ToLower(inst.Tags[tagKey])
		} else {
			return false
		}
	}

	switch operator {
	case "=", "==":
		return fieldValue == value
	case "!=":
		return fieldValue != value
	case "contains":
		return strings.Contains(fieldValue, value)
	case "starts_with":
		return strings.HasPrefix(fieldValue, value)
	case "ends_with":
		return strings.HasSuffix(fieldValue, value)
	default:
		return strings.Contains(fieldValue, value)
	}
}

// renderTable rebuilds the table display.
func (v *ListView) renderTable() {
	v.mu.RLock()
	instances := v.filtered
	v.mu.RUnlock()

	v.table.Clear()

	// Header row
	headers := []string{"NAME", "INSTANCE ID", "STATE", "TYPE", "PRIVATE IP", "PUBLIC IP", "AZ", "KEY"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		if col == 0 {
			cell.SetExpansion(2) // Name gets more space
		}
		v.table.SetCell(0, col, cell)
	}

	// Data rows
	for row, inst := range instances {
		stateColor := stateColor(inst.State)
		name := inst.Name
		if name == "" {
			name = "[gray]-"
		}

		cells := []struct {
			text  string
			color tcell.Color
		}{
			{name, tcell.ColorWhite},
			{inst.InstanceID, tcell.ColorLightGray},
			{inst.State, stateColor},
			{inst.Type, tcell.ColorLightGray},
			{inst.PrivateIP, tcell.ColorWhite},
			{orDash(inst.PublicIP), tcell.ColorLightGray},
			{inst.AZ, tcell.ColorLightGray},
			{orDash(inst.KeyName), tcell.ColorLightGray},
		}

		for col, c := range cells {
			cell := tview.NewTableCell(c.text).
				SetTextColor(c.color).
				SetExpansion(1)
			if col == 0 {
				cell.SetExpansion(2)
			}
			v.table.SetCell(row+1, col, cell)
		}
	}

	// Update title with count
	if v.filter != "" {
		v.table.SetTitle(fmt.Sprintf(" EC2 Instances (%d/%d) [yellow][filter: %s] ", len(instances), len(v.instances), v.filter))
	} else {
		v.table.SetTitle(fmt.Sprintf(" EC2 Instances (%d) ", len(instances)))
	}
}

// handleInput processes key events for the EC2 list.
func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	row, _ := v.table.GetSelection()
	if row <= 0 {
		return event
	}

	inst := v.getInstanceAtRow(row)
	if inst == nil {
		return event
	}

	switch event.Rune() {
	case 't':
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Terminating %s (%s)...", inst.Name, inst.InstanceID))
		go func() {
			err := v.navigator.EC2Service().TerminateInstance(v.navigator.Context(), inst.InstanceID)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed to terminate: %s", err.Error()))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]Terminated %s", inst.InstanceID))
					v.Refresh(v.navigator.Context())
				}
			})
		}()
		return nil

	case 'r':
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Rebooting %s (%s)...", inst.Name, inst.InstanceID))
		go func() {
			err := v.navigator.EC2Service().RebootInstance(v.navigator.Context(), inst.InstanceID)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed to reboot: %s", err.Error()))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]Rebooting %s", inst.InstanceID))
				}
			})
		}()
		return nil

	case 's':
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Stopping %s (%s)...", inst.Name, inst.InstanceID))
		go func() {
			err := v.navigator.EC2Service().StopInstance(v.navigator.Context(), inst.InstanceID)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed to stop: %s", err.Error()))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]Stopping %s", inst.InstanceID))
					v.Refresh(v.navigator.Context())
				}
			})
		}()
		return nil

	case 'S':
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Starting %s (%s)...", inst.Name, inst.InstanceID))
		go func() {
			err := v.navigator.EC2Service().StartInstance(v.navigator.Context(), inst.InstanceID)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed to start: %s", err.Error()))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]Starting %s", inst.InstanceID))
					v.Refresh(v.navigator.Context())
				}
			})
		}()
		return nil

	case 'R':
		go func() {
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				v.navigator.SetStatus("[yellow]Refreshing...")
			})
			v.Refresh(v.navigator.Context())
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				v.navigator.SetStatus("[green]Refreshed")
			})
		}()
		return nil
	}

	return event
}

// onSelect handles Enter key on an instance row.
func (v *ListView) onSelect(row, _ int) {
	if row <= 0 {
		return
	}

	inst := v.getInstanceAtRow(row)
	if inst == nil {
		return
	}

	v.navigator.Navigate(navigation.Route{
		Resource:   "ec2-detail",
		ResourceID: inst.InstanceID,
		Params:     map[string]string{"name": inst.Name},
	})
}

// getInstanceAtRow returns the instance at the given table row (1-indexed).
func (v *ListView) getInstanceAtRow(row int) *ec2.Instance {
	v.mu.RLock()
	defer v.mu.RUnlock()

	idx := row - 1 // account for header row
	if idx < 0 || idx >= len(v.filtered) {
		return nil
	}
	return &v.filtered[idx]
}

func stateColor(state string) tcell.Color {
	switch state {
	case "running":
		return tcell.ColorGreen
	case "stopped":
		return tcell.ColorRed
	case "terminated", "shutting-down":
		return tcell.ColorDarkGray
	case "pending", "stopping":
		return tcell.ColorYellow
	default:
		return tcell.ColorWhite
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
