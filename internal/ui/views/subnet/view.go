// Package subnetview provides the Subnet view.
package subnetview

import (
	"context"
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// Navigator is the interface for subnet views.
type Navigator interface {
	Navigate(route navigation.Route)
	EC2Service() *ec2.Service
	TviewApp() *tview.Application
	Context() context.Context
	SetStatus(text string)
}

// ListView displays subnets.
type ListView struct {
	table     *tview.Table
	navigator Navigator
	vpcID     string // optional filter by VPC

	mu      sync.RWMutex
	subnets []ec2.Subnet
}

// NewListView creates a new subnet list view.
func NewListView(navigator Navigator, vpcID string) *ListView {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetSelectable(true, false)
	table.SetBorder(true)
	table.SetBorderColor(tcell.ColorDodgerBlue)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	title := " Subnets "
	if vpcID != "" {
		title = fmt.Sprintf(" Subnets (%s) ", vpcID)
	}
	table.SetTitle(title)

	v := &ListView{
		table:     table,
		navigator: navigator,
		vpcID:     vpcID,
	}

	table.SetInputCapture(v.handleInput)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "subnet"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.table
}

// Refresh reloads subnet data from AWS.
func (v *ListView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()
	subnets, err := svc.ListSubnets(ctx, v.vpcID)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.subnets = subnets
	v.mu.Unlock()

	v.renderTable()
	return nil
}

// Shortcuts returns subnet-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "v", Label: "goto VPC"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *ListView) FilterFields() []string {
	return []string{"subnet_id", "name", "vpc_id", "cidr", "az", "public_ip"}
}

// HandleFilter applies a filter.
func (v *ListView) HandleFilter(_ string) {}

// renderTable rebuilds the table display.
func (v *ListView) renderTable() {
	v.mu.RLock()
	subnets := v.subnets
	v.mu.RUnlock()

	v.table.Clear()

	headers := []string{"SUBNET ID", "NAME", "VPC ID", "CIDR", "AZ", "AVAILABLE IPs", "PUBLIC IP"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.table.SetCell(0, col, cell)
	}

	for row, sn := range subnets {
		name := sn.Name
		if name == "" {
			name = "-"
		}
		publicIP := "No"
		if sn.MapPublicIP {
			publicIP = "[green]Yes"
		}

		v.table.SetCell(row+1, 0, tview.NewTableCell(sn.SubnetID).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.table.SetCell(row+1, 1, tview.NewTableCell(name).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row+1, 2, tview.NewTableCell(sn.VPCID).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row+1, 3, tview.NewTableCell(sn.CIDRBlock).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row+1, 4, tview.NewTableCell(sn.AZ).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row+1, 5, tview.NewTableCell(fmt.Sprintf("%d", sn.AvailableIPs)).SetTextColor(tcell.ColorYellow).SetExpansion(1))
		v.table.SetCell(row+1, 6, tview.NewTableCell(publicIP).SetExpansion(1))
	}

	v.table.SetTitle(fmt.Sprintf(" Subnets (%d) ", len(subnets)))
}

func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'v':
		row, _ := v.table.GetSelection()
		if row <= 0 {
			return event
		}
		v.mu.RLock()
		idx := row - 1
		if idx >= len(v.subnets) {
			v.mu.RUnlock()
			return event
		}
		subnet := v.subnets[idx]
		v.mu.RUnlock()

		v.navigator.Navigate(navigation.Route{
			Resource:   "vpc",
			ResourceID: subnet.VPCID,
		})
		return nil

	case 'R':
		go func() {
			v.Refresh(v.navigator.Context())
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				v.navigator.SetStatus("[green]Refreshed")
			})
		}()
		return nil
	}
	return event
}
