// Package vpcview provides the VPC view.
package vpcview

import (
	"context"
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

var vpcColumns = []components.Column{
	{Title: "VPC ID", Field: "vpc_id", Expansion: 1},
	{Title: "NAME", Field: "name", Expansion: 1},
	{Title: "CIDR", Field: "cidr", Expansion: 1},
	{Title: "STATE", Field: "state", Expansion: 1},
	{Title: "DEFAULT", Field: "default", Expansion: 1},
}

// ListView displays VPCs.
type ListView struct {
	st        *components.SortableTable
	navigator Navigator

	mu   sync.RWMutex
	vpcs []ec2.VPC
}

// NewListView creates a new VPC list view.
func NewListView(navigator Navigator) *ListView {
	v := &ListView{
		navigator: navigator,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "VPCs",
		Columns:  vpcColumns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)
	v.st.SetSelectedFunc(v.onSelect)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "vpc"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.st.Table
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

	v.rebuildRows()
	return nil
}

// Shortcuts returns VPC-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "subnets"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
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

// rebuildRows converts VPCs into table rows and applies sort.
func (v *ListView) rebuildRows() {
	v.mu.RLock()
	vpcs := make([]ec2.VPC, len(v.vpcs))
	copy(vpcs, v.vpcs)
	v.mu.RUnlock()

	rows := make([]components.Row, len(vpcs))
	for i, vpc := range vpcs {
		name := vpc.Name
		if name == "" {
			name = "-"
		}
		isDefault := "No"
		defaultColor := tcell.ColorLightGray
		if vpc.IsDefault {
			isDefault = "Yes"
			defaultColor = tcell.ColorGreen
		}

		rows[i] = components.Row{
			ID: vpc.VPCID,
			Cells: []string{
				vpc.VPCID,
				name,
				vpc.CIDRBlock,
				strings.ToUpper(vpc.State),
				isDefault,
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorGreen,
				defaultColor,
			},
		}
	}

	v.st.SetRows(rows)
	v.st.SetSortKeyFn(func(row components.Row, field string) string {
		return vpcSortKey(row, field)
	})
}

func vpcSortKey(row components.Row, col string) string {
	idx := -1
	for i, c := range vpcColumns {
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

func (v *ListView) onSelect(_ int, id string) {
	if id == "" {
		return
	}
	v.navigator.Navigate(navigation.Route{
		Resource:   "vpc-detail",
		ResourceID: id,
	})
}
