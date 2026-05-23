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
	ShowConfirm(prompt string, onConfirm func())
	SetStatus(text string)
}

// --- ECR Repository List View ---

var ecrColumns = []components.Column{
	{Title: "NAME", Field: "name", Expansion: 2},
	{Title: "URI", Field: "uri", Expansion: 2},
	{Title: "MUTABILITY", Field: "mutability", Expansion: 1},
	{Title: "SCAN ON PUSH", Field: "scan_on_push", Expansion: 1},
	{Title: "CREATED", Field: "created", Expansion: 1},
}

// ListView displays a list of ECR repositories.
type ListView struct {
	st        *components.SortableTable
	navigator Navigator

	mu     sync.RWMutex
	repos  []ecr.Repository
	filter string
}

// NewListView creates a new ECR list view.
func NewListView(navigator Navigator) *ListView {
	v := &ListView{
		navigator: navigator,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "ECR Repositories",
		Columns:  ecrColumns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)
	v.st.SetSelectedFunc(v.onSelect)

	return v
}

// Name returns the view identifier.
func (v *ListView) Name() string {
	return "ecr"
}

// Render returns the tview primitive.
func (v *ListView) Render() tview.Primitive {
	return v.st.Table
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

	v.rebuildRows()
	return nil
}

// Shortcuts returns ECR-specific shortcuts.
func (v *ListView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "images"},
		{Key: "c", Label: "create"},
		{Key: "Del", Label: "delete"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
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
	v.rebuildRows()
}

// rebuildRows converts repos into table rows, applies filter and sort.
func (v *ListView) rebuildRows() {
	v.mu.RLock()
	filter := v.filter
	repos := make([]ecr.Repository, 0, len(v.repos))
	for _, repo := range v.repos {
		if filter != "" && !strings.Contains(strings.ToLower(repo.Name), strings.ToLower(filter)) {
			continue
		}
		repos = append(repos, repo)
	}
	v.mu.RUnlock()

	rows := make([]components.Row, len(repos))
	for i, repo := range repos {
		scanIcon := "[red]No"
		if repo.ScanOnPush {
			scanIcon = "[green]Yes"
		}
		mutText := repo.MutabilityTag
		mutColor := tcell.ColorYellow
		if repo.MutabilityTag == "IMMUTABLE" {
			mutColor = tcell.ColorGreen
		}

		rows[i] = components.Row{
			ID: repo.Name,
			Cells: []string{
				repo.Name,
				repo.URI,
				mutText,
				scanIcon,
				repo.CreatedAt.Format("2006-01-02"),
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorLightGray,
				mutColor,
				tcell.ColorWhite, // tview dynamic colors handle this
				tcell.ColorLightGray,
			},
		}
	}

	col := v.st.SortColumn()
	v.st.SetRows(rows)
	v.st.SortRows(func(row components.Row) string {
		return ecrSortKey(row, col)
	})
}

func ecrSortKey(row components.Row, col string) string {
	idx := -1
	for i, c := range ecrColumns {
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

// handleInput processes view-specific keys.
func (v *ListView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyDelete {
		idx := v.st.GetSelectedIndex()
		if idx < 0 {
			return event
		}
		v.mu.RLock()
		if idx >= len(v.repos) {
			v.mu.RUnlock()
			return event
		}
		// Find the actual repo — we need to account for filtering
		repoName := v.st.GetRowID()
		v.mu.RUnlock()
		if repoName == "" {
			return event
		}
		v.navigator.ShowConfirm(fmt.Sprintf("Delete repository %s?", repoName), func() {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]Deleting repository %s...", repoName))
			go func() {
				err := v.navigator.ECRService().DeleteRepository(v.navigator.Context(), repoName, false)
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					if err != nil {
						v.navigator.SetStatus(fmt.Sprintf("[red]Failed to delete: %s", err.Error()))
					} else {
						v.navigator.SetStatus(fmt.Sprintf("[green]Deleted %s", repoName))
						v.Refresh(v.navigator.Context())
					}
				})
			}()
		})
		return nil
	}

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
		Resource:   "ecr-detail",
		ResourceID: id,
	})
}

// --- ECR Image View ---

var ecrImageColumns = []components.Column{
	{Title: "TAGS", Field: "tags", Expansion: 2},
	{Title: "DIGEST", Field: "digest", Expansion: 1},
	{Title: "SIZE (MB)", Field: "size", Expansion: 1},
	{Title: "PUSHED", Field: "pushed", Expansion: 1},
	{Title: "SCAN STATUS", Field: "scan_status", Expansion: 1},
}

// ImageView displays images within an ECR repository.
type ImageView struct {
	st        *components.SortableTable
	navigator Navigator
	repoName  string

	mu     sync.RWMutex
	images []ecr.Image
}

// NewImageView creates a new ECR image list view.
func NewImageView(navigator Navigator, repoName string) *ImageView {
	v := &ImageView{
		navigator: navigator,
		repoName:  repoName,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    fmt.Sprintf("ECR Images: %s", repoName),
		Columns:  ecrImageColumns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)

	return v
}

// Name returns the view identifier.
func (v *ImageView) Name() string {
	return "ecr-detail"
}

// Render returns the tview primitive.
func (v *ImageView) Render() tview.Primitive {
	return v.st.Table
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

	v.rebuildRows()
	return nil
}

// Shortcuts returns image view shortcuts.
func (v *ImageView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Del", Label: "delete"},
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
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

// rebuildRows converts images into table rows and applies sort.
func (v *ImageView) rebuildRows() {
	v.mu.RLock()
	images := make([]ecr.Image, len(v.images))
	copy(images, v.images)
	v.mu.RUnlock()

	rows := make([]components.Row, len(images))
	for i, img := range images {
		tags := strings.Join(img.Tags, ", ")
		if tags == "" {
			tags = "[gray]<untagged>"
		}
		sizeMB := fmt.Sprintf("%.1f", float64(img.SizeBytes)/1024/1024)
		digest := img.Digest
		if len(digest) > 19 {
			digest = digest[:19] + "..."
		}
		scanStatus := orDash(img.ScanStatus)
		scanColor := tcell.ColorGray
		if img.ScanStatus == "COMPLETE" {
			scanColor = tcell.ColorGreen
		} else if img.ScanStatus == "FAILED" {
			scanColor = tcell.ColorRed
		}

		rows[i] = components.Row{
			ID: img.Digest, // full digest as ID
			Cells: []string{
				tags,
				digest,
				sizeMB,
				img.PushedAt.Format("2006-01-02 15:04"),
				scanStatus,
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				scanColor,
			},
		}
	}

	col := v.st.SortColumn()
	v.st.SetRows(rows)
	v.st.SortRows(func(row components.Row) string {
		return ecrImageSortKey(row, col)
	})
}

func ecrImageSortKey(row components.Row, col string) string {
	idx := -1
	for i, c := range ecrImageColumns {
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

// handleInput processes view-specific keys for image view.
func (v *ImageView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyDelete {
		idx := v.st.GetSelectedIndex()
		if idx < 0 {
			return event
		}
		v.mu.RLock()
		if idx >= len(v.images) {
			v.mu.RUnlock()
			return event
		}
		img := v.images[idx]
		v.mu.RUnlock()

		digestShort := img.Digest
		if len(digestShort) > 19 {
			digestShort = digestShort[:19]
		}
		tagLabel := digestShort
		if len(img.Tags) > 0 {
			tagLabel = img.Tags[0]
		}
		digest := img.Digest
		repoName := v.repoName
		v.navigator.ShowConfirm(fmt.Sprintf("Delete image %s?", tagLabel), func() {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]Deleting image %s...", digestShort))
			go func() {
				err := v.navigator.ECRService().DeleteImage(v.navigator.Context(), repoName, digest)
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					if err != nil {
						v.navigator.SetStatus(fmt.Sprintf("[red]Failed to delete: %s", err.Error()))
					} else {
						v.navigator.SetStatus("[green]Image deleted")
						v.Refresh(v.navigator.Context())
					}
				})
			}()
		})
		return nil
	}

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

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
