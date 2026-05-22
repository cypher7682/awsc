package ec2view

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// DetailView displays details for a single EC2 instance.
type DetailView struct {
	flex       *tview.Flex
	infoPanel  *tview.TextView
	sgTable    *tview.Table
	navigator  Navigator
	instanceID string
	instance   *ec2.Instance
	sgs        []ec2.SecurityGroup
}

// NewDetailView creates a new EC2 detail view.
func NewDetailView(navigator Navigator, instanceID string) *DetailView {
	info := tview.NewTextView()
	info.SetDynamicColors(true)
	info.SetBorder(true)
	info.SetTitle(" Instance Details ")
	info.SetBorderColor(tcell.ColorDodgerBlue)

	sgTable := tview.NewTable()
	sgTable.SetBorders(false)
	sgTable.SetSelectable(true, false)
	sgTable.SetTitle(" Security Groups ")
	sgTable.SetBorder(true)
	sgTable.SetBorderColor(tcell.ColorDodgerBlue)
	sgTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(info, 14, 0, false)
	flex.AddItem(sgTable, 0, 1, true)

	v := &DetailView{
		flex:       flex,
		infoPanel:  info,
		sgTable:    sgTable,
		navigator:  navigator,
		instanceID: instanceID,
	}

	sgTable.SetSelectedFunc(v.onSGSelect)
	sgTable.SetInputCapture(v.handleInput)

	return v
}

// Name returns the view identifier.
func (v *DetailView) Name() string {
	return "ec2-detail"
}

// Render returns the tview primitive.
func (v *DetailView) Render() tview.Primitive {
	return v.flex
}

// Refresh reloads instance detail data from AWS.
func (v *DetailView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()

	instance, err := svc.GetInstance(ctx, v.instanceID)
	if err != nil {
		return err
	}
	v.instance = instance

	// Load security groups
	if len(instance.SecurityGroupIDs) > 0 {
		sgs, err := svc.ListSecurityGroups(ctx, instance.SecurityGroupIDs)
		if err != nil {
			return err
		}
		v.sgs = sgs
	}

	v.renderInfo()
	v.renderSGs()
	return nil
}

// Shortcuts returns detail view shortcuts.
func (v *DetailView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "open SG"},
		{Key: "v", Label: "goto VPC"},
		{Key: "n", Label: "goto subnet"},
		{Key: "t", Label: "terminate"},
		{Key: "r", Label: "reboot"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *DetailView) FilterFields() []string {
	return []string{"sg_name", "sg_id", "port", "protocol", "source"}
}

// HandleFilter applies a filter.
func (v *DetailView) HandleFilter(_ string) {}

// renderInfo renders the instance details panel.
func (v *DetailView) renderInfo() {
	if v.instance == nil {
		return
	}
	inst := v.instance

	stColor := "green"
	switch inst.State {
	case "stopped":
		stColor = "red"
	case "terminated", "shutting-down":
		stColor = "gray"
	case "pending", "stopping":
		stColor = "yellow"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[dodgerblue]Instance ID:[-]  %s\n", inst.InstanceID))
	b.WriteString(fmt.Sprintf("[dodgerblue]Name:[-]         %s\n", orDash(inst.Name)))
	b.WriteString(fmt.Sprintf("[dodgerblue]State:[-]        [%s]%s[-]\n", stColor, inst.State))
	b.WriteString(fmt.Sprintf("[dodgerblue]Type:[-]         %s\n", inst.Type))
	b.WriteString(fmt.Sprintf("[dodgerblue]Private IP:[-]   %s\n", orDash(inst.PrivateIP)))
	b.WriteString(fmt.Sprintf("[dodgerblue]Public IP:[-]    %s\n", orDash(inst.PublicIP)))
	b.WriteString(fmt.Sprintf("[dodgerblue]VPC:[-]          %s\n", orDash(inst.VPCID)))
	b.WriteString(fmt.Sprintf("[dodgerblue]Subnet:[-]       %s\n", orDash(inst.SubnetID)))
	b.WriteString(fmt.Sprintf("[dodgerblue]AZ:[-]           %s\n", orDash(inst.AZ)))
	b.WriteString(fmt.Sprintf("[dodgerblue]AMI:[-]          %s\n", orDash(inst.AMI)))
	b.WriteString(fmt.Sprintf("[dodgerblue]Key Name:[-]     %s\n", orDash(inst.KeyName)))
	b.WriteString(fmt.Sprintf("[dodgerblue]Launch Time:[-]  %s\n", inst.LaunchTime.Format("2006-01-02 15:04:05 UTC")))

	if len(inst.Tags) > 0 {
		b.WriteString("\n[dodgerblue]Tags:[-]\n")
		for k, val := range inst.Tags {
			if k != "Name" {
				b.WriteString(fmt.Sprintf("  [gray]%s:[-] %s\n", k, val))
			}
		}
	}

	v.infoPanel.SetText(b.String())
}

// renderSGs renders the security groups table.
func (v *DetailView) renderSGs() {
	v.sgTable.Clear()

	// Header
	headers := []string{"SG ID", "NAME", "PROTOCOL", "PORTS", "SOURCE", "DESCRIPTION"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.sgTable.SetCell(0, col, cell)
	}

	row := 1
	for _, sg := range v.sgs {
		if len(sg.IngressRules) == 0 {
			// Show the SG even without rules
			v.sgTable.SetCell(row, 0, tview.NewTableCell(sg.GroupID).SetTextColor(tcell.ColorWhite))
			v.sgTable.SetCell(row, 1, tview.NewTableCell(sg.GroupName).SetTextColor(tcell.ColorLightGray))
			v.sgTable.SetCell(row, 2, tview.NewTableCell("-").SetTextColor(tcell.ColorGray))
			v.sgTable.SetCell(row, 3, tview.NewTableCell("-").SetTextColor(tcell.ColorGray))
			v.sgTable.SetCell(row, 4, tview.NewTableCell("-").SetTextColor(tcell.ColorGray))
			v.sgTable.SetCell(row, 5, tview.NewTableCell(sg.Description).SetTextColor(tcell.ColorGray))
			row++
		}
		for _, rule := range sg.IngressRules {
			ports := formatPorts(rule.FromPort, rule.ToPort)
			v.sgTable.SetCell(row, 0, tview.NewTableCell(sg.GroupID).SetTextColor(tcell.ColorWhite))
			v.sgTable.SetCell(row, 1, tview.NewTableCell(sg.GroupName).SetTextColor(tcell.ColorLightGray))
			v.sgTable.SetCell(row, 2, tview.NewTableCell(rule.Protocol).SetTextColor(tcell.ColorLightGray))
			v.sgTable.SetCell(row, 3, tview.NewTableCell(ports).SetTextColor(tcell.ColorYellow))
			v.sgTable.SetCell(row, 4, tview.NewTableCell(rule.Source).SetTextColor(tcell.ColorLightGray))
			v.sgTable.SetCell(row, 5, tview.NewTableCell(orDash(rule.Description)).SetTextColor(tcell.ColorGray))
			row++
		}
	}

	v.sgTable.SetTitle(fmt.Sprintf(" Security Groups (%d rules) ", row-1))
}

// handleInput processes key events for the detail view.
func (v *DetailView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if v.instance == nil {
		return event
	}

	switch event.Rune() {
	case 'v':
		if v.instance.VPCID != "" {
			v.navigator.Navigate(navigation.Route{
				Resource:   "vpc",
				ResourceID: v.instance.VPCID,
			})
		}
		return nil
	case 'n':
		if v.instance.SubnetID != "" {
			v.navigator.Navigate(navigation.Route{
				Resource:   "subnet",
				ResourceID: v.instance.SubnetID,
			})
		}
		return nil
	case 't':
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Terminating %s...", v.instanceID))
		go func() {
			err := v.navigator.EC2Service().TerminateInstance(v.navigator.Context(), v.instanceID)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed: %s", err.Error()))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]Terminated %s", v.instanceID))
				}
			})
		}()
		return nil
	case 'r':
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Rebooting %s...", v.instanceID))
		go func() {
			err := v.navigator.EC2Service().RebootInstance(v.navigator.Context(), v.instanceID)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed: %s", err.Error()))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]Rebooting %s", v.instanceID))
				}
			})
		}()
		return nil
	}

	return event
}

// onSGSelect handles selecting a security group row.
func (v *DetailView) onSGSelect(row, _ int) {
	if row <= 0 || row > len(v.sgs) {
		return
	}
	// Navigate to the security group detail
	sgID := v.sgTable.GetCell(row, 0).Text
	v.navigator.Navigate(navigation.Route{
		Resource:   "sg",
		ResourceID: sgID,
	})
}

// formatPorts returns a human-readable port range string.
func formatPorts(from, to int32) string {
	if from == 0 && to == 0 {
		return "All"
	}
	if from == to {
		return fmt.Sprintf("%d", from)
	}
	return fmt.Sprintf("%d-%d", from, to)
}
