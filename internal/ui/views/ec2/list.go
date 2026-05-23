// Package ec2view provides the EC2 instance list and detail views.
package ec2view

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/cloudwatch"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// Navigator is the interface for views to navigate.
type Navigator interface {
	Navigate(route navigation.Route)
	NavigateBack()
	EC2Service() *ec2.Service
	CloudWatchService() *cloudwatch.Service
	TviewApp() *tview.Application
	Context() context.Context
	ShowConfirm(prompt string, onConfirm func())
	ShowInput(prompt, prefill string, callback func(string))
	SetStatus(text string)
	HandleAuthError(err error) bool
	RunEC2ConnectCmd(instanceID string) bool
}

// ec2Columns defines the column layout for the EC2 table.
var ec2Columns = []components.Column{
	{Title: "NAME", Field: "name", Expansion: 2},
	{Title: "INSTANCE ID", Field: "instance_id", Expansion: 1},
	{Title: "STATE", Field: "state", Expansion: 1},
	{Title: "TYPE", Field: "type", Expansion: 1},
	{Title: "PRIVATE IP", Field: "private_ip", Expansion: 1},
	{Title: "PUBLIC IP", Field: "public_ip", Expansion: 1},
	{Title: "AZ", Field: "az", Expansion: 1},
	{Title: "KEY", Field: "key", Expansion: 1},
}

// ListView displays a list of EC2 instances.
type ListView struct {
	st        *components.SortableTable
	navigator Navigator

	mu        sync.RWMutex
	instances []ec2.Instance
	filtered  []ec2.Instance
	filter    string

	// Multi-select split view
	layout      *tview.Flex
	selectPanel *tview.TextView
}

// NewListView creates a new EC2 list view.
func NewListView(navigator Navigator) *ListView {
	v := &ListView{
		navigator: navigator,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "EC2 Instances",
		Columns:  ec2Columns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)
	v.st.SetSelectedFunc(v.onSelect)

	// Selection panel (bottom viewport, shown during multi-select)
	v.selectPanel = tview.NewTextView()
	v.selectPanel.SetDynamicColors(true)
	v.selectPanel.SetBorder(true)
	v.selectPanel.SetBorderColor(tcell.ColorYellow)
	v.selectPanel.SetTitle(" Selected Instances ")
	v.selectPanel.SetBackgroundColor(tcell.ColorDefault)

	v.st.SetOnSelectionChanged(v.onMultiSelectChanged)

	// Layout starts as table-only; split added when select mode activates
	v.layout = tview.NewFlex().SetDirection(tview.FlexRow)
	v.layout.AddItem(v.st.Table, 0, 1, true)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "ec2"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.layout
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

	v.rebuildRows()
	return nil
}

// Shortcuts returns EC2-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	sc := []components.Shortcut{
		{Key: "Enter", Label: "details"},
		{Key: "c", Label: "connect"},
		{Key: "S", Label: "multi-select"},
		{Key: "Del", Label: "terminate"},
		{Key: "r", Label: "reboot"},
		{Key: "x", Label: "stop"},
		{Key: "a", Label: "start"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
	if v.st.SelectMode() {
		sc = append([]components.Shortcut{
			{Key: "Space", Label: "toggle"},
			{Key: "S", Label: "exit select"},
		}, sc[2:]...) // replace Enter/S with Space/S at front
	}
	return sc
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
	v.rebuildRows()
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

// rebuildRows converts filtered instances into table rows and applies sort.
func (v *ListView) rebuildRows() {
	v.mu.RLock()
	filtered := make([]ec2.Instance, len(v.filtered))
	copy(filtered, v.filtered)
	total := len(v.instances)
	filter := v.filter
	v.mu.RUnlock()

	rows := make([]components.Row, len(filtered))
	for i, inst := range filtered {
		name := inst.Name
		if name == "" {
			name = "[gray]-"
		}
		rows[i] = components.Row{
			ID: inst.InstanceID,
			Cells: []string{
				name,
				inst.InstanceID,
				inst.State,
				inst.Type,
				inst.PrivateIP,
				orDash(inst.PublicIP),
				inst.AZ,
				orDash(inst.KeyName),
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorLightGray,
				stateColor(inst.State),
				tcell.ColorLightGray,
				tcell.ColorWhite,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
			},
		}
	}

	// Set rows and configure sort - SetSortKeyFn is retained for s/d re-sorting
	v.st.SetRows(rows)
	v.st.SetSortKeyFn(func(row components.Row, field string) string {
		return ec2SortKey(row, field)
	})

	// Update title with filter info
	if filter != "" {
		v.st.SetTitle(fmt.Sprintf("EC2 Instances (%d/%d) [yellow][filter: %s]", len(rows), total, filter))
	}
}

// ec2SortKey extracts a sort key from a Row for a given column field.
func ec2SortKey(row components.Row, col string) string {
	idx := -1
	for i, c := range ec2Columns {
		if c.Field == col {
			idx = i
			break
		}
	}
	if idx < 0 || idx >= len(row.Cells) {
		return ""
	}
	return strings.ToLower(row.Cells[idx])
}

// handleInput processes view-specific key events (beyond sort).
func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	// S (capital) toggles multi-select mode
	if event.Rune() == 'S' {
		v.toggleSelectMode()
		return nil
	}

	// Delete key = terminate
	if event.Key() == tcell.KeyDelete {
		idx := v.st.GetSelectedIndex()
		if idx < 0 {
			return event
		}
		v.mu.RLock()
		if idx >= len(v.filtered) {
			v.mu.RUnlock()
			return event
		}
		inst := v.filtered[idx]
		v.mu.RUnlock()

		name := inst.Name
		if name == "" {
			name = inst.InstanceID
		}
		instanceID := inst.InstanceID
		v.navigator.ShowConfirm(fmt.Sprintf("Terminate %s?", name), func() {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]Terminating %s...", instanceID))
			go func() {
				err := v.navigator.EC2Service().TerminateInstance(v.navigator.Context(), instanceID)
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					if err != nil {
						v.navigator.SetStatus(fmt.Sprintf("[red]Failed to terminate: %s", err.Error()))
					} else {
						v.navigator.SetStatus(fmt.Sprintf("[green]Terminated %s", instanceID))
						v.Refresh(v.navigator.Context())
					}
				})
			}()
		})
		return nil
	}

	switch event.Rune() {
	case 'r':
		idx := v.st.GetSelectedIndex()
		if idx < 0 {
			return event
		}
		v.mu.RLock()
		if idx >= len(v.filtered) {
			v.mu.RUnlock()
			return event
		}
		inst := v.filtered[idx]
		v.mu.RUnlock()

		name := inst.Name
		if name == "" {
			name = inst.InstanceID
		}
		instanceID := inst.InstanceID
		v.navigator.ShowConfirm(fmt.Sprintf("Reboot %s?", name), func() {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]Rebooting %s...", instanceID))
			go func() {
				err := v.navigator.EC2Service().RebootInstance(v.navigator.Context(), instanceID)
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					if err != nil {
						v.navigator.SetStatus(fmt.Sprintf("[red]Failed to reboot: %s", err.Error()))
					} else {
						v.navigator.SetStatus(fmt.Sprintf("[green]Rebooting %s", instanceID))
					}
				})
			}()
		})
		return nil

	case 'x':
		idx := v.st.GetSelectedIndex()
		if idx < 0 {
			return event
		}
		v.mu.RLock()
		if idx >= len(v.filtered) {
			v.mu.RUnlock()
			return event
		}
		inst := v.filtered[idx]
		v.mu.RUnlock()

		name := inst.Name
		if name == "" {
			name = inst.InstanceID
		}
		instanceID := inst.InstanceID
		v.navigator.ShowConfirm(fmt.Sprintf("Stop %s?", name), func() {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]Stopping %s...", instanceID))
			go func() {
				err := v.navigator.EC2Service().StopInstance(v.navigator.Context(), instanceID)
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					if err != nil {
						v.navigator.SetStatus(fmt.Sprintf("[red]Failed to stop: %s", err.Error()))
					} else {
						v.navigator.SetStatus(fmt.Sprintf("[green]Stopping %s", instanceID))
						v.Refresh(v.navigator.Context())
					}
				})
			}()
		})
		return nil

	case 'a':
		idx := v.st.GetSelectedIndex()
		if idx < 0 {
			return event
		}
		v.mu.RLock()
		if idx >= len(v.filtered) {
			v.mu.RUnlock()
			return event
		}
		inst := v.filtered[idx]
		v.mu.RUnlock()

		instanceID := inst.InstanceID
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Starting %s...", instanceID))
		go func() {
			err := v.navigator.EC2Service().StartInstance(v.navigator.Context(), instanceID)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed to start: %s", err.Error()))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]Starting %s", instanceID))
					v.Refresh(v.navigator.Context())
				}
			})
		}()
		return nil

	case 'c':
		idx := v.st.GetSelectedIndex()
		if idx < 0 {
			return event
		}
		v.mu.RLock()
		if idx >= len(v.filtered) {
			v.mu.RUnlock()
			return event
		}
		inst := v.filtered[idx]
		v.mu.RUnlock()

		v.navigator.RunEC2ConnectCmd(inst.InstanceID)
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

// toggleSelectMode switches multi-select mode on/off.
func (v *ListView) toggleSelectMode() {
	newMode := !v.st.SelectMode()
	v.st.SetSelectMode(newMode)

	if newMode {
		v.navigator.SetStatus("[yellow]Multi-select mode: Space to toggle, S to exit")
		v.rebuildSplitLayout(true)
	} else {
		v.st.ClearSelected()
		v.navigator.SetStatus("[green]Multi-select mode off")
		v.rebuildSplitLayout(false)
	}
}

// rebuildSplitLayout reconfigures the layout flex for split/non-split mode.
func (v *ListView) rebuildSplitLayout(split bool) {
	v.layout.Clear()
	if split {
		v.layout.AddItem(v.st.Table, 0, 2, true)
		v.layout.AddItem(v.selectPanel, 0, 1, false)
	} else {
		v.layout.AddItem(v.st.Table, 0, 1, true)
	}
}

// onMultiSelectChanged is called when multi-select checkboxes change.
func (v *ListView) onMultiSelectChanged(ids []string) {
	if len(ids) == 0 {
		v.selectPanel.SetText("\n  [gray]No instances selected. Press Space to select.")
		v.selectPanel.SetTitle(" Selected Instances ")
		return
	}

	v.mu.RLock()
	var b strings.Builder
	for _, id := range ids {
		for _, inst := range v.filtered {
			if inst.InstanceID == id {
				name := inst.Name
				if name == "" {
					name = inst.InstanceID
				}
				stColor := stateColor(inst.State)
				b.WriteString(fmt.Sprintf("  [white]%s[gray] (%s) [%s]%s[-]\n",
					name, inst.InstanceID,
					colorName(stColor), inst.State))
				break
			}
		}
	}
	v.mu.RUnlock()

	v.selectPanel.SetTitle(fmt.Sprintf(" Selected Instances (%d) ", len(ids)))
	v.selectPanel.SetText(b.String())
}

// onSelect handles Enter key on an instance row.
func (v *ListView) onSelect(idx int, id string) {
	v.mu.RLock()
	if idx >= len(v.filtered) {
		v.mu.RUnlock()
		return
	}
	inst := v.filtered[idx]
	v.mu.RUnlock()

	v.navigator.Navigate(navigation.Route{
		Resource:   "ec2-detail",
		ResourceID: id,
		Params:     map[string]string{"name": inst.Name},
	})
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

// colorName maps tcell.Color to a tview dynamic color tag name.
func colorName(c tcell.Color) string {
	switch c {
	case tcell.ColorGreen:
		return "green"
	case tcell.ColorRed:
		return "red"
	case tcell.ColorYellow:
		return "yellow"
	case tcell.ColorDarkGray:
		return "gray"
	default:
		return "white"
	}
}
