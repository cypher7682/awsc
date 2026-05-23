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

var sgColumns = []components.Column{
	{Title: "SG ID", Field: "sg_id", Expansion: 1},
	{Title: "NAME", Field: "name", Expansion: 1},
	{Title: "VPC ID", Field: "vpc_id", Expansion: 1},
	{Title: "INBOUND RULES", Field: "inbound", Expansion: 1},
	{Title: "OUTBOUND RULES", Field: "outbound", Expansion: 1},
	{Title: "DESCRIPTION", Field: "description", Expansion: 1},
}

// ListView displays security groups.
type ListView struct {
	st        *components.SortableTable
	navigator Navigator

	mu     sync.RWMutex
	groups []ec2.SecurityGroup
	filter string
}

// NewListView creates a new security groups list view.
func NewListView(navigator Navigator) *ListView {
	v := &ListView{
		navigator: navigator,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "Security Groups",
		Columns:  sgColumns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)
	v.st.SetSelectedFunc(v.onSelect)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "sg"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.st.Table
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

	v.rebuildRows()
	return nil
}

// Shortcuts returns SG-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "rules"},
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
	return []string{"sg_id", "name", "vpc_id", "description"}
}

// HandleFilter applies a filter.
func (v *ListView) HandleFilter(expression string) {
	v.mu.Lock()
	v.filter = expression
	v.mu.Unlock()
	v.rebuildRows()
}

// rebuildRows converts groups into table rows with filter and sort.
func (v *ListView) rebuildRows() {
	v.mu.RLock()
	filter := v.filter
	groups := make([]ec2.SecurityGroup, 0, len(v.groups))
	for _, sg := range v.groups {
		if filter != "" {
			lower := strings.ToLower(filter)
			if !strings.Contains(strings.ToLower(sg.GroupName), lower) &&
				!strings.Contains(strings.ToLower(sg.GroupID), lower) &&
				!strings.Contains(strings.ToLower(sg.Description), lower) {
				continue
			}
		}
		groups = append(groups, sg)
	}
	v.mu.RUnlock()

	rows := make([]components.Row, len(groups))
	for i, sg := range groups {
		desc := sg.Description
		if len(desc) > 40 {
			desc = desc[:40] + "..."
		}
		rows[i] = components.Row{
			ID: sg.GroupID,
			Cells: []string{
				sg.GroupID,
				sg.GroupName,
				sg.VPCID,
				fmt.Sprintf("%d", len(sg.IngressRules)),
				fmt.Sprintf("%d", len(sg.EgressRules)),
				desc,
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorYellow,
				tcell.ColorLightGray,
				tcell.ColorGray,
			},
		}
	}

	v.st.SetRows(rows)
	v.st.SetSortKeyFn(func(row components.Row, field string) string {
		return sgSortKey(row, field)
	})
}

func sgSortKey(row components.Row, col string) string {
	idx := -1
	for i, c := range sgColumns {
		if c.Field == col {
			idx = i
			break
		}
	}
	if idx < 0 || idx >= len(row.Cells) {
		return ""
	}
	// Zero-pad numeric fields for proper string sorting
	switch col {
	case "inbound", "outbound":
		return fmt.Sprintf("%05s", row.Cells[idx])
	}
	return strings.ToLower(row.Cells[idx])
}

// handleInput processes view-specific key events.
func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'v':
		id := v.st.GetRowID()
		if id == "" {
			return event
		}
		v.mu.RLock()
		var vpcID string
		for _, sg := range v.groups {
			if sg.GroupID == id {
				vpcID = sg.VPCID
				break
			}
		}
		v.mu.RUnlock()
		if vpcID != "" {
			v.navigator.Navigate(navigation.Route{
				Resource:   "vpc",
				ResourceID: vpcID,
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
func (v *ListView) onSelect(_ int, id string) {
	if id == "" {
		return
	}
	v.navigator.Navigate(navigation.Route{
		Resource:   "sg-detail",
		ResourceID: id,
	})
}
