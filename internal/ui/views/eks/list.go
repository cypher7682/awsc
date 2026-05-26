// Package eksview provides the EKS cluster and node group views.
package eksview

import (
	"context"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	ec2svc "github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/aws/eks"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// Navigator is the interface for EKS views to navigate.
type Navigator interface {
	Navigate(route navigation.Route)
	NavigateBack()
	EKSService() *eks.Service
	EC2Service() *ec2svc.Service
	TviewApp() *tview.Application
	Context() context.Context
	ShowConfirm(prompt string, onConfirm func())
	SetStatus(text string)
	RefreshShortcuts()
}

// --- EKS Cluster List View ---

var eksColumns = []components.Column{
	{Title: "NAME", Field: "name", Expansion: 2},
	{Title: "STATUS", Field: "status", Expansion: 1},
	{Title: "VERSION", Field: "version", Expansion: 1},
	{Title: "PLATFORM", Field: "platform", Expansion: 1},
	{Title: "VPC", Field: "vpc", Expansion: 1},
	{Title: "CREATED", Field: "created", Expansion: 1},
}

// ListView displays a list of EKS clusters.
type ListView struct {
	st        *components.SortableTable
	navigator Navigator

	mu       sync.RWMutex
	clusters []eks.Cluster
	filter   string
}

// NewListView creates a new EKS list view.
func NewListView(navigator Navigator) *ListView {
	v := &ListView{
		navigator: navigator,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "EKS Clusters",
		Columns:  eksColumns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)
	v.st.SetSelectedFunc(v.onSelect)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "eks"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.st.Table
}

// Refresh reloads cluster data from AWS.
func (v *ListView) Refresh(ctx context.Context) error {
	svc := v.navigator.EKSService()
	clusters, err := svc.ListAllClusters(ctx)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.clusters = clusters
	v.mu.Unlock()

	v.rebuildRows()
	return nil
}

// Shortcuts returns EKS-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "detail"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields for EKS.
func (v *ListView) FilterFields() []string {
	return []string{"name", "status", "version", "vpc"}
}

// HandleFilter applies a filter expression.
func (v *ListView) HandleFilter(expression string) {
	v.mu.Lock()
	v.filter = expression
	v.mu.Unlock()
	v.rebuildRows()
}

// rebuildRows converts clusters into table rows, applies filter and sort.
func (v *ListView) rebuildRows() {
	v.mu.RLock()
	filter := v.filter
	clusters := make([]eks.Cluster, 0, len(v.clusters))
	for _, c := range v.clusters {
		if filter != "" && !strings.Contains(strings.ToLower(c.Name), strings.ToLower(filter)) {
			continue
		}
		clusters = append(clusters, c)
	}
	v.mu.RUnlock()

	rows := make([]components.Row, len(clusters))
	for i, c := range clusters {
		statusColor := statusToColor(c.Status)
		vpcShort := c.VPCID
		if len(vpcShort) > 12 {
			vpcShort = vpcShort[:12] + "..."
		}

		rows[i] = components.Row{
			ID: c.Name,
			Cells: []string{
				c.Name,
				c.Status,
				c.Version,
				c.PlatformVersion,
				vpcShort,
				c.CreatedAt.Format("2006-01-02"),
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				statusColor,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorDodgerBlue,
				tcell.ColorLightGray,
			},
		}
	}

	v.st.SetRows(rows)
	v.st.SetSortKeyFn(func(row components.Row, field string) string {
		return eksSortKey(row, field)
	})
}

func eksSortKey(row components.Row, col string) string {
	idx := -1
	for i, c := range eksColumns {
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

func statusToColor(status string) tcell.Color {
	switch status {
	case "ACTIVE":
		return tcell.ColorGreen
	case "CREATING", "UPDATING":
		return tcell.ColorYellow
	case "DELETING":
		return tcell.ColorOrange
	case "FAILED":
		return tcell.ColorRed
	default:
		return tcell.ColorGray
	}
}

// handleInput processes view-specific keys.
func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
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

func (v *ListView) onSelect(_ int, id string) {
	if id == "" {
		return
	}
	v.navigator.Navigate(navigation.Route{
		Resource:   "eks-detail",
		ResourceID: id,
	})
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
