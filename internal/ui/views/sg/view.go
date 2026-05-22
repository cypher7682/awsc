// Package sgview provides the Security Groups view.
package sgview

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

// Navigator is the interface for SG views.
type Navigator interface {
	Navigate(route navigation.Route)
	EC2Service() *ec2.Service
	TviewApp() *tview.Application
	Context() context.Context
	SetStatus(text string)
}

// ListView displays security groups.
type ListView struct {
	table     *tview.Table
	navigator Navigator

	mu     sync.RWMutex
	groups []ec2.SecurityGroup
	filter string
}

// NewListView creates a new security groups list view.
func NewListView(navigator Navigator) *ListView {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetSelectable(true, false)
	table.SetTitle(" Security Groups ")
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
	return "sg"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.table
}

// Refresh reloads security group data from AWS.
func (v *ListView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()
	groups, err := svc.ListSecurityGroups(ctx, nil)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.groups = groups
	v.mu.Unlock()

	v.renderTable()
	return nil
}

// Shortcuts returns SG-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "rules"},
		{Key: "v", Label: "goto VPC"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *ListView) FilterFields() []string {
	return []string{"sg_id", "name", "vpc_id", "description"}
}

// HandleFilter applies a filter.
func (v *ListView) HandleFilter(expression string) {
	v.mu.Lock()
	v.filter = expression
	v.mu.Unlock()
	v.renderTable()
}

// renderTable rebuilds the table display.
func (v *ListView) renderTable() {
	v.mu.RLock()
	groups := v.groups
	filter := v.filter
	v.mu.RUnlock()

	v.table.Clear()

	headers := []string{"SG ID", "NAME", "VPC ID", "INBOUND RULES", "OUTBOUND RULES", "DESCRIPTION"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.table.SetCell(0, col, cell)
	}

	row := 1
	for _, sg := range groups {
		if filter != "" {
			lower := strings.ToLower(filter)
			if !strings.Contains(strings.ToLower(sg.GroupName), lower) &&
				!strings.Contains(strings.ToLower(sg.GroupID), lower) &&
				!strings.Contains(strings.ToLower(sg.Description), lower) {
				continue
			}
		}

		desc := sg.Description
		if len(desc) > 40 {
			desc = desc[:40] + "..."
		}

		v.table.SetCell(row, 0, tview.NewTableCell(sg.GroupID).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.table.SetCell(row, 1, tview.NewTableCell(sg.GroupName).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row, 2, tview.NewTableCell(sg.VPCID).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf("%d", len(sg.IngressRules))).SetTextColor(tcell.ColorYellow).SetExpansion(1))
		v.table.SetCell(row, 4, tview.NewTableCell(fmt.Sprintf("%d", len(sg.EgressRules))).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row, 5, tview.NewTableCell(desc).SetTextColor(tcell.ColorGray).SetExpansion(1))
		row++
	}

	v.table.SetTitle(fmt.Sprintf(" Security Groups (%d) ", row-1))
}

// handleInput processes key events.
func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'v':
		row, _ := v.table.GetSelection()
		if row <= 0 {
			return event
		}
		sg := v.getSGAtRow(row)
		if sg != nil && sg.VPCID != "" {
			v.navigator.Navigate(navigation.Route{
				Resource:   "vpc",
				ResourceID: sg.VPCID,
			})
		}
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

// onSelect handles Enter on a security group.
func (v *ListView) onSelect(row, _ int) {
	if row <= 0 {
		return
	}
	sg := v.getSGAtRow(row)
	if sg == nil {
		return
	}
	v.navigator.Navigate(navigation.Route{
		Resource:   "sg-detail",
		ResourceID: sg.GroupID,
	})
}

func (v *ListView) getSGAtRow(row int) *ec2.SecurityGroup {
	v.mu.RLock()
	defer v.mu.RUnlock()

	idx := row - 1
	if idx < 0 || idx >= len(v.groups) {
		return nil
	}
	return &v.groups[idx]
}
