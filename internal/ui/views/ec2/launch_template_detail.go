// Package ec2view provides EC2 views including launch template details.
package ec2view

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// LaunchTemplateDetailView displays details of a single launch template.
type LaunchTemplateDetailView struct {
	textView  *tview.TextView
	navigator Navigator
	templateID string
	template  *ec2.LaunchTemplateDetail
}

// NewLaunchTemplateDetailView creates a new launch template detail view.
func NewLaunchTemplateDetailView(navigator Navigator, templateID string) *LaunchTemplateDetailView {
	v := &LaunchTemplateDetailView{
		textView:   tview.NewTextView(),
		navigator:  navigator,
		templateID: templateID,
	}

	v.textView.SetDynamicColors(true)
	v.textView.SetBorder(true)
	v.textView.SetBorderColor(tcell.ColorDodgerBlue)
	v.textView.SetTitle(fmt.Sprintf(" Launch Template: %s ", templateID))
	v.textView.SetInputCapture(v.handleInput)

	return v
}

// Name returns the view identifier.
func (v *LaunchTemplateDetailView) Name() string {
	return "ec2/launch-template-detail"
}

// Render returns the tview primitive.
func (v *LaunchTemplateDetailView) Render() tview.Primitive {
	return v.textView
}

// Refresh reloads launch template details from AWS.
func (v *LaunchTemplateDetailView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()
	template, err := svc.GetLaunchTemplate(ctx, v.templateID)
	if err != nil {
		return err
	}

	v.template = template
	v.render()
	return nil
}

// render builds the text content for the detail view.
func (v *LaunchTemplateDetailView) render() {
	if v.template == nil {
		v.textView.SetText("[red]No data loaded")
		return
	}

	lt := v.template
	var b strings.Builder

	// Header
	b.WriteString("[yellow]═══ LAUNCH TEMPLATE ═══[-]\n\n")

	// Basic info
	section(&b, "BASIC INFORMATION")
	row(&b, "Launch Template ID", lt.LaunchTemplateID)
	row(&b, "Name", lt.LaunchTemplateName)
	row(&b, "Default Version", fmt.Sprintf("%d", lt.DefaultVersion))
	row(&b, "Latest Version", fmt.Sprintf("%d", lt.LatestVersion))
	row(&b, "Version Description", orDash(lt.VersionDescription))
	row(&b, "Created", lt.CreateTime.Format("2006-01-02 15:04:05 MST"))
	row(&b, "Created By", orDash(lt.CreatedBy))
	b.WriteString("\n")

	// Instance configuration
	section(&b, "INSTANCE CONFIGURATION")
	row(&b, "Instance Type", orDash(lt.InstanceType))
	row(&b, "AMI ID", orDash(lt.ImageID))
	row(&b, "Key Pair", orDash(lt.KeyName))
	row(&b, "EBS Optimized", boolStr(lt.EBSOptimized))
	row(&b, "Monitoring", boolStr(lt.MonitoringEnabled))
	row(&b, "Disable API Termination", boolStr(lt.DisableAPITerminate))
	row(&b, "Disable API Stop", boolStr(lt.DisableAPIStop))
	b.WriteString("\n")

	// Placement
	section(&b, "PLACEMENT")
	row(&b, "Availability Zone", orDash(lt.AvailabilityZone))
	row(&b, "Tenancy", orDash(lt.Tenancy))
	b.WriteString("\n")

	// Security
	section(&b, "SECURITY")
	row(&b, "IAM Instance Profile", orDash(lt.IAMInstanceProfile))
	if len(lt.SecurityGroupIDs) > 0 {
		row(&b, "Security Group IDs", strings.Join(lt.SecurityGroupIDs, ", "))
	} else {
		row(&b, "Security Group IDs", "-")
	}
	if len(lt.SecurityGroupNames) > 0 {
		row(&b, "Security Group Names", strings.Join(lt.SecurityGroupNames, ", "))
	}
	b.WriteString("\n")

	// Network interfaces
	if len(lt.NetworkInterfaces) > 0 {
		section(&b, "NETWORK INTERFACES")
		for i, ni := range lt.NetworkInterfaces {
			b.WriteString(fmt.Sprintf("  [white]Interface %d[-]\n", i))
			b.WriteString(fmt.Sprintf("    Device Index:       %d\n", ni.DeviceIndex))
			b.WriteString(fmt.Sprintf("    Subnet ID:          %s\n", orDash(ni.SubnetID)))
			b.WriteString(fmt.Sprintf("    Public IP:          %s\n", boolStr(ni.AssociatePublicIPAddress)))
			b.WriteString(fmt.Sprintf("    Delete on Term:     %s\n", boolStr(ni.DeleteOnTermination)))
			if len(ni.SecurityGroupIDs) > 0 {
				b.WriteString(fmt.Sprintf("    Security Groups:    %s\n", strings.Join(ni.SecurityGroupIDs, ", ")))
			}
			if ni.Description != "" {
				b.WriteString(fmt.Sprintf("    Description:        %s\n", ni.Description))
			}
		}
		b.WriteString("\n")
	}

	// Block devices / Storage
	if len(lt.BlockDeviceMappings) > 0 {
		section(&b, "STORAGE (BLOCK DEVICES)")
		for _, bd := range lt.BlockDeviceMappings {
			b.WriteString(fmt.Sprintf("  [white]%s[-]\n", bd.DeviceName))
			if bd.VolumeType != "" {
				b.WriteString(fmt.Sprintf("    Volume Type:        %s\n", bd.VolumeType))
			}
			if bd.VolumeSize > 0 {
				b.WriteString(fmt.Sprintf("    Size:               %d GiB\n", bd.VolumeSize))
			}
			if bd.IOPS > 0 {
				b.WriteString(fmt.Sprintf("    IOPS:               %d\n", bd.IOPS))
			}
			if bd.Throughput > 0 {
				b.WriteString(fmt.Sprintf("    Throughput:         %d MiB/s\n", bd.Throughput))
			}
			b.WriteString(fmt.Sprintf("    Encrypted:          %s\n", boolStr(bd.Encrypted)))
			b.WriteString(fmt.Sprintf("    Delete on Term:     %s\n", boolStr(bd.DeleteOnTermination)))
			if bd.SnapshotID != "" {
				b.WriteString(fmt.Sprintf("    Snapshot ID:        %s\n", bd.SnapshotID))
			}
		}
		b.WriteString("\n")
	}

	// Tag specifications
	if len(lt.TagSpecifications) > 0 {
		section(&b, "TAG SPECIFICATIONS")
		for _, ts := range lt.TagSpecifications {
			b.WriteString(fmt.Sprintf("  [white]%s[-]\n", ts.ResourceType))
			for k, val := range ts.Tags {
				b.WriteString(fmt.Sprintf("    %s: %s\n", k, val))
			}
		}
		b.WriteString("\n")
	}

	// Resource tags
	if len(lt.Tags) > 0 {
		section(&b, "RESOURCE TAGS")
		for k, val := range lt.Tags {
			row(&b, k, val)
		}
		b.WriteString("\n")
	}

	// User data (truncated preview)
	if lt.UserData != "" {
		section(&b, "USER DATA")
		preview := lt.UserData
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		b.WriteString(fmt.Sprintf("  [gray](base64, %d chars)[-]\n", len(lt.UserData)))
		b.WriteString(fmt.Sprintf("  %s\n", preview))
	}

	v.textView.SetText(b.String())
	v.textView.ScrollToBeginning()
}

// section writes a section header.
func section(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("[dodgerblue]── %s ──[-]\n", title))
}

// row writes a key-value row.
func row(b *strings.Builder, key, value string) {
	b.WriteString(fmt.Sprintf("  [white]%-24s[-] %s\n", key+":", value))
}

// boolStr returns "yes" or "no" for a boolean.
func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// Shortcuts returns detail view shortcuts.
func (v *LaunchTemplateDetailView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields (none for detail view).
func (v *LaunchTemplateDetailView) FilterFields() []string {
	return nil
}

// HandleFilter is a no-op for detail views.
func (v *LaunchTemplateDetailView) HandleFilter(_ string) {}

// handleInput processes view-specific key events.
func (v *LaunchTemplateDetailView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'R':
		// Refresh
		v.navigator.SetStatus("[yellow]Refreshing launch template...")
		go func() {
			ctx := v.navigator.Context()
			if err := v.Refresh(ctx); err != nil {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus(fmt.Sprintf("[red]Refresh failed: %s", err.Error()))
				})
			} else {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus("[green]Launch template refreshed")
				})
			}
		}()
		return nil
	}
	return event
}
