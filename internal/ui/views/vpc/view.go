// Package vpcview provides the VPC view.
package vpcview

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

// Navigator is the interface for VPC views.
type Navigator interface {
	Navigate(route navigation.Route)
	EC2Service() *ec2.Service
	TviewApp() *tview.Application
	Context() context.Context
	SetStatus(text string)
}

// ListView displays VPCs.
type ListView struct {
	table     *tview.Table
	navigator Navigator

	mu   sync.RWMutex
	vpcs []ec2.VPC
}

// NewListView creates a new VPC list view.
func NewListView(navigator Navigator) *ListView {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetSelectable(true, false)
	table.SetTitle(" VPCs ")
	table.SetBorder(true)
	table.SetBorderColor(tcell.ColorDodgerBlue)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	v := &ListView{
		table:     table,
		navigator: navigator,
	}

	table.SetInputCapture(v.handleInput)
	table.SetSelectedFunc(v.onSelect)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "vpc"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.table
}

// Refresh reloads VPC data from AWS.
func (v *ListView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()
	vpcs, err := svc.ListVPCs(ctx)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.vpcs = vpcs
	v.mu.Unlock()

	v.renderTable()
	return nil
}

// Shortcuts returns VPC-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "subnets"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *ListView) FilterFields() []string {
	return []string{"vpc_id", "name", "cidr", "state", "default"}
}

// HandleFilter applies a filter.
func (v *ListView) HandleFilter(_ string) {}

// renderTable rebuilds the table display.
func (v *ListView) renderTable() {
	v.mu.RLock()
	vpcs := v.vpcs
	v.mu.RUnlock()

	v.table.Clear()

	headers := []string{"VPC ID", "NAME", "CIDR", "STATE", "DEFAULT"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.table.SetCell(0, col, cell)
	}

	for row, vpc := range vpcs {
		name := vpc.Name
		if name == "" {
			name = "-"
		}
		isDefault := "No"
		if vpc.IsDefault {
			isDefault = "[green]Yes"
		}

		v.table.SetCell(row+1, 0, tview.NewTableCell(vpc.VPCID).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.table.SetCell(row+1, 1, tview.NewTableCell(name).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row+1, 2, tview.NewTableCell(vpc.CIDRBlock).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row+1, 3, tview.NewTableCell(strings.ToUpper(vpc.State)).SetTextColor(tcell.ColorGreen).SetExpansion(1))
		v.table.SetCell(row+1, 4, tview.NewTableCell(isDefault).SetExpansion(1))
	}

	v.table.SetTitle(fmt.Sprintf(" VPCs (%d) ", len(vpcs)))
}

func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
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

func (v *ListView) onSelect(row, _ int) {
	if row <= 0 {
		return
	}
	v.mu.RLock()
	idx := row - 1
	if idx >= len(v.vpcs) {
		v.mu.RUnlock()
		return
	}
	vpc := v.vpcs[idx]
	v.mu.RUnlock()

	v.navigator.Navigate(navigation.Route{
		Resource:   "subnet",
		Params:     map[string]string{"vpc_id": vpc.VPCID},
	})
}
