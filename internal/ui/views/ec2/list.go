// Package ec2view provides the EC2 instance list and detail views.
package ec2view

import (
	"context"
	"fmt"
	"sort"
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
	RefreshShortcuts()
}

// ColumnDef defines a column that can be shown/hidden.
type ColumnDef struct {
	Title     string
	Field     string
	Expansion int
	Enabled   bool   // currently visible
	IsTag     bool   // true if this is a tag column
	TagKey    string // the tag key if IsTag
}

// Default columns for EC2 view.
var defaultEC2Columns = []ColumnDef{
	{Title: "NAME", Field: "name", Expansion: 2, Enabled: true},
	{Title: "INSTANCE ID", Field: "instance_id", Expansion: 1, Enabled: true},
	{Title: "STATE", Field: "state", Expansion: 1, Enabled: true},
	{Title: "TYPE", Field: "type", Expansion: 1, Enabled: true},
	{Title: "PRIVATE IP", Field: "private_ip", Expansion: 1, Enabled: true},
	{Title: "PUBLIC IP", Field: "public_ip", Expansion: 1, Enabled: true},
	{Title: "AZ", Field: "az", Expansion: 1, Enabled: true},
	{Title: "KEY", Field: "key", Expansion: 1, Enabled: true},
	// Additional columns (hidden by default)
	{Title: "VPC ID", Field: "vpc_id", Expansion: 1, Enabled: false},
	{Title: "SUBNET ID", Field: "subnet_id", Expansion: 1, Enabled: false},
	{Title: "AMI ID", Field: "ami_id", Expansion: 1, Enabled: false},
	{Title: "PLATFORM", Field: "platform", Expansion: 1, Enabled: false},
	{Title: "LAUNCH TIME", Field: "launch_time", Expansion: 1, Enabled: false},
	{Title: "IAM ROLE", Field: "iam_role", Expansion: 1, Enabled: false},
}

// ListView displays a list of EC2 instances.
type ListView struct {
	navigator Navigator

	mu        sync.RWMutex
	instances []ec2.Instance
	filtered  []ec2.Instance
	filter    string

	// Column configuration
	columns    []ColumnDef
	tagColumns []ColumnDef // discovered tag columns

	// UI components
	st           *components.SortableTable
	layout       *tview.Flex
	selectPanel  *tview.TextView
	picker       *components.Picker
	pickerActive bool

	// Column picker state
	columnPickerActive bool
	columnList         *tview.List
}

// NewListView creates a new EC2 list view.
func NewListView(navigator Navigator) *ListView {
	v := &ListView{
		navigator: navigator,
		columns:   make([]ColumnDef, len(defaultEC2Columns)),
	}
	copy(v.columns, defaultEC2Columns)

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "EC2 Instances",
		Columns:  v.activeColumns(),
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

	// Layout starts as table-only
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
	v.discoverTagColumns()
	v.applyFilter()
	v.mu.Unlock()

	v.rebuildRows()
	return nil
}

// discoverTagColumns finds all unique tag keys across instances.
// Must be called with lock held.
func (v *ListView) discoverTagColumns() {
	tagKeys := make(map[string]bool)
	for _, inst := range v.instances {
		for key := range inst.Tags {
			if key != "Name" { // Name is already a default column
				tagKeys[key] = true
			}
		}
	}

	// Convert to sorted slice
	var keys []string
	for k := range tagKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build tag columns (all disabled by default)
	v.tagColumns = nil
	for _, key := range keys {
		v.tagColumns = append(v.tagColumns, ColumnDef{
			Title:     "TAG:" + key,
			Field:     "tag:" + key,
			Expansion: 1,
			Enabled:   false,
			IsTag:     true,
			TagKey:    key,
		})
	}
}

// activeColumns returns the currently enabled columns as components.Column slice.
func (v *ListView) activeColumns() []components.Column {
	var cols []components.Column
	for _, c := range v.columns {
		if c.Enabled {
			cols = append(cols, components.Column{
				Title:     c.Title,
				Field:     c.Field,
				Expansion: c.Expansion,
			})
		}
	}
	for _, c := range v.tagColumns {
		if c.Enabled {
			cols = append(cols, components.Column{
				Title:     c.Title,
				Field:     c.Field,
				Expansion: c.Expansion,
			})
		}
	}
	return cols
}

// Shortcuts returns EC2-specific shortcuts.
// Service-specific at top, standard controls at bottom (marked Standard: true).
func (v *ListView) Shortcuts() []components.Shortcut {
	// Service-specific shortcuts (top section)
	serviceShortcuts := []components.Shortcut{
		{Key: "Enter", Label: "details"},
		{Key: "c", Label: "connect"},
		{Key: "Del", Label: "power mgmt"},
		{Key: "R", Label: "refresh"},
	}

	// Standard shortcuts (bottom line, greyed out)
	standardShortcuts := []components.Shortcut{
		{Key: "m", Label: "multi-select", Standard: true},
		{Key: "o", Label: "columns", Standard: true},
		{Key: "s", Label: "sort", Standard: true},
		{Key: "d", Label: "dir", Standard: true},
		{Key: "/", Label: "filter", Standard: true},
		{Key: "Esc", Label: "back", Standard: true},
	}

	if v.st.SelectMode() {
		serviceShortcuts = []components.Shortcut{
			{Key: "Space", Label: "toggle select"},
			{Key: "Del", Label: "power mgmt"},
			{Key: "R", Label: "refresh"},
		}
		standardShortcuts[0] = components.Shortcut{Key: "m", Label: "exit select", Standard: true}
	}

	return append(serviceShortcuts, standardShortcuts...)
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

// rebuildRows converts filtered instances into table rows.
func (v *ListView) rebuildRows() {
	v.mu.RLock()
	filtered := make([]ec2.Instance, len(v.filtered))
	copy(filtered, v.filtered)
	total := len(v.instances)
	filter := v.filter
	activeCols := v.activeColumns()
	v.mu.RUnlock()

	rows := make([]components.Row, len(filtered))
	for i, inst := range filtered {
		cells := make([]string, len(activeCols))
		colors := make([]tcell.Color, len(activeCols))

		for j, col := range activeCols {
			cells[j], colors[j] = v.getCellValue(inst, col.Field)
		}

		rows[i] = components.Row{
			ID:     inst.InstanceID,
			Cells:  cells,
			Colors: colors,
		}
	}

	// Rebuild table with current columns
	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "EC2 Instances",
		Columns:  activeCols,
		OnStatus: v.navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)
	v.st.SetSelectedFunc(v.onSelect)
	v.st.SetOnSelectionChanged(v.onMultiSelectChanged)

	v.st.SetRows(rows)
	v.st.SetSortKeyFn(func(row components.Row, field string) string {
		return v.sortKey(row, field, activeCols)
	})

	// Rebuild layout
	v.rebuildLayout()

	// Update title with filter info
	if filter != "" {
		v.st.SetTitle(fmt.Sprintf("EC2 Instances (%d/%d) [yellow][filter: %s]", len(rows), total, filter))
	}
}

// getCellValue returns the cell text and color for a given field.
func (v *ListView) getCellValue(inst ec2.Instance, field string) (string, tcell.Color) {
	switch field {
	case "name":
		name := inst.Name
		if name == "" {
			return "[gray]-", tcell.ColorGray
		}
		return name, tcell.ColorWhite
	case "instance_id":
		return inst.InstanceID, tcell.ColorLightGray
	case "state":
		return inst.State, stateColor(inst.State)
	case "type":
		return inst.Type, tcell.ColorLightGray
	case "private_ip":
		return orDash(inst.PrivateIP), tcell.ColorWhite
	case "public_ip":
		return orDash(inst.PublicIP), tcell.ColorLightGray
	case "az":
		return inst.AZ, tcell.ColorLightGray
	case "key":
		return orDash(inst.KeyName), tcell.ColorLightGray
	case "vpc_id":
		return orDash(inst.VPCID), tcell.ColorLightGray
	case "subnet_id":
		return orDash(inst.SubnetID), tcell.ColorLightGray
	case "ami_id":
		return orDash(inst.AMI), tcell.ColorLightGray
	case "platform":
		return orDash(inst.Platform), tcell.ColorLightGray
	case "launch_time":
		return inst.LaunchTime.Format("2006-01-02 15:04"), tcell.ColorLightGray
	case "iam_role":
		return orDash(inst.IAMRole), tcell.ColorLightGray
	default:
		if strings.HasPrefix(field, "tag:") {
			tagKey := strings.TrimPrefix(field, "tag:")
			return orDash(inst.Tags[tagKey]), tcell.ColorLightGray
		}
		return "-", tcell.ColorGray
	}
}

// sortKey extracts a sort key from a row.
func (v *ListView) sortKey(row components.Row, field string, cols []components.Column) string {
	for i, col := range cols {
		if col.Field == field && i < len(row.Cells) {
			return strings.ToLower(row.Cells[i])
		}
	}
	return ""
}

// handleInput processes view-specific key events.
func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	// If picker is active, route to picker
	if v.pickerActive {
		return v.handlePickerInput(event)
	}

	// If column picker is active, route to it
	if v.columnPickerActive {
		return v.handleColumnPickerInput(event)
	}

	switch event.Key() {
	case tcell.KeyDelete:
		v.showPowerPicker()
		return nil
	}

	switch event.Rune() {
	case 'm':
		v.toggleSelectMode()
		return nil

	case 'o':
		v.showColumnPicker()
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

// showPowerPicker displays the power management picker.
func (v *ListView) showPowerPicker() {
	// Get selected instances
	var targets []ec2.Instance
	var targetIDs []string

	if v.st.SelectMode() && v.st.SelectedCount() > 0 {
		targetIDs = v.st.SelectedIDs()
		v.mu.RLock()
		for _, id := range targetIDs {
			for _, inst := range v.filtered {
				if inst.InstanceID == id {
					targets = append(targets, inst)
					break
				}
			}
		}
		v.mu.RUnlock()
	} else {
		idx := v.st.GetSelectedIndex()
		if idx < 0 {
			return
		}
		v.mu.RLock()
		if idx >= len(v.filtered) {
			v.mu.RUnlock()
			return
		}
		targets = []ec2.Instance{v.filtered[idx]}
		targetIDs = []string{targets[0].InstanceID}
		v.mu.RUnlock()
	}

	if len(targets) == 0 {
		return
	}

	// Determine which options are applicable based on instance states
	hasRunning := false
	hasStopped := false
	for _, inst := range targets {
		if inst.State == "running" {
			hasRunning = true
		}
		if inst.State == "stopped" {
			hasStopped = true
		}
	}

	title := "Power Management"
	if len(targets) == 1 {
		name := targets[0].Name
		if name == "" {
			name = targets[0].InstanceID
		}
		title = fmt.Sprintf("Power: %s", name)
	} else {
		title = fmt.Sprintf("Power: %d instances", len(targets))
	}

	options := []components.PickerOption{
		{Key: "a", Label: "Start", Description: "Start stopped instances", Disabled: !hasStopped, Color: tcell.ColorGreen},
		{Key: "x", Label: "Stop", Description: "Stop running instances", Disabled: !hasRunning, Color: tcell.ColorYellow},
		{Key: "r", Label: "Reboot", Description: "Reboot running instances", Disabled: !hasRunning, Color: tcell.ColorYellow},
		{Key: "t", Label: "Terminate", Description: "Terminate instances (with confirmation)", Color: tcell.ColorRed},
		{Key: "f", Label: "Force Stop", Description: "Force stop (may cause data loss)", Disabled: !hasRunning, Color: tcell.ColorRed},
	}

	v.picker = components.NewPicker(title, options)
	v.picker.SetOnSelect(func(opt components.PickerOption) {
		v.pickerActive = false
		v.rebuildLayout()
		v.executePowerAction(opt.Key, targetIDs, targets)
	})
	v.picker.SetOnCancel(func() {
		v.pickerActive = false
		v.rebuildLayout()
		v.navigator.SetStatus("[gray]Cancelled")
	})
	v.picker.Show()
	v.pickerActive = true
	v.rebuildLayout()
}

// executePowerAction performs the selected power action.
func (v *ListView) executePowerAction(action string, ids []string, targets []ec2.Instance) {
	ctx := v.navigator.Context()
	svc := v.navigator.EC2Service()

	var actionName string
	var actionFn func(ctx context.Context, id string) error
	var needsConfirm bool

	switch action {
	case "a": // Start
		actionName = "Starting"
		actionFn = svc.StartInstance
	case "x": // Stop
		actionName = "Stopping"
		actionFn = svc.StopInstance
		needsConfirm = true
	case "r": // Reboot
		actionName = "Rebooting"
		actionFn = svc.RebootInstance
		needsConfirm = true
	case "t": // Terminate
		actionName = "Terminating"
		actionFn = svc.TerminateInstance
		needsConfirm = true
	case "f": // Force stop
		actionName = "Force stopping"
		actionFn = func(ctx context.Context, id string) error {
			return svc.ForceStopInstance(ctx, id)
		}
		needsConfirm = true
	default:
		return
	}

	doAction := func() {
		v.navigator.SetStatus(fmt.Sprintf("[yellow]%s %d instance(s)...", actionName, len(ids)))
		go func() {
			var errors []string
			for _, id := range ids {
				if err := actionFn(ctx, id); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %s", id, err.Error()))
				}
			}
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if len(errors) > 0 {
					v.navigator.SetStatus(fmt.Sprintf("[red]%s failed for %d instance(s)", actionName, len(errors)))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]%s %d instance(s)", actionName, len(ids)))
					v.Refresh(ctx)
				}
			})
		}()
	}

	if needsConfirm {
		var desc string
		if len(targets) == 1 {
			name := targets[0].Name
			if name == "" {
				name = targets[0].InstanceID
			}
			desc = name
		} else {
			desc = fmt.Sprintf("%d instances", len(targets))
		}
		v.navigator.ShowConfirm(fmt.Sprintf("%s %s?", actionName[:len(actionName)-3], desc), doAction)
	} else {
		doAction()
	}
}

// handlePickerInput routes input to the power picker.
func (v *ListView) handlePickerInput(event *tcell.EventKey) *tcell.EventKey {
	if v.picker != nil {
		handler := v.picker.InputHandler()
		handler(event, nil)
	}
	return nil
}

// showColumnPicker displays the column configuration picker.
func (v *ListView) showColumnPicker() {
	v.columnList = tview.NewList()
	v.columnList.SetBorder(true)
	v.columnList.SetBorderColor(tcell.ColorDodgerBlue)
	v.columnList.SetTitle(" Columns (Space=toggle, Enter=done) ")
	v.columnList.SetHighlightFullLine(true)
	v.columnList.ShowSecondaryText(false)

	v.rebuildColumnList()

	v.columnList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter, tcell.KeyEscape:
			v.columnPickerActive = false
			v.rebuildRows()
			v.navigator.RefreshShortcuts()
			// Set focus back to the table
			v.navigator.TviewApp().SetFocus(v.st.Table)
			return nil
		case tcell.KeyRune:
			if event.Rune() == ' ' {
				v.toggleColumnAt(v.columnList.GetCurrentItem())
				return nil
			}
		}
		return event
	})

	v.columnPickerActive = true
	v.rebuildLayout()
}

// rebuildColumnList updates the column list items.
func (v *ListView) rebuildColumnList() {
	v.columnList.Clear()

	// Standard columns
	for i, col := range v.columns {
		marker := "[ ]"
		if col.Enabled {
			marker = "[✓]"
		}
		idx := i // capture for closure
		v.columnList.AddItem(fmt.Sprintf("%s %s", marker, col.Title), "", 0, func() {
			v.toggleColumnAt(idx)
		})
	}

	// Tag columns
	for i, col := range v.tagColumns {
		marker := "[ ]"
		if col.Enabled {
			marker = "[✓]"
		}
		idx := len(v.columns) + i
		v.columnList.AddItem(fmt.Sprintf("%s %s", marker, col.Title), "", 0, func() {
			v.toggleColumnAt(idx)
		})
	}
}

// toggleColumnAt toggles the column at the given list index.
func (v *ListView) toggleColumnAt(idx int) {
	// Save current position
	currentIdx := v.columnList.GetCurrentItem()

	if idx < len(v.columns) {
		v.columns[idx].Enabled = !v.columns[idx].Enabled
	} else {
		tagIdx := idx - len(v.columns)
		if tagIdx < len(v.tagColumns) {
			v.tagColumns[tagIdx].Enabled = !v.tagColumns[tagIdx].Enabled
		}
	}
	v.rebuildColumnList()

	// Restore position
	v.columnList.SetCurrentItem(currentIdx)
}

// handleColumnPickerInput routes input to the column picker.
func (v *ListView) handleColumnPickerInput(event *tcell.EventKey) *tcell.EventKey {
	if v.columnList != nil {
		handler := v.columnList.InputHandler()
		handler(event, nil)
	}
	return nil
}

// toggleSelectMode switches multi-select mode on/off.
func (v *ListView) toggleSelectMode() {
	newMode := !v.st.SelectMode()
	v.st.SetSelectMode(newMode)

	if newMode {
		v.navigator.SetStatus("[yellow]Multi-select: Space=toggle, m=exit, Del=power mgmt")
	} else {
		v.st.ClearSelected()
		v.navigator.SetStatus("[green]Multi-select mode off")
	}
	v.rebuildLayout()
	v.navigator.RefreshShortcuts()
}

// rebuildLayout reconfigures the layout based on current state.
func (v *ListView) rebuildLayout() {
	v.layout.Clear()

	if v.columnPickerActive && v.columnList != nil {
		// Split: table left, column picker right
		v.layout.SetDirection(tview.FlexColumn)
		v.layout.AddItem(v.st.Table, 0, 2, true)
		v.layout.AddItem(v.columnList, 40, 0, true)
		return
	}

	v.layout.SetDirection(tview.FlexRow)

	if v.st.SelectMode() {
		// Split: table top, selection panel bottom
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
	// In select mode, Enter doesn't navigate
	if v.st.SelectMode() {
		v.st.ToggleSelected()
		return
	}

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
