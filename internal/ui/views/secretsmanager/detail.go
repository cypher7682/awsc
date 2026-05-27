package secretsmanagerview

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/secretsmanager"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// DetailView displays detailed information about a secret.
type DetailView struct {
	flex      *tview.Flex
	table     *tview.Table
	navigator Navigator
	secretID  string

	mu          sync.RWMutex
	secret      *secretsmanager.Secret
	secretValue *secretsmanager.SecretValue
	showValue   bool
}

// NewDetailView creates a new secret detail view.
func NewDetailView(navigator Navigator, secretID string) *DetailView {
	v := &DetailView{
		navigator: navigator,
		secretID:  secretID,
	}

	v.table = tview.NewTable()
	v.table.SetBorders(false)
	v.table.SetSelectable(true, false)
	v.table.SetTitle(" Secret: " + secretID + " ")
	v.table.SetBorder(true)
	v.table.SetBorderColor(tcell.ColorGray)

	v.flex = tview.NewFlex().SetDirection(tview.FlexRow)
	v.flex.AddItem(v.table, 0, 1, true)

	return v
}

// Name returns the view identifier.
func (v *DetailView) Name() string {
	return "asm-detail"
}

// Render returns the tview primitive.
func (v *DetailView) Render() tview.Primitive {
	return v.flex
}

// Refresh reloads secret data from AWS.
func (v *DetailView) Refresh(ctx context.Context) error {
	svc := v.navigator.SecretsManagerService()

	secret, err := svc.DescribeSecret(ctx, v.secretID)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.secret = secret
	v.secretValue = nil // Clear cached value on refresh
	v.showValue = false
	v.mu.Unlock()

	v.rebuildTable()
	return nil
}

// Shortcuts returns available keyboard shortcuts.
func (v *DetailView) Shortcuts() []components.Shortcut {
	v.mu.RLock()
	showValue := v.showValue
	v.mu.RUnlock()

	shortcuts := []components.Shortcut{
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}

	if showValue {
		shortcuts = append([]components.Shortcut{{Key: "r", Label: "hide value"}}, shortcuts...)
	} else {
		shortcuts = append([]components.Shortcut{{Key: "r", Label: "retrieve value"}}, shortcuts...)
	}

	return shortcuts
}

// FilterFields returns available filter fields.
func (v *DetailView) FilterFields() []string {
	return nil
}

// HandleFilter does nothing for detail view.
func (v *DetailView) HandleFilter(_ string) {}

// HandleInput processes view-specific keys.
func (v *DetailView) HandleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		v.navigator.NavigateBack()
		return nil
	}

	switch event.Rune() {
	case 'r':
		v.toggleSecretValue()
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

// toggleSecretValue retrieves or hides the secret value.
func (v *DetailView) toggleSecretValue() {
	v.mu.Lock()
	if v.showValue {
		// Hide value
		v.showValue = false
		v.mu.Unlock()
		v.rebuildTable()
		v.navigator.RefreshShortcuts()
		v.navigator.SetStatus("Secret value hidden")
		return
	}
	v.mu.Unlock()

	// Retrieve value
	v.navigator.SetStatus("[yellow]Retrieving secret value...")

	go func() {
		svc := v.navigator.SecretsManagerService()
		value, err := svc.GetSecretValue(v.navigator.Context(), v.secretID, nil, nil)

		v.navigator.TviewApp().QueueUpdateDraw(func() {
			if err != nil {
				v.navigator.SetStatus(fmt.Sprintf("[red]Failed to retrieve: %s", err.Error()))
				return
			}

			v.mu.Lock()
			v.secretValue = value
			v.showValue = true
			v.mu.Unlock()

			v.rebuildTable()
			v.navigator.RefreshShortcuts()
			v.navigator.SetStatus("[green]Secret value retrieved")
		})
	}()
}

// rebuildTable rebuilds the detail table.
func (v *DetailView) rebuildTable() {
	v.mu.RLock()
	s := v.secret
	sv := v.secretValue
	showValue := v.showValue
	v.mu.RUnlock()

	if s == nil {
		return
	}

	v.table.Clear()

	row := 0
	addRow := func(label, value string, valueColor tcell.Color) {
		v.table.SetCell(row, 0,
			tview.NewTableCell(label).
				SetTextColor(tcell.ColorYellow).
				SetSelectable(false).
				SetAlign(tview.AlignRight))
		v.table.SetCell(row, 1,
			tview.NewTableCell("  "+value).
				SetTextColor(valueColor).
				SetExpansion(1))
		row++
	}

	addSection := func(title string) {
		if row > 0 {
			row++ // Add blank row before section
		}
		v.table.SetCell(row, 0,
			tview.NewTableCell("── "+title+" ").
				SetTextColor(tcell.ColorDodgerBlue).
				SetSelectable(false).
				SetAttributes(tcell.AttrBold))
		v.table.SetCell(row, 1,
			tview.NewTableCell(strings.Repeat("─", 60)).
				SetTextColor(tcell.ColorDodgerBlue).
				SetSelectable(false))
		row++
	}

	// Basic Info
	addSection("Basic Information")
	addRow("Name", s.Name, tcell.ColorWhite)
	addRow("ARN", s.ARN, tcell.ColorLightGray)
	if s.Description != "" {
		addRow("Description", s.Description, tcell.ColorLightGray)
	}
	if s.OwningService != "" {
		addRow("Owning Service", s.OwningService, tcell.ColorLightGray)
	}

	// Dates
	addSection("Dates")
	addRow("Created", formatDateTime(s.CreatedDate), tcell.ColorLightGray)
	addRow("Last Changed", formatDateTime(s.LastChangedDate), tcell.ColorLightGray)
	addRow("Last Accessed", formatDateTime(s.LastAccessedDate), tcell.ColorLightGray)
	if s.DeletedDate != nil {
		addRow("Scheduled Deletion", formatDateTime(s.DeletedDate), tcell.ColorRed)
	}

	// Encryption
	addSection("Encryption")
	kmsKey := s.KmsKeyID
	if kmsKey == "" {
		kmsKey = "aws/secretsmanager (default)"
	}
	addRow("KMS Key", kmsKey, tcell.ColorLightGray)

	// Rotation
	addSection("Rotation")
	if s.RotationEnabled {
		addRow("Status", "Enabled", tcell.ColorGreen)
		if s.RotationSchedule != "" {
			addRow("Schedule", s.RotationSchedule, tcell.ColorLightGray)
		}
		if s.RotationLambdaARN != "" {
			addRow("Lambda ARN", s.RotationLambdaARN, tcell.ColorLightGray)
		}
	} else {
		addRow("Status", "Disabled", tcell.ColorGray)
	}

	// Replication
	if s.PrimaryRegion != "" || len(s.Replicas) > 0 {
		addSection("Replication")
		if s.PrimaryRegion != "" {
			addRow("Primary Region", s.PrimaryRegion, tcell.ColorLightGray)
		}
		for _, r := range s.Replicas {
			statusColor := tcell.ColorGreen
			if r.Status != "InSync" {
				statusColor = tcell.ColorYellow
			}
			addRow("Replica: "+r.Region, r.Status, statusColor)
		}
	}

	// Versions
	if len(s.Versions) > 0 {
		addSection("Versions")
		for _, ver := range s.Versions {
			labels := strings.Join(ver.StagingLabels, ", ")
			versionColor := tcell.ColorLightGray
			if containsLabel(ver.StagingLabels, "AWSCURRENT") {
				versionColor = tcell.ColorGreen
			} else if containsLabel(ver.StagingLabels, "AWSPENDING") {
				versionColor = tcell.ColorYellow
			}
			addRow(ver.VersionID, labels, versionColor)
		}
	}

	// Tags
	if len(s.Tags) > 0 {
		addSection("Tags")
		for k, val := range s.Tags {
			addRow(k, val, tcell.ColorLightGray)
		}
	}

	// Secret Value
	addSection("Secret Value")
	if showValue && sv != nil {
		if sv.SecretString != "" {
			// Try to pretty-format JSON
			value := sv.SecretString
			addRow("Value", value, tcell.ColorWhite)
		} else if len(sv.SecretBinary) > 0 {
			addRow("Value", fmt.Sprintf("[binary data, %d bytes]", len(sv.SecretBinary)), tcell.ColorYellow)
		}
		if sv.VersionID != "" {
			addRow("Version ID", sv.VersionID, tcell.ColorLightGray)
		}
		if len(sv.VersionStages) > 0 {
			addRow("Version Stages", strings.Join(sv.VersionStages, ", "), tcell.ColorLightGray)
		}
	} else {
		addRow("Value", "[Press 'r' to retrieve]", tcell.ColorDarkGray)
	}
}

// containsLabel checks if a slice contains a specific label.
func containsLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// formatDate formats a time as date only.
func formatDate(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02")
}

// formatDateTime formats a time with date and time.
func formatDateTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
