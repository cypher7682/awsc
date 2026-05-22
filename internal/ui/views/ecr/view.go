// Package ecrview provides the ECR repository and image views.
package ecrview

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ecr"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// Navigator is the interface for ECR views to navigate.
type Navigator interface {
	Navigate(route navigation.Route)
	ECRService() *ecr.Service
	TviewApp() *tview.Application
	Context() context.Context
	ShowConfirm(prompt string)
	SetStatus(text string)
}

// ListView displays a list of ECR repositories.
type ListView struct {
	table     *tview.Table
	navigator Navigator

	mu     sync.RWMutex
	repos  []ecr.Repository
	filter string
}

// NewListView creates a new ECR list view.
func NewListView(navigator Navigator) *ListView {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetSelectable(true, false)
	table.SetTitle(" ECR Repositories ")
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
	return "ecr"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.table
}

// Refresh reloads repository data from AWS.
func (v *ListView) Refresh(ctx context.Context) error {
	svc := v.navigator.ECRService()
	repos, err := svc.ListRepositories(ctx)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.repos = repos
	v.mu.Unlock()

	v.renderTable()
	return nil
}

// Shortcuts returns ECR-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "images"},
		{Key: "c", Label: "create"},
		{Key: "d", Label: "delete"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields for ECR.
func (v *ListView) FilterFields() []string {
	return []string{"name", "uri", "mutability", "scan_on_push"}
}

// HandleFilter applies a filter expression.
func (v *ListView) HandleFilter(expression string) {
	v.mu.Lock()
	v.filter = expression
	v.mu.Unlock()
	v.renderTable()
}

// renderTable rebuilds the table display.
func (v *ListView) renderTable() {
	v.mu.RLock()
	repos := v.repos
	filter := v.filter
	v.mu.RUnlock()

	v.table.Clear()

	// Header row
	headers := []string{"NAME", "URI", "MUTABILITY", "SCAN ON PUSH", "CREATED"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		if col == 0 || col == 1 {
			cell.SetExpansion(2)
		}
		v.table.SetCell(0, col, cell)
	}

	// Data rows
	row := 1
	for _, repo := range repos {
		// Apply filter
		if filter != "" && !strings.Contains(strings.ToLower(repo.Name), strings.ToLower(filter)) {
			continue
		}

		scanIcon := "[red]No"
		if repo.ScanOnPush {
			scanIcon = "[green]Yes"
		}

		mutColor := "yellow"
		if repo.MutabilityTag == "IMMUTABLE" {
			mutColor = "green"
		}

		v.table.SetCell(row, 0, tview.NewTableCell(repo.Name).SetTextColor(tcell.ColorWhite).SetExpansion(2))
		v.table.SetCell(row, 1, tview.NewTableCell(repo.URI).SetTextColor(tcell.ColorLightGray).SetExpansion(2))
		v.table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("[%s]%s", mutColor, repo.MutabilityTag)).SetExpansion(1))
		v.table.SetCell(row, 3, tview.NewTableCell(scanIcon).SetExpansion(1))
		v.table.SetCell(row, 4, tview.NewTableCell(repo.CreatedAt.Format("2006-01-02")).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		row++
	}

	v.table.SetTitle(fmt.Sprintf(" ECR Repositories (%d) ", row-1))
}

// handleInput processes key events for the ECR list.
func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'd':
		row, _ := v.table.GetSelection()
		if row <= 0 {
			return event
		}
		repo := v.getRepoAtRow(row)
		if repo == nil {
			return event
		}
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Deleting repository %s...", repo.Name))
		go func() {
			err := v.navigator.ECRService().DeleteRepository(v.navigator.Context(), repo.Name, false)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed to delete: %s", err.Error()))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]Deleted %s", repo.Name))
					v.Refresh(v.navigator.Context())
				}
			})
		}()
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

// onSelect handles Enter key on a repository row.
func (v *ListView) onSelect(row, _ int) {
	if row <= 0 {
		return
	}
	repo := v.getRepoAtRow(row)
	if repo == nil {
		return
	}
	v.navigator.Navigate(navigation.Route{
		Resource:   "ecr-detail",
		ResourceID: repo.Name,
	})
}

// getRepoAtRow returns the repository at the given table row.
func (v *ListView) getRepoAtRow(row int) *ecr.Repository {
	v.mu.RLock()
	defer v.mu.RUnlock()

	idx := row - 1
	if idx < 0 || idx >= len(v.repos) {
		return nil
	}
	return &v.repos[idx]
}

// ImageView displays images within an ECR repository.
type ImageView struct {
	table     *tview.Table
	navigator Navigator
	repoName  string

	mu     sync.RWMutex
	images []ecr.Image
}

// NewImageView creates a new ECR image list view.
func NewImageView(navigator Navigator, repoName string) *ImageView {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetSelectable(true, false)
	table.SetTitle(fmt.Sprintf(" ECR Images: %s ", repoName))
	table.SetBorder(true)
	table.SetBorderColor(tcell.ColorDodgerBlue)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	v := &ImageView{
		table:     table,
		navigator: navigator,
		repoName:  repoName,
	}

	table.SetInputCapture(v.handleInput)
	return v
}

// Name returns the view identifier.
func (v *ImageView) Name() string {
	return "ecr-detail"
}

// Render returns the tview primitive.
func (v *ImageView) Render() tview.Primitive {
	return v.table
}

// Refresh reloads image data from AWS.
func (v *ImageView) Refresh(ctx context.Context) error {
	svc := v.navigator.ECRService()
	images, err := svc.ListImages(ctx, v.repoName)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.images = images
	v.mu.Unlock()

	v.renderTable()
	return nil
}

// Shortcuts returns image view shortcuts.
func (v *ImageView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "d", Label: "delete"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *ImageView) FilterFields() []string {
	return []string{"tag", "digest", "scan_status"}
}

// HandleFilter applies a filter.
func (v *ImageView) HandleFilter(_ string) {}

// renderTable rebuilds the image table.
func (v *ImageView) renderTable() {
	v.mu.RLock()
	images := v.images
	v.mu.RUnlock()

	v.table.Clear()

	headers := []string{"TAGS", "DIGEST", "SIZE (MB)", "PUSHED", "SCAN STATUS"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		if col == 0 {
			cell.SetExpansion(2)
		}
		v.table.SetCell(0, col, cell)
	}

	for row, img := range images {
		tags := strings.Join(img.Tags, ", ")
		if tags == "" {
			tags = "[gray]<untagged>"
		}
		sizeMB := fmt.Sprintf("%.1f", float64(img.SizeBytes)/1024/1024)

		digest := img.Digest
		if len(digest) > 19 {
			digest = digest[:19] + "..."
		}

		scanColor := "gray"
		scanStatus := orDash(img.ScanStatus)
		if img.ScanStatus == "COMPLETE" {
			scanColor = "green"
		} else if img.ScanStatus == "FAILED" {
			scanColor = "red"
		}

		v.table.SetCell(row+1, 0, tview.NewTableCell(tags).SetTextColor(tcell.ColorWhite).SetExpansion(2))
		v.table.SetCell(row+1, 1, tview.NewTableCell(digest).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row+1, 2, tview.NewTableCell(sizeMB).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row+1, 3, tview.NewTableCell(img.PushedAt.Format("2006-01-02 15:04")).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.table.SetCell(row+1, 4, tview.NewTableCell(fmt.Sprintf("[%s]%s", scanColor, scanStatus)).SetExpansion(1))
	}

	v.table.SetTitle(fmt.Sprintf(" ECR Images: %s (%d) ", v.repoName, len(images)))
}

// handleInput processes key events for the image view.
func (v *ImageView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'd':
		row, _ := v.table.GetSelection()
		if row <= 0 {
			return event
		}
		v.mu.RLock()
		idx := row - 1
		if idx >= len(v.images) {
			v.mu.RUnlock()
			return event
		}
		img := v.images[idx]
		v.mu.RUnlock()

		v.navigator.SetStatus(fmt.Sprintf("[yellow]Deleting image %s...", img.Digest[:19]))
		go func() {
			err := v.navigator.ECRService().DeleteImage(v.navigator.Context(), v.repoName, img.Digest)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed to delete: %s", err.Error()))
				} else {
					v.navigator.SetStatus("[green]Image deleted")
					v.Refresh(v.navigator.Context())
				}
			})
		}()
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

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
