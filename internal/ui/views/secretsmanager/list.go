// Package secretsmanagerview provides the Secrets Manager views.
package secretsmanagerview

import (
	"context"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/secretsmanager"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// Navigator is the interface for Secrets Manager views to navigate.
type Navigator interface {
	Navigate(route navigation.Route)
	NavigateBack()
	SecretsManagerService() *secretsmanager.Service
	TviewApp() *tview.Application
	Context() context.Context
	ShowConfirm(prompt string, onConfirm func())
	SetStatus(text string)
	RefreshShortcuts()
}

// ListView displays secrets in AWS Secrets Manager.
type ListView struct {
	table     *components.SortableTable
	navigator Navigator

	mu      sync.RWMutex
	secrets []secretsmanager.Secret
	filter  string
}

// NewListView creates a new Secrets Manager list view.
func NewListView(navigator Navigator) *ListView {
	v := &ListView{
		navigator: navigator,
	}

	v.table = components.NewSortableTable(components.SortableTableConfig{
		Title: "Secrets Manager",
		Columns: []components.Column{
			{Title: "NAME", Field: "name", Expansion: 3},
			{Title: "DESCRIPTION", Field: "description", Expansion: 3},
			{Title: "CREATED", Field: "created", Expansion: 1},
			{Title: "LAST CHANGED", Field: "changed", Expansion: 1},
			{Title: "LAST ACCESSED", Field: "accessed", Expansion: 1},
			{Title: "LAST RETRIEVED", Field: "retrieved", Expansion: 1},
			{Title: "ROTATION", Field: "rotation", Expansion: 1},
		},
		OnStatus: navigator.SetStatus,
	})

	v.table.SetSelectedFunc(func(row int, id string) {
		if id != "" {
			navigator.Navigate(navigation.Route{
				Resource:   "asm-detail",
				ResourceID: id,
			})
		}
	})

	v.table.SetExtraInput(v.handleInput)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "asm"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.table.Table
}

// Refresh reloads secrets from AWS.
func (v *ListView) Refresh(ctx context.Context) error {
	svc := v.navigator.SecretsManagerService()

	secrets, err := svc.ListSecretsWithClient(ctx)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.secrets = secrets
	v.mu.Unlock()

	v.rebuildTable()
	return nil
}

// Shortcuts returns available keyboard shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "details"},
		{Key: "c", Label: "create"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *ListView) FilterFields() []string {
	return []string{"name", "description"}
}

// HandleFilter applies a filter expression.
func (v *ListView) HandleFilter(expression string) {
	v.mu.Lock()
	v.filter = strings.ToLower(expression)
	v.mu.Unlock()
	v.rebuildTable()
}

// handleInput processes view-specific keys.
func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		v.navigator.NavigateBack()
		return nil
	}

	switch event.Rune() {
	case 'c':
		// Create placeholder - will be implemented later
		v.navigator.SetStatus("[yellow]Create secret: coming soon")
		return nil
	case 'R':
		go func() {
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				v.navigator.SetStatus("[yellow]Refreshing...")
			})
			err := v.Refresh(v.navigator.Context())
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus("[red]" + err.Error())
				} else {
					v.navigator.SetStatus("[green]Refreshed")
				}
			})
		}()
		return nil
	}

	return event
}

// rebuildTable rebuilds the table with current data and filter.
func (v *ListView) rebuildTable() {
	v.mu.RLock()
	secrets := v.secrets
	filter := v.filter
	v.mu.RUnlock()

	var rows []components.Row
	for _, s := range secrets {
		// Apply filter
		if filter != "" {
			name := strings.ToLower(s.Name)
			desc := strings.ToLower(s.Description)
			if !strings.Contains(name, filter) && !strings.Contains(desc, filter) {
				continue
			}
		}

		// Format dates
		created := formatDate(s.CreatedDate)
		changed := formatDate(s.LastChangedDate)
		accessed := formatDate(s.LastAccessedDate)
		retrieved := formatDate(s.LastRetrievedDate)

		// Rotation status
		rotation := "Disabled"
		rotationColor := tcell.ColorGray
		if s.RotationEnabled {
			rotation = "Enabled"
			rotationColor = tcell.ColorGreen
			if s.RotationSchedule != "" {
				rotation = s.RotationSchedule
			}
		}

		rows = append(rows, components.Row{
			ID: s.Name,
			Cells: []string{
				s.Name,
				truncate(s.Description, 50),
				created,
				changed,
				accessed,
				retrieved,
				rotation,
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				rotationColor,
			},
		})
	}

	v.table.SetRows(rows)
}
