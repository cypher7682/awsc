// Package ec2view provides EC2 views including instance types.
package ec2view

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// instanceTypeColumns defines the column layout for the instance types table.
var instanceTypeColumns = []components.Column{
	{Title: "INSTANCE TYPE", Field: "name", Expansion: 1},
	{Title: "FREE TIER", Field: "free_tier", Expansion: 0},
	{Title: "VCPUS", Field: "vcpus", Expansion: 0},
	{Title: "ARCH", Field: "arch", Expansion: 0},
	{Title: "MEMORY (GIB)", Field: "memory", Expansion: 0},
	{Title: "STORAGE (GB)", Field: "storage", Expansion: 0},
	{Title: "STORAGE TYPE", Field: "storage_type", Expansion: 0},
	{Title: "NETWORK", Field: "network", Expansion: 1},
	{Title: "GENERATION", Field: "generation", Expansion: 0},
	{Title: "HYPERVISOR", Field: "hypervisor", Expansion: 0},
}

// InstanceTypesView displays the list of available EC2 instance types.
type InstanceTypesView struct {
	st        *components.SortableTable
	navigator Navigator

	mu            sync.RWMutex
	instanceTypes []ec2.InstanceTypeInfo
	filtered      []ec2.InstanceTypeInfo
	filter        string
}

// NewInstanceTypesView creates a new instance types list view.
func NewInstanceTypesView(navigator Navigator) *InstanceTypesView {
	v := &InstanceTypesView{
		navigator: navigator,
	}

	v.st = components.NewSortableTable(components.SortableTableConfig{
		Title:    "EC2 Instance Types",
		Columns:  instanceTypeColumns,
		OnStatus: navigator.SetStatus,
	})
	v.st.SetExtraInput(v.handleInput)

	return v
}

// Name returns the view identifier.
func (v *InstanceTypesView) Name() string {
	return "ec2/instance-types"
}

// Render returns the tview primitive.
func (v *InstanceTypesView) Render() tview.Primitive {
	return v.st.Table
}

// Refresh reloads instance type data from AWS.
func (v *InstanceTypesView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()
	types, err := svc.ListInstanceTypes(ctx)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.instanceTypes = types
	v.applyFilter()
	v.mu.Unlock()

	v.rebuildRows()
	return nil
}

// Shortcuts returns instance types view shortcuts.
func (v *InstanceTypesView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "s", Label: "sort-by"},
		{Key: "d", Label: "sort-dir"},
		{Key: "/", Label: "filter"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields.
func (v *InstanceTypesView) FilterFields() []string {
	return []string{
		"name", "arch", "vcpus", "memory", "storage_type",
		"network", "generation", "hypervisor", "free_tier",
	}
}

// HandleFilter applies a filter expression.
func (v *InstanceTypesView) HandleFilter(expression string) {
	v.mu.Lock()
	v.filter = expression
	v.applyFilter()
	v.mu.Unlock()
	v.rebuildRows()
}

// applyFilter filters instance types based on the current filter expression.
// Must be called with mu held.
func (v *InstanceTypesView) applyFilter() {
	if v.filter == "" {
		v.filtered = v.instanceTypes
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

	for _, it := range v.instanceTypes {
		match := false

		switch field {
		case "name":
			match = strings.Contains(strings.ToLower(it.Name), value)
		case "arch":
			for _, a := range it.Architectures {
				if strings.Contains(strings.ToLower(a), value) {
					match = true
					break
				}
			}
		case "vcpus":
			match = strings.Contains(fmt.Sprintf("%d", it.VCPUs), value)
		case "memory":
			match = strings.Contains(fmt.Sprintf("%d", it.MemoryMiB/1024), value)
		case "storage_type":
			match = strings.Contains(strings.ToLower(it.StorageType), value)
		case "network":
			match = strings.Contains(strings.ToLower(it.NetworkPerformance), value)
		case "generation":
			if value == "current" && it.CurrentGeneration {
				match = true
			} else if value == "previous" && !it.CurrentGeneration {
				match = true
			}
		case "hypervisor":
			match = strings.Contains(strings.ToLower(it.Hypervisor), value)
		case "free_tier":
			if value == "true" || value == "yes" {
				match = it.FreeTierEligible
			} else if value == "false" || value == "no" {
				match = !it.FreeTierEligible
			}
		default:
			// No field specified - search across all text fields
			searchable := strings.ToLower(fmt.Sprintf("%s %s %s %s",
				it.Name,
				strings.Join(it.Architectures, " "),
				it.NetworkPerformance,
				it.StorageType,
			))
			match = strings.Contains(searchable, value)
		}

		if match {
			v.filtered = append(v.filtered, it)
		}
	}
}

// rebuildRows updates the table rows from filtered data.
func (v *InstanceTypesView) rebuildRows() {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rows := make([]components.Row, 0, len(v.filtered))
	for _, it := range v.filtered {
		// Format architectures
		arch := strings.Join(it.Architectures, ",")

		// Format memory in GiB
		memoryGiB := fmt.Sprintf("%.1f", float64(it.MemoryMiB)/1024)

		// Format storage
		storage := "-"
		if it.StorageGB > 0 {
			storage = fmt.Sprintf("%d", it.StorageGB)
		}

		// Format storage type
		storageType := "-"
		if it.StorageType != "" {
			storageType = strings.ToLower(it.StorageType)
		}

		// Format free tier
		freeTier := "no"
		if it.FreeTierEligible {
			freeTier = "yes"
		}

		// Format generation
		generation := "previous"
		if it.CurrentGeneration {
			generation = "current"
		}

		// Format hypervisor
		hypervisor := "-"
		if it.Hypervisor != "" {
			hypervisor = strings.ToLower(it.Hypervisor)
		}

		// Cells in column order: name, free_tier, vcpus, arch, memory, storage, storage_type, network, generation, hypervisor
		rows = append(rows, components.Row{
			ID: it.Name,
			Cells: []string{
				it.Name,
				freeTier,
				fmt.Sprintf("%d", it.VCPUs),
				arch,
				memoryGiB,
				storage,
				storageType,
				it.NetworkPerformance,
				generation,
				hypervisor,
			},
		})
	}

	v.st.SetRows(rows)
}

// handleInput processes view-specific key events.
func (v *InstanceTypesView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'R':
		// Refresh
		v.navigator.SetStatus("[yellow]Refreshing instance types...")
		go func() {
			ctx := v.navigator.Context()
			if err := v.Refresh(ctx); err != nil {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus(fmt.Sprintf("[red]Refresh failed: %s", err.Error()))
				})
			} else {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus("[green]Instance types refreshed")
				})
			}
		}()
		return nil
	}
	return event
}
