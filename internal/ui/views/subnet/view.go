// Package subnetview provides the Subnet view.
package subnetview

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

// Navigator is the interface for subnet views.
type Navigator interface {
	Navigate(route navigation.Route)
	EC2Service() *ec2.Service
	TviewApp() *tview.Application
	Context() context.Context
	SetStatus(text string)
}

var subnetColumns = []components.Column{
	{Title: "SUBNET ID", Field: "subnet_id", Expansion: 1},
	{Title: "NAME", Field: "name", Expansion: 1},
	{Title: "VPC ID", Field: "vpc_id", Expansion: 1},
	{Title: "CIDR", Field: "cidr", Expansion: 1},
	{Title: "AZ", Field: "az", Expansion: 1},
	{Title: "AVAILABLE IPs", Field: "available_ips", Expansion: 1},
	{Title: "PUBLIC IP", Field: "public_ip", Expansion: 1},
}

// ListView displays subnets.
type ListView struct {
	st        *components.SortableTable
	navigator Navigator
	vpcID     string // optional filter by VPC

	mu      sync.RWMutex
	subnets []ec2.Subnet
}

// NewListView creates a new subnet list view.
func NewListView(navigator Navigator, vpcID string) *ListView {
	title := "Subnets"
	if vpcID != "" {
		title = fmt.Sprintf("Subnets (%s)", vpcID)
	}

	v := &ListView{
		navigator: navigator,
		vpcID:     vpcID,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    title,
		Columns:  subnetColumns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)
	v.st.SetSelectedFunc(v.onSelect)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "subnet"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.st.Table
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

	v.rebuildRows()
	return nil
}

// Shortcuts returns subnet-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "v", Label: "goto VPC"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
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

// rebuildRows converts subnets into table rows and applies sort.
func (v *ListView) rebuildRows() {
	v.mu.RLock()
	subnets := make([]ec2.Subnet, len(v.subnets))
	copy(subnets, v.subnets)
	v.mu.RUnlock()

	rows := make([]components.Row, len(subnets))
	for i, sn := range subnets {
		name := sn.Name
		if name == "" {
			name = "-"
		}
		publicIP := "No"
		publicColor := tcell.ColorLightGray
		if sn.MapPublicIP {
			publicIP = "Yes"
			publicColor = tcell.ColorGreen
		}

		rows[i] = components.Row{
			ID: sn.SubnetID,
			Cells: []string{
				sn.SubnetID,
				name,
				sn.VPCID,
				sn.CIDRBlock,
				sn.AZ,
				fmt.Sprintf("%d", sn.AvailableIPs),
				publicIP,
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorYellow,
				publicColor,
			},
		}
	}

	v.st.SetRows(rows)
	v.st.SetSortKeyFn(func(row components.Row, field string) string {
		return subnetSortKey(row, field)
	})
}

func subnetSortKey(row components.Row, col string) string {
	idx := -1
	for i, c := range subnetColumns {
		if c.Field == col {
			idx = i
			break
		}
	}
	if idx < 0 || idx >= len(row.Cells) {
		return ""
	}
	// Zero-pad numeric fields
	if col == "available_ips" {
		return fmt.Sprintf("%010s", row.Cells[idx])
	}
	return strings.ToLower(row.Cells[idx])
}

func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'v':
		idx := v.st.GetSelectedIndex()
		if idx < 0 {
			return event
		}
		v.mu.RLock()
		if idx >= len(v.subnets) {
			v.mu.RUnlock()
			return event
		}
		subnet := v.subnets[idx]
		v.mu.RUnlock()

		v.navigator.Navigate(navigation.Route{
			Resource:   "vpc-detail",
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

func (v *ListView) onSelect(_ int, id string) {
	if id == "" {
		return
	}
	v.navigator.Navigate(navigation.Route{
		Resource:   "subnet-detail",
		ResourceID: id,
	})
}
