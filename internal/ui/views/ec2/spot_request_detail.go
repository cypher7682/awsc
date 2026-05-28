// Package ec2view provides EC2 views including spot request details.
package ec2view

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// SpotRequestDetailView displays details of a single spot instance request.
type SpotRequestDetailView struct {
	textView  *tview.TextView
	navigator Navigator
	requestID string
	request   *ec2.SpotInstanceRequestDetail
}

// NewSpotRequestDetailView creates a new spot request detail view.
func NewSpotRequestDetailView(navigator Navigator, requestID string) *SpotRequestDetailView {
	v := &SpotRequestDetailView{
		textView:  tview.NewTextView(),
		navigator: navigator,
		requestID: requestID,
	}

	v.textView.SetDynamicColors(true)
	v.textView.SetBorder(true)
	v.textView.SetBorderColor(tcell.ColorDodgerBlue)
	v.textView.SetTitle(fmt.Sprintf(" Spot Request: %s ", requestID))
	v.textView.SetInputCapture(v.handleInput)

	return v
}

// Name returns the view identifier.
func (v *SpotRequestDetailView) Name() string {
	return "ec2/spot-detail"
}

// Render returns the tview primitive.
func (v *SpotRequestDetailView) Render() tview.Primitive {
	return v.textView
}

// Refresh reloads spot request details from AWS.
func (v *SpotRequestDetailView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()
	request, err := svc.GetSpotInstanceRequest(ctx, v.requestID)
	if err != nil {
		return err
	}

	v.request = request
	v.render()
	return nil
}

// render builds the text content for the detail view.
func (v *SpotRequestDetailView) render() {
	if v.request == nil {
		v.textView.SetText("[red]No data loaded")
		return
	}

	req := v.request
	var b strings.Builder

	// Header
	b.WriteString("[yellow]═══ SPOT INSTANCE REQUEST ═══[-]\n\n")

	// Request info
	spotSection(&b, "REQUEST INFORMATION")
	spotRow(&b, "Request ID", req.RequestID)
	spotRow(&b, "State", spotStateColorStr(req.State))
	spotRow(&b, "Status Code", req.StatusCode)
	if req.StatusMessage != "" {
		spotRow(&b, "Status Message", req.StatusMessage)
	}
	spotRow(&b, "Type", req.Type)
	spotRow(&b, "Created", req.CreateTime.Format("2006-01-02 15:04:05 MST"))
	if !req.ValidFrom.IsZero() {
		spotRow(&b, "Valid From", req.ValidFrom.Format("2006-01-02 15:04:05 MST"))
	}
	if !req.ValidUntil.IsZero() {
		spotRow(&b, "Valid Until", req.ValidUntil.Format("2006-01-02 15:04:05 MST"))
	}
	if req.LaunchGroup != "" {
		spotRow(&b, "Launch Group", req.LaunchGroup)
	}
	b.WriteString("\n")

	// Instance info (if fulfilled)
	if req.InstanceID != "" {
		spotSection(&b, "FULFILLED INSTANCE")
		spotRow(&b, "Instance ID", fmt.Sprintf("[green]%s[-] ↩", req.InstanceID))
		b.WriteString("\n")
	}

	// Pricing
	spotSection(&b, "PRICING")
	spotRow(&b, "Spot Price", orDash(req.SpotPrice))
	spotRow(&b, "Product Description", orDash(req.ProductDescription))
	b.WriteString("\n")

	// Launch specification
	spotSection(&b, "LAUNCH SPECIFICATION")
	spotRow(&b, "Instance Type", orDash(req.InstanceType))
	spotRow(&b, "AMI ID", orDash(req.ImageID))
	spotRow(&b, "Availability Zone", orDash(req.AvailabilityZone))
	spotRow(&b, "Key Pair", orDash(req.KeyName))
	spotRow(&b, "Subnet ID", orDash(req.SubnetID))
	spotRow(&b, "EBS Optimized", boolStr(req.EBSOptimized))
	spotRow(&b, "Monitoring", boolStr(req.Monitoring))
	if req.IAMInstanceProfile != "" {
		spotRow(&b, "IAM Profile", req.IAMInstanceProfile)
	}
	b.WriteString("\n")

	// Security groups
	if len(req.SecurityGroupIDs) > 0 {
		spotSection(&b, "SECURITY GROUPS")
		for _, sg := range req.SecurityGroupIDs {
			b.WriteString(fmt.Sprintf("  %s\n", sg))
		}
		b.WriteString("\n")
	}

	// Block devices
	if len(req.BlockDeviceMappings) > 0 {
		spotSection(&b, "BLOCK DEVICES")
		for _, bd := range req.BlockDeviceMappings {
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
			b.WriteString(fmt.Sprintf("    Encrypted:          %s\n", boolStr(bd.Encrypted)))
			b.WriteString(fmt.Sprintf("    Delete on Term:     %s\n", boolStr(bd.DeleteOnTermination)))
			if bd.SnapshotID != "" {
				b.WriteString(fmt.Sprintf("    Snapshot ID:        %s\n", bd.SnapshotID))
			}
		}
		b.WriteString("\n")
	}

	// Fault info
	if req.FaultCode != "" || req.FaultMessage != "" {
		spotSection(&b, "FAULT INFORMATION")
		if req.FaultCode != "" {
			spotRow(&b, "Fault Code", fmt.Sprintf("[red]%s[-]", req.FaultCode))
		}
		if req.FaultMessage != "" {
			spotRow(&b, "Fault Message", fmt.Sprintf("[red]%s[-]", req.FaultMessage))
		}
		b.WriteString("\n")
	}

	// Tags
	if len(req.Tags) > 0 {
		spotSection(&b, "TAGS")
		for k, val := range req.Tags {
			spotRow(&b, k, val)
		}
		b.WriteString("\n")
	}

	// User data (truncated preview)
	if req.UserData != "" {
		spotSection(&b, "USER DATA")
		preview := req.UserData
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		b.WriteString(fmt.Sprintf("  [gray](base64, %d chars)[-]\n", len(req.UserData)))
		b.WriteString(fmt.Sprintf("  %s\n", preview))
	}

	v.textView.SetText(b.String())
	v.textView.ScrollToBeginning()
}

// spotSection writes a section header.
func spotSection(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("[dodgerblue]── %s ──[-]\n", title))
}

// spotRow writes a key-value row.
func spotRow(b *strings.Builder, key, value string) {
	b.WriteString(fmt.Sprintf("  [white]%-20s[-] %s\n", key+":", value))
}

// spotStateColorStr returns the state with appropriate coloring.
func spotStateColorStr(state string) string {
	switch state {
	case "active":
		return "[green]" + state + "[-]"
	case "open":
		return "[yellow]" + state + "[-]"
	case "failed", "cancelled":
		return "[red]" + state + "[-]"
	case "closed":
		return "[gray]" + state + "[-]"
	default:
		return state
	}
}

// Shortcuts returns detail view shortcuts.
func (v *SpotRequestDetailView) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "i", Label: "go to instance"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}
}

// FilterFields returns available filter fields (none for detail view).
func (v *SpotRequestDetailView) FilterFields() []string {
	return nil
}

// HandleFilter is a no-op for detail views.
func (v *SpotRequestDetailView) HandleFilter(_ string) {}

// handleInput processes view-specific key events.
func (v *SpotRequestDetailView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'i':
		// Navigate to instance if fulfilled
		if v.request != nil && v.request.InstanceID != "" {
			v.navigator.Navigate(navigation.Route{
				Resource:   "ec2-detail",
				ResourceID: v.request.InstanceID,
			})
		} else {
			v.navigator.SetStatus("[yellow]No instance associated with this request")
		}
		return nil

	case 'R':
		// Refresh
		v.navigator.SetStatus("[yellow]Refreshing spot request...")
		go func() {
			ctx := v.navigator.Context()
			if err := v.Refresh(ctx); err != nil {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus(fmt.Sprintf("[red]Refresh failed: %s", err.Error()))
				})
			} else {
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					v.navigator.SetStatus("[green]Spot request refreshed")
				})
			}
		}()
		return nil
	}
	return event
}
