package vpcview

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// DetailView displays details for a single VPC across multiple tabs.
type DetailView struct {
	tabs      *components.TabbedView
	navigator Navigator
	vpcID     string
	vpc       *ec2.VPC
	subnets   []ec2.Subnet

	// Page widgets
	overviewTable *tview.Table
	subnetsST     *components.SortableTable
	routePanel    *tview.TextView
	tagsTable     *tview.Table

	// Navigation targets for selectable rows (indexed by "tab:row")
	navTargets map[string]navigation.Route
}

// vpcSubnetColumns defines columns for the subnets table within VPC detail.
var vpcSubnetColumns = []components.Column{
	{Title: "SUBNET ID", Field: "subnet_id", Expansion: 1},
	{Title: "NAME", Field: "name", Expansion: 1},
	{Title: "CIDR", Field: "cidr", Expansion: 1},
	{Title: "AZ", Field: "az", Expansion: 1},
	{Title: "AVAILABLE IPs", Field: "available_ips", Expansion: 1},
	{Title: "PUBLIC IP", Field: "public_ip", Expansion: 1},
}

// NewDetailView creates a new VPC detail view.
func NewDetailView(navigator Navigator, vpcID string) *DetailView {
	v := &DetailView{
		navigator:  navigator,
		vpcID:      vpcID,
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

	// --- Subnets page (SortableTable) ---
	v.subnetsST = components.NewSortableTable(components.SortableTableConfig{
		Title:    "Subnets",
		Columns:  vpcSubnetColumns,
		OnStatus: navigator.SetStatus,
	})
	v.subnetsST.SetSelectedFunc(func(_ int, id string) {
		if id != "" {
			v.navigator.Navigate(navigation.Route{
				Resource:   "subnet-detail",
				ResourceID: id,
			})
		}
	})

	// --- Route Tables page (placeholder) ---
	v.routePanel = tview.NewTextView()
	v.routePanel.SetDynamicColors(true)
	v.routePanel.SetBorder(true)
	v.routePanel.SetTitle(" Route Tables ")
	v.routePanel.SetBorderColor(tcell.ColorDodgerBlue)
	v.routePanel.SetText("  Route tables not yet implemented")

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
		{Name: "Subnets", Content: v.subnetsST.Table},
		{Name: "Route Tables", Content: v.routePanel},
		{Name: "Tags", Content: v.tagsTable},
	})
	v.tabs.SetExtraInput(v.handleInput)

	return v
}

// Name returns the view identifier.
func (v *DetailView) Name() string {
	return "vpc-detail"
}

// Render returns the tview primitive.
func (v *DetailView) Render() tview.Primitive {
	return v.tabs.Widget()
}

// Refresh reloads VPC detail data from AWS.
func (v *DetailView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()

	// Get VPC by listing all and filtering
	vpcs, err := svc.ListVPCs(ctx)
	if err != nil {
		return err
	}
	for i := range vpcs {
		if vpcs[i].VPCID == v.vpcID {
			v.vpc = &vpcs[i]
			break
		}
	}
	if v.vpc == nil {
		return fmt.Errorf("VPC %s not found", v.vpcID)
	}

	// Get subnets for this VPC
	subnets, err := svc.ListSubnets(ctx, v.vpcID)
	if err != nil {
		return err
	}
	v.subnets = subnets

	v.renderOverview()
	v.renderSubnets()
	v.renderTags()

	return nil
}

// Shortcuts returns detail view shortcuts.
func (v *DetailView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "\u2190/\u2192", Label: "tabs"},
		{Key: "Enter", Label: "navigate"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
		{Key: "n", Label: "goto subnet"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *DetailView) FilterFields() []string {
	return nil
}

// HandleFilter applies a filter (no-op).
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
	if v.vpc == nil {
		return
	}
	vpc := v.vpc
	v.overviewTable.Clear()

	// Clear nav targets for this tab
	for k := range v.navTargets {
		if len(k) > 9 && k[:9] == "overview:" {
			delete(v.navTargets, k)
		}
	}

	isDefault := "No"
	if vpc.IsDefault {
		isDefault = "Yes"
	}

	row := 0

	v.setNavRow(v.overviewTable, "overview", row, "VPC ID:", vpc.VPCID, nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "Name:", orDash(vpc.Name), nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "CIDR:", orDash(vpc.CIDRBlock), nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "State:", orDash(vpc.State), nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "Default:", isDefault, nil)
	row++

	// Separator
	v.overviewTable.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	v.overviewTable.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	row++

	// Subnets section header
	v.overviewTable.SetCell(row, 0, tview.NewTableCell("  Subnets:").SetTextColor(tcell.ColorDodgerBlue).SetSelectable(false))
	v.overviewTable.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d in this VPC", len(v.subnets))).SetTextColor(tcell.ColorWhite).SetSelectable(false))
	row++

	// Each subnet as a navigable row
	for _, subnet := range v.subnets {
		label := fmt.Sprintf("  ├─ %s", orDash(subnet.Name))
		value := fmt.Sprintf("%s (%s, %s)", subnet.SubnetID, subnet.CIDRBlock, subnet.AZ)
		v.setNavRow(v.overviewTable, "overview", row, label, value, &navigation.Route{
			Resource:   "subnet-detail",
			ResourceID: subnet.SubnetID,
		})
		row++
	}

	v.overviewTable.SetTitle(fmt.Sprintf(" Overview: %s ", orDash(vpc.Name)))
}

func (v *DetailView) renderSubnets() {
	rows := make([]components.Row, len(v.subnets))
	for i, subnet := range v.subnets {
		publicIP := "No"
		publicColor := tcell.ColorLightGray
		if subnet.MapPublicIP {
			publicIP = "Yes"
			publicColor = tcell.ColorGreen
		}

		rows[i] = components.Row{
			ID: subnet.SubnetID,
			Cells: []string{
				subnet.SubnetID,
				orDash(subnet.Name),
				subnet.CIDRBlock,
				subnet.AZ,
				fmt.Sprintf("%d", subnet.AvailableIPs),
				publicIP,
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorYellow,
				publicColor,
			},
		}
	}

	v.subnetsST.SetRows(rows)
	v.subnetsST.SetSortKeyFn(func(row components.Row, field string) string {
		idx := -1
		for i, c := range vpcSubnetColumns {
			if c.Field == field {
				idx = i
				break
			}
		}
		if idx < 0 || idx >= len(row.Cells) {
			return ""
		}
		if field == "available_ips" {
			return fmt.Sprintf("%010s", row.Cells[idx])
		}
		return strings.ToLower(row.Cells[idx])
	})
}

func (v *DetailView) renderTags() {
	v.tagsTable.Clear()

	headers := []string{"KEY", "VALUE"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.tagsTable.SetCell(0, col, cell)
	}

	if v.vpc == nil {
		return
	}

	keys := make([]string, 0, len(v.vpc.Tags))
	for k := range v.vpc.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for row, k := range keys {
		v.tagsTable.SetCell(row+1, 0, tview.NewTableCell(k).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.tagsTable.SetCell(row+1, 1, tview.NewTableCell(v.vpc.Tags[k]).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
	}

	v.tagsTable.SetTitle(fmt.Sprintf(" Tags (%d) ", len(v.vpc.Tags)))
	v.tagsTable.SetFixed(1, 0)
}

// --- Event handlers ---

func (v *DetailView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'n':
		// Navigate to selected subnet from subnets tab
		if v.tabs.CurrentPage() == 1 {
			id := v.subnetsST.GetRowID()
			if id != "" {
				v.navigator.Navigate(navigation.Route{
					Resource:   "subnet-detail",
					ResourceID: id,
				})
				return nil
			}
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
