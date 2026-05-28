// Package ec2view provides EC2 views including launch templates.
package ec2view

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

// launchTemplateColumns defines the column layout for the launch templates table.
var launchTemplateColumns = []components.Column{
	{Title: "LAUNCH TEMPLATE ID", Field: "id", Expansion: 1},
	{Title: "NAME", Field: "name", Expansion: 2},
	{Title: "DEFAULT VER", Field: "default_ver", Expansion: 0},
	{Title: "LATEST VER", Field: "latest_ver", Expansion: 0},
	{Title: "CREATED", Field: "created", Expansion: 1},
	{Title: "CREATED BY", Field: "created_by", Expansion: 1},
}

// LaunchTemplatesView displays the list of EC2 launch templates.
type LaunchTemplatesView struct {
	st        *components.SortableTable
	navigator Navigator

	mu        sync.RWMutex
	templates []ec2.LaunchTemplate
	filtered  []ec2.LaunchTemplate
	filter    string
}

// NewLaunchTemplatesView creates a new launch templates list view.
func NewLaunchTemplatesView(navigator Navigator) *LaunchTemplatesView {
	v := &LaunchTemplatesView{
		navigator: navigator,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "EC2 Launch Templates",
		Columns:  launchTemplateColumns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)
	v.st.SetSelectedFunc(v.onSelect)

	return v
}

// Name returns the view identifier.
func (v *LaunchTemplatesView) Name() string {
	return "ec2/launch-templates"
}

// Render returns the tview primitive.
func (v *LaunchTemplatesView) Render() tview.Primitive {
	return v.st.Table
}

// Refresh reloads launch template data from AWS.
func (v *LaunchTemplatesView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()
	templates, err := svc.ListLaunchTemplates(ctx)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.templates = templates
	v.applyFilter()
	v.mu.Unlock()

	v.rebuildRows()
	return nil
}

// Shortcuts returns launch templates view shortcuts.
func (v *LaunchTemplatesView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "details"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *LaunchTemplatesView) FilterFields() []string {
	return []string{"id", "name", "created_by"}
}

// HandleFilter applies a filter expression.
func (v *LaunchTemplatesView) HandleFilter(expression string) {
	v.mu.Lock()
	v.filter = expression
	v.applyFilter()
	v.mu.Unlock()
	v.rebuildRows()
}

// applyFilter filters templates based on the current filter expression.
// Must be called with mu held.
func (v *LaunchTemplatesView) applyFilter() {
	if v.filter == "" {
		v.filtered = v.templates
		return
	}

	v.filtered = nil
	lower := strings.ToLower(v.filter)

	// Parse field:value filters
	var field, value string
	if idx := strings.Index(lower, ":"); idx > 0 {
		field = lower[:idx]
		value = lower[idx+1:]
	} else {
		value = lower
	}

	for _, lt := range v.templates {
		match := false

		switch field {
		case "id":
			match = strings.Contains(strings.ToLower(lt.LaunchTemplateID), value)
		case "name":
			match = strings.Contains(strings.ToLower(lt.LaunchTemplateName), value)
		case "created_by":
			match = strings.Contains(strings.ToLower(lt.CreatedBy), value)
		default:
			// No field specified - search across all text fields
			searchable := strings.ToLower(fmt.Sprintf("%s %s %s",
				lt.LaunchTemplateID,
				lt.LaunchTemplateName,
				lt.CreatedBy,
			))
			match = strings.Contains(searchable, value)
		}

		if match {
			v.filtered = append(v.filtered, lt)
		}
	}
}

// rebuildRows updates the table rows from filtered data.
func (v *LaunchTemplatesView) rebuildRows() {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rows := make([]components.Row, 0, len(v.filtered))
	for _, lt := range v.filtered {
		// Format create time
		created := lt.CreateTime.Format("2006-01-02 15:04")

		// Simplify created by (often an ARN)
		createdBy := lt.CreatedBy
		if strings.Contains(createdBy, "/") {
			parts := strings.Split(createdBy, "/")
			createdBy = parts[len(parts)-1]
		}

		// Cells in column order: id, name, default_ver, latest_ver, created, created_by
		rows = append(rows, components.Row{
			ID: lt.LaunchTemplateID,
			Cells: []string{
				lt.LaunchTemplateID,
				lt.LaunchTemplateName,
				fmt.Sprintf("%d", lt.DefaultVersion),
				fmt.Sprintf("%d", lt.LatestVersion),
				created,
				createdBy,
			},
		})
	}

	v.st.SetRows(rows)
}

// onSelect handles selection of a launch template.
func (v *LaunchTemplatesView) onSelect(row int, id string) {
	v.navigator.Navigate(navigation.Route{
		Resource:   "ec2/launch-template-detail",
		ResourceID: id,
	})
}

// handleInput processes view-specific key events.
func (v *LaunchTemplatesView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		row, _ := v.st.Table.GetSelection()
		if row > 0 && row-1 < len(v.filtered) {
			v.mu.RLock()
			lt := v.filtered[row-1]
			v.mu.RUnlock()
			v.onSelect(row-1, lt.LaunchTemplateID)
		}
		return nil
	}

	switch event.Rune() {
	case 'R':
		// Refresh
		v.navigator.SetStatus("[yellow]Refreshing launch templates...")
		go func() {
			ctx := v.navigator.Context()
			if err := v.Refresh(ctx); err != nil {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus(fmt.Sprintf("[red]Refresh failed: %s", err.Error()))
				})
			} else {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus("[green]Launch templates refreshed")
				})
			}
		}()
		return nil
	}
	return event
}
