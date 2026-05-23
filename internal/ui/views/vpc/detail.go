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
	overviewPanel *tview.TextView
	subnetsTable  *tview.Table
	routePanel    *tview.TextView
	tagsTable     *tview.Table
}

// NewDetailView creates a new VPC detail view.
func NewDetailView(navigator Navigator, vpcID string) *DetailView {
	v := &DetailView{
		navigator: navigator,
		vpcID:     vpcID,
	}

	// --- Overview page ---
	v.overviewPanel = tview.NewTextView()
	v.overviewPanel.SetDynamicColors(true)
	v.overviewPanel.SetBorder(true)
	v.overviewPanel.SetTitle(" Overview ")
	v.overviewPanel.SetBorderColor(tcell.ColorDodgerBlue)

	// --- Subnets page ---
	v.subnetsTable = tview.NewTable()
	v.subnetsTable.SetBorders(false)
	v.subnetsTable.SetSelectable(true, false)
	v.subnetsTable.SetBorder(true)
	v.subnetsTable.SetTitle(" Subnets ")
	v.subnetsTable.SetBorderColor(tcell.ColorDodgerBlue)
	v.subnetsTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

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
		{Name: "Overview", Content: v.overviewPanel},
		{Name: "Subnets", Content: v.subnetsTable},
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

// --- Render methods ---

func (v *DetailView) renderOverview() {
	if v.vpc == nil {
		return
	}
	vpc := v.vpc

	isDefault := "No"
	if vpc.IsDefault {
		isDefault = "Yes"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("  [dodgerblue]VPC ID:[-]      %s\n", vpc.VPCID))
	b.WriteString(fmt.Sprintf("  [dodgerblue]Name:[-]        %s\n", orDash(vpc.Name)))
	b.WriteString(fmt.Sprintf("  [dodgerblue]CIDR:[-]        %s\n", orDash(vpc.CIDRBlock)))
	b.WriteString(fmt.Sprintf("  [dodgerblue]State:[-]       %s\n", orDash(vpc.State)))
	b.WriteString(fmt.Sprintf("  [dodgerblue]Default:[-]     %s\n", isDefault))
	b.WriteString("\n")

	// Show all tags
	if len(vpc.Tags) > 0 {
		b.WriteString("  [dodgerblue]Tags:[-]\n")
		keys := make([]string, 0, len(vpc.Tags))
		for k := range vpc.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("    %s = %s\n", k, vpc.Tags[k]))
		}
	}

	v.overviewPanel.SetText(b.String())
	v.overviewPanel.SetTitle(fmt.Sprintf(" Overview: %s ", orDash(vpc.Name)))
}

func (v *DetailView) renderSubnets() {
	v.subnetsTable.Clear()

	headers := []string{"SUBNET ID", "NAME", "CIDR", "AZ", "AVAILABLE IPS", "PUBLIC IP"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.subnetsTable.SetCell(0, col, cell)
	}

	for row, subnet := range v.subnets {
		publicIP := "No"
		if subnet.MapPublicIP {
			publicIP = "Yes"
		}
		v.subnetsTable.SetCell(row+1, 0, tview.NewTableCell(subnet.SubnetID).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.subnetsTable.SetCell(row+1, 1, tview.NewTableCell(orDash(subnet.Name)).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.subnetsTable.SetCell(row+1, 2, tview.NewTableCell(subnet.CIDRBlock).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.subnetsTable.SetCell(row+1, 3, tview.NewTableCell(subnet.AZ).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.subnetsTable.SetCell(row+1, 4, tview.NewTableCell(fmt.Sprintf("%d", subnet.AvailableIPs)).SetTextColor(tcell.ColorYellow).SetExpansion(1))
		v.subnetsTable.SetCell(row+1, 5, tview.NewTableCell(publicIP).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
	}

	v.subnetsTable.SetTitle(fmt.Sprintf(" Subnets (%d) ", len(v.subnets)))
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
}

// --- Event handlers ---

func (v *DetailView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'n':
		// Navigate to selected subnet
		row, _ := v.subnetsTable.GetSelection()
		if row > 0 && row-1 < len(v.subnets) {
			v.navigator.Navigate(navigation.Route{
				Resource:   "subnet",
				ResourceID: v.subnets[row-1].SubnetID,
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
