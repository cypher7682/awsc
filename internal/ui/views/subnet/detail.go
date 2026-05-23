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
	overviewPanel *tview.TextView
	instanceTable *tview.Table
	tagsTable     *tview.Table
}

// NewDetailView creates a new subnet detail view.
func NewDetailView(navigator Navigator, subnetID string) *DetailView {
	v := &DetailView{
		navigator: navigator,
		subnetID:  subnetID,
	}

	// --- Overview page ---
	v.overviewPanel = tview.NewTextView()
	v.overviewPanel.SetDynamicColors(true)
	v.overviewPanel.SetBorder(true)
	v.overviewPanel.SetTitle(" Overview ")
	v.overviewPanel.SetBorderColor(tcell.ColorDodgerBlue)

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
		{Key: "Enter", Label: "select instance"},
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

func (v *DetailView) renderOverview() {
	v.overviewPanel.Clear()
	if v.subnet == nil {
		return
	}
	s := v.subnet

	publicIP := "No"
	if s.MapPublicIP {
		publicIP = "[green]Yes[-]"
	}

	text := fmt.Sprintf(
		"[yellow]Subnet ID:[-]      %s\n"+
			"[yellow]Name:[-]           %s\n"+
			"[yellow]VPC ID:[-]         %s\n"+
			"[yellow]CIDR:[-]           %s\n"+
			"[yellow]AZ:[-]             %s\n"+
			"[yellow]Available IPs:[-]  %d\n"+
			"[yellow]Map Public IP:[-]  %s\n",
		s.SubnetID,
		orDash(s.Name),
		s.VPCID,
		s.CIDRBlock,
		s.AZ,
		s.AvailableIPs,
		publicIP,
	)

	if len(s.Tags) > 0 {
		text += "\n[yellow]Tags:[-]\n"
		keys := sortedKeys(s.Tags)
		for _, k := range keys {
			text += fmt.Sprintf("  [blue]%s[-] = %s\n", k, s.Tags[k])
		}
	}

	v.overviewPanel.SetText(text)
}

func (v *DetailView) renderInstances() {
	v.instanceTable.Clear()

	// Header row
	headers := []string{"INSTANCE ID", "NAME", "STATE", "TYPE", "PRIVATE IP"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		v.instanceTable.SetCell(0, col, cell)
	}

	for i, inst := range v.instances {
		row := i + 1
		stateColor := tcell.ColorWhite
		switch inst.State {
		case "running":
			stateColor = tcell.ColorGreen
		case "stopped":
			stateColor = tcell.ColorRed
		case "terminated":
			stateColor = tcell.ColorGray
		}

		cells := []struct {
			text  string
			color tcell.Color
		}{
			{inst.InstanceID, tcell.ColorWhite},
			{orDash(inst.Name), tcell.ColorLightGray},
			{inst.State, stateColor},
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

	v.instanceTable.SetFixed(1, 0)
}

func (v *DetailView) renderTags() {
	v.tagsTable.Clear()

	// Header
	for col, h := range []string{"KEY", "VALUE"} {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
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
			SetTextColor(tcell.ColorBlue).
			SetExpansion(1))
		v.tagsTable.SetCell(row, 1, tview.NewTableCell(v.subnet.Tags[k]).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
	}

	v.tagsTable.SetFixed(1, 0)
}

func (v *DetailView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		// Navigate to EC2 detail if on Instances tab
		if v.tabs.CurrentPage() == 1 {
			row, _ := v.instanceTable.GetSelection()
			idx := row - 1 // account for header
			if idx >= 0 && idx < len(v.instances) {
				v.navigator.Navigate(navigation.Route{
					Resource:   "ec2-detail",
					ResourceID: v.instances[idx].InstanceID,
				})
				return nil
			}
		}
	}

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
