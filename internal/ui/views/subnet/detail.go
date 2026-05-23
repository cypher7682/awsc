package subnetview

import (
	"context"
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// DetailView displays details for a single subnet across multiple tabs.
type DetailView struct {
	tabs      *components.TabbedView
	navigator Navigator
	subnetID  string
	subnet    *ec2.Subnet
	instances []ec2.Instance

	// Page widgets
	overviewTable *tview.Table
	instanceTable *tview.Table
	tagsTable     *tview.Table

	// Navigation targets for selectable rows (indexed by "tab:row")
	navTargets map[string]navigation.Route
}

// NewDetailView creates a new subnet detail view.
func NewDetailView(navigator Navigator, subnetID string) *DetailView {
	v := &DetailView{
		navigator:  navigator,
		subnetID:   subnetID,
		navTargets: make(map[string]navigation.Route),
	}

	// --- Overview page ---
	v.overviewTable = tview.NewTable()
	v.overviewTable.SetBorders(false)
	v.overviewTable.SetSelectable(true, false)
	v.overviewTable.SetBorder(true)
	v.overviewTable.SetTitle(" Overview ")
	v.overviewTable.SetBorderColor(tcell.ColorDodgerBlue)
	v.overviewTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDarkSlateGray).
		Foreground(tcell.ColorWhite))
	v.overviewTable.SetSelectedFunc(func(row, _ int) {
		v.navigateRow("overview", row)
	})

	// --- Instances page ---
	v.instanceTable = tview.NewTable()
	v.instanceTable.SetBorders(false)
	v.instanceTable.SetSelectable(true, false)
	v.instanceTable.SetBorder(true)
	v.instanceTable.SetTitle(" Instances ")
	v.instanceTable.SetBorderColor(tcell.ColorDodgerBlue)
	v.instanceTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))
	v.instanceTable.SetSelectedFunc(func(row, _ int) {
		if row > 0 && row-1 < len(v.instances) {
			v.navigator.Navigate(navigation.Route{
				Resource:   "ec2-detail",
				ResourceID: v.instances[row-1].InstanceID,
			})
		}
	})

	// --- Tags page ---
	v.tagsTable = tview.NewTable()
	v.tagsTable.SetBorders(false)
	v.tagsTable.SetSelectable(true, false)
	v.tagsTable.SetBorder(true)
	v.tagsTable.SetTitle(" Tags ")
	v.tagsTable.SetBorderColor(tcell.ColorDodgerBlue)
	v.tagsTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	// Build tabbed view
	v.tabs = components.NewTabbedView([]components.TabPage{
		{Name: "Overview", Content: v.overviewTable},
		{Name: "Instances", Content: v.instanceTable},
		{Name: "Tags", Content: v.tagsTable},
	})
	v.tabs.SetExtraInput(v.handleInput)

	return v
}

// Name returns the view identifier.
func (v *DetailView) Name() string {
	return "subnet-detail"
}

// Render returns the tview primitive.
func (v *DetailView) Render() tview.Primitive {
	return v.tabs.Widget()
}

// Refresh reloads subnet data from AWS.
func (v *DetailView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()

	// Get all subnets and find the matching one
	subnets, err := svc.ListSubnets(ctx, "")
	if err != nil {
		return fmt.Errorf("listing subnets: %w", err)
	}

	var found *ec2.Subnet
	for i := range subnets {
		if subnets[i].SubnetID == v.subnetID {
			found = &subnets[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("subnet %s not found", v.subnetID)
	}
	v.subnet = found

	// Get instances and filter to this subnet
	allInstances, err := svc.ListInstances(ctx, nil)
	if err != nil {
		return fmt.Errorf("listing instances: %w", err)
	}

	v.instances = nil
	for _, inst := range allInstances {
		if inst.SubnetID == v.subnetID {
			v.instances = append(v.instances, inst)
		}
	}

	// Render all tabs
	v.renderOverview()
	v.renderInstances()
	v.renderTags()

	return nil
}

// Shortcuts returns subnet-detail-specific shortcuts.
func (v *DetailView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "←/→", Label: "tab"},
		{Key: "Enter", Label: "navigate"},
		{Key: "v", Label: "goto VPC"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns nil (no filtering on detail view).
func (v *DetailView) FilterFields() []string {
	return nil
}

// HandleFilter is a no-op for the detail view.
func (v *DetailView) HandleFilter(_ string) {}

// --- Navigation helpers ---

// setNavRow adds a key-value row to a table. If route is non-nil, the value gets
// a ↩ indicator and the row is selectable for navigation.
func (v *DetailView) setNavRow(table *tview.Table, tab string, row int, label, value string, route *navigation.Route) {
	// Label column (non-selectable)
	cell0 := tview.NewTableCell("  " + label).
		SetTextColor(tcell.ColorDodgerBlue).
		SetSelectable(false)
	table.SetCell(row, 0, cell0)

	// Value column
	valueText := value
	valueColor := tcell.ColorWhite
	if route != nil {
		valueText = value + " [gray]↩[-]"
		valueColor = tcell.ColorSkyblue
		key := fmt.Sprintf("%s:%d", tab, row)
		v.navTargets[key] = *route
	}

	cell1 := tview.NewTableCell(valueText).
		SetTextColor(valueColor).
		SetExpansion(1).
		SetSelectable(route != nil)
	table.SetCell(row, 1, cell1)
}

// navigateRow navigates to the route for the given tab and row.
func (v *DetailView) navigateRow(tab string, row int) {
	key := fmt.Sprintf("%s:%d", tab, row)
	if route, ok := v.navTargets[key]; ok {
		v.navigator.Navigate(route)
	}
}

// --- Render methods ---

func (v *DetailView) renderOverview() {
	if v.subnet == nil {
		return
	}
	s := v.subnet
	v.overviewTable.Clear()

	// Clear nav targets for this tab
	for k := range v.navTargets {
		if len(k) > 9 && k[:9] == "overview:" {
			delete(v.navTargets, k)
		}
	}

	publicIP := "No"
	if s.MapPublicIP {
		publicIP = "Yes"
	}

	row := 0

	v.setNavRow(v.overviewTable, "overview", row, "Subnet ID:", s.SubnetID, nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "Name:", orDash(s.Name), nil)
	row++

	// Navigable: VPC
	v.setNavRow(v.overviewTable, "overview", row, "VPC ID:", s.VPCID, &navigation.Route{
		Resource:   "vpc-detail",
		ResourceID: s.VPCID,
	})
	row++

	v.setNavRow(v.overviewTable, "overview", row, "CIDR:", s.CIDRBlock, nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "AZ:", s.AZ, nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "Available IPs:", fmt.Sprintf("%d", s.AvailableIPs), nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "Map Public IP:", publicIP, nil)
	row++

	// Separator
	v.overviewTable.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	v.overviewTable.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	row++

	// Instances section header
	v.overviewTable.SetCell(row, 0, tview.NewTableCell("  Instances:").SetTextColor(tcell.ColorDodgerBlue).SetSelectable(false))
	v.overviewTable.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d in this subnet", len(v.instances))).SetTextColor(tcell.ColorWhite).SetSelectable(false))
	row++

	// Each instance as a navigable row
	for _, inst := range v.instances {
		label := fmt.Sprintf("  ├─ %s", orDash(inst.Name))
		value := fmt.Sprintf("%s (%s, %s)", inst.InstanceID, inst.Type, inst.State)
		v.setNavRow(v.overviewTable, "overview", row, label, value, &navigation.Route{
			Resource:   "ec2-detail",
			ResourceID: inst.InstanceID,
		})
		row++
	}

	v.overviewTable.SetTitle(fmt.Sprintf(" Overview: %s ", orDash(s.Name)))
}

func (v *DetailView) renderInstances() {
	v.instanceTable.Clear()

	// Header row
	headers := []string{"INSTANCE ID", "NAME", "STATE", "TYPE", "PRIVATE IP"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.instanceTable.SetCell(0, col, cell)
	}

	for i, inst := range v.instances {
		row := i + 1
		instStateColor := tcell.ColorWhite
		switch inst.State {
		case "running":
			instStateColor = tcell.ColorGreen
		case "stopped":
			instStateColor = tcell.ColorRed
		case "terminated":
			instStateColor = tcell.ColorGray
		case "pending", "stopping":
			instStateColor = tcell.ColorYellow
		}

		cells := []struct {
			text  string
			color tcell.Color
		}{
			{inst.InstanceID, tcell.ColorWhite},
			{orDash(inst.Name), tcell.ColorLightGray},
			{inst.State, instStateColor},
			{inst.Type, tcell.ColorLightGray},
			{orDash(inst.PrivateIP), tcell.ColorLightGray},
		}

		for col, c := range cells {
			cell := tview.NewTableCell(c.text).
				SetTextColor(c.color).
				SetExpansion(1)
			v.instanceTable.SetCell(row, col, cell)
		}
	}

	v.instanceTable.SetTitle(fmt.Sprintf(" Instances (%d) ", len(v.instances)))
	v.instanceTable.SetFixed(1, 0)
}

func (v *DetailView) renderTags() {
	v.tagsTable.Clear()

	// Header
	for col, h := range []string{"KEY", "VALUE"} {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.tagsTable.SetCell(0, col, cell)
	}

	if v.subnet == nil {
		return
	}

	keys := sortedKeys(v.subnet.Tags)
	for i, k := range keys {
		row := i + 1
		v.tagsTable.SetCell(row, 0, tview.NewTableCell(k).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		v.tagsTable.SetCell(row, 1, tview.NewTableCell(v.subnet.Tags[k]).
			SetTextColor(tcell.ColorLightGray).
			SetExpansion(1))
	}

	v.tagsTable.SetTitle(fmt.Sprintf(" Tags (%d) ", len(v.subnet.Tags)))
	v.tagsTable.SetFixed(1, 0)
}

// --- Event handlers ---

func (v *DetailView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'v':
		if v.subnet != nil {
			v.navigator.Navigate(navigation.Route{
				Resource:   "vpc-detail",
				ResourceID: v.subnet.VPCID,
			})
			return nil
		}
	}
	return event
}

// --- Helpers ---

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
