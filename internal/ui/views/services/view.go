// Package services provides the service hub view with expandable tree navigation.
package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// ServiceEntry represents a navigable AWS service or sub-resource.
type ServiceEntry struct {
	Name        string
	Command     string          // The command for this entry (e.g., "ec2/instances")
	Default     string          // For parent entries: the default child command (e.g., "ec2/instances")
	Description string
	Status      string          // "supported", "planned"
	Children    []*ServiceEntry // nil = leaf node
	Expanded    bool            // runtime state for tree nodes
	depth       int             // indentation level (set during flatten)
	parent      *ServiceEntry   // parent node (set during init)
}

// HasChildren returns true if this entry has child items.
func (e *ServiceEntry) HasChildren() bool {
	return len(e.Children) > 0
}

// Navigator is the interface for views to navigate.
type Navigator interface {
	Navigate(route navigation.Route)
	SetStatus(text string)
}

// View displays the hub with expandable service tree.
type View struct {
	table     *tview.Table
	navigator Navigator
	services  []*ServiceEntry
	visible   []*ServiceEntry // flattened visible items
	selected  int
}

// NewView creates a new hub view.
func NewView(navigator Navigator) *View {
	v := &View{
		table:     tview.NewTable(),
		navigator: navigator,
		services:  buildServiceTree(),
	}

	// Set parent references
	var setParents func(entries []*ServiceEntry, parent *ServiceEntry)
	setParents = func(entries []*ServiceEntry, parent *ServiceEntry) {
		for _, e := range entries {
			e.parent = parent
			if e.HasChildren() {
				setParents(e.Children, e)
			}
		}
	}
	setParents(v.services, nil)

	v.table.SetTitle(" AWS Services Hub ")
	v.table.SetBorder(true)
	v.table.SetBorderColor(tcell.ColorDodgerBlue)
	v.table.SetSelectable(true, false)
	v.table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	v.table.SetInputCapture(v.handleInput)

	return v
}

// buildServiceTree creates the full service hierarchy.
func buildServiceTree() []*ServiceEntry {
	return []*ServiceEntry{
		{
			Name: "EC2", Command: "ec2", Default: "ec2/instances", Description: "Elastic Compute Cloud", Status: "supported",
			Children: []*ServiceEntry{
				// Instances section
				{Name: "Instances", Command: "ec2/instances", Description: "Virtual Servers", Status: "supported"},
				{Name: "Instance Types", Command: "ec2/instance-types", Description: "Available Instance Types", Status: "supported"},
				{Name: "Launch Templates", Command: "ec2/launch-templates", Description: "Instance Launch Templates", Status: "supported"},
				{Name: "Spot Requests", Command: "ec2/spot", Description: "Spot Instance Requests", Status: "supported"},
				{Name: "Reserved Instances", Command: "ec2/reserved", Description: "Reserved Instance Purchases", Status: "planned"},
				{Name: "Dedicated Hosts", Command: "ec2/dedicated", Description: "Dedicated Physical Servers", Status: "planned"},
				{Name: "Capacity Reservations", Command: "ec2/capacity", Description: "On-Demand Capacity", Status: "planned"},
				// Images section
				{Name: "AMIs", Command: "ec2/ami", Description: "Amazon Machine Images", Status: "planned"},
				{Name: "AMI Catalog", Command: "ec2/ami-catalog", Description: "Public AMI Marketplace", Status: "planned"},
				// EBS section
				{Name: "Volumes", Command: "ec2/volumes", Description: "EBS Volumes", Status: "planned"},
				{Name: "Snapshots", Command: "ec2/snapshots", Description: "EBS Snapshots", Status: "planned"},
				{Name: "Lifecycle Manager", Command: "ec2/lifecycle", Description: "EBS Lifecycle Policies", Status: "planned"},
				// Network & Security section
				{Name: "Security Groups", Command: "ec2/sg", Description: "Firewall Rules", Status: "supported"},
				{Name: "Elastic IPs", Command: "ec2/eip", Description: "Static Public IPs", Status: "planned"},
				{Name: "Placement Groups", Command: "ec2/placement", Description: "Instance Placement", Status: "planned"},
				{Name: "Key Pairs", Command: "ec2/keypairs", Description: "SSH Key Pairs", Status: "planned"},
				{Name: "Network Interfaces", Command: "ec2/eni", Description: "Elastic Network Interfaces", Status: "planned"},
				// Load Balancing (nested under EC2)
				{
					Name: "Load Balancing", Command: "ec2/elb", Default: "ec2/elb/lb", Description: "Elastic Load Balancing", Status: "planned",
					Children: []*ServiceEntry{
						{Name: "Load Balancers", Command: "ec2/elb/lb", Description: "ALB / NLB / CLB", Status: "planned"},
						{Name: "Target Groups", Command: "ec2/elb/tg", Description: "Target Groups", Status: "planned"},
						{Name: "Trust Stores", Command: "ec2/elb/trust", Description: "mTLS Trust Stores", Status: "planned"},
					},
				},
				// Auto Scaling (nested under EC2)
				{
					Name: "Auto Scaling", Command: "ec2/asg", Default: "ec2/asg/groups", Description: "Auto Scaling Groups", Status: "planned",
					Children: []*ServiceEntry{
						{Name: "Auto Scaling Groups", Command: "ec2/asg/groups", Description: "Scaling Groups", Status: "planned"},
						{Name: "Launch Configurations", Command: "ec2/asg/launch-config", Description: "Legacy Launch Configs", Status: "planned"},
					},
				},
			},
		},
		{
			Name: "VPC", Command: "vpc", Default: "vpc/vpcs", Description: "Virtual Private Cloud", Status: "supported",
			Children: []*ServiceEntry{
				{Name: "VPCs", Command: "vpc/vpcs", Description: "Virtual Private Clouds", Status: "supported"},
				{Name: "Subnets", Command: "vpc/subnets", Description: "Network Segments", Status: "supported"},
				{Name: "Route Tables", Command: "vpc/routes", Description: "Routing Configuration", Status: "planned"},
				{Name: "Internet Gateways", Command: "vpc/igw", Description: "Internet Access", Status: "planned"},
				{Name: "NAT Gateways", Command: "vpc/nat", Description: "NAT for Private Subnets", Status: "planned"},
				{Name: "Peering Connections", Command: "vpc/peering", Description: "VPC Peering", Status: "planned"},
				{Name: "Endpoints", Command: "vpc/endpoints", Description: "VPC Endpoints", Status: "planned"},
			},
		},
		{Name: "EKS", Command: "eks", Description: "Elastic Kubernetes Service", Status: "supported"},
		{Name: "ECS", Command: "ecs", Description: "Elastic Container Service", Status: "planned"},
		{Name: "ECR", Command: "ecr", Description: "Elastic Container Registry", Status: "supported"},
		{Name: "Lambda", Command: "lambda", Description: "Serverless Functions", Status: "planned"},
		{Name: "S3", Command: "s3", Description: "Simple Storage Service", Status: "planned"},
		{Name: "RDS", Command: "rds", Description: "Relational Database Service", Status: "planned"},
		{Name: "DynamoDB", Command: "dynamodb", Description: "NoSQL Database", Status: "planned"},
		{Name: "Secrets Manager", Command: "asm", Description: "Secret Storage", Status: "supported"},
		{Name: "Systems Manager", Command: "ssm", Description: "Parameter Store & Run Command", Status: "planned"},
		{Name: "CloudWatch", Command: "cloudwatch", Description: "Monitoring & Logs", Status: "planned"},
		{Name: "IAM", Command: "iam", Description: "Identity & Access Management", Status: "planned"},
		{Name: "CloudFormation", Command: "cfn", Description: "Infrastructure as Code", Status: "planned"},
		{Name: "Route 53", Command: "r53", Description: "DNS & Domain Management", Status: "planned"},
		{Name: "SNS", Command: "sns", Description: "Simple Notification Service", Status: "planned"},
		{Name: "SQS", Command: "sqs", Description: "Simple Queue Service", Status: "planned"},
	}
}

// Name returns the view identifier.
func (v *View) Name() string {
	return "services"
}

// Render returns the tview primitive.
func (v *View) Render() tview.Primitive {
	return v.table
}

// Refresh reloads the service list.
func (v *View) Refresh(_ context.Context) error {
	v.flatten()
	v.render()
	return nil
}

// flatten builds the visible list based on expansion state.
func (v *View) flatten() {
	v.visible = nil
	var walk func(entries []*ServiceEntry, depth int)
	walk = func(entries []*ServiceEntry, depth int) {
		for _, e := range entries {
			e.depth = depth
			v.visible = append(v.visible, e)
			if e.HasChildren() && e.Expanded {
				walk(e.Children, depth+1)
			}
		}
	}
	walk(v.services, 0)
}

// render draws the table from visible items.
func (v *View) render() {
	v.table.Clear()

	for row, svc := range v.visible {
		// Build tree prefix for arbitrary depth
		prefix := v.buildTreePrefix(svc)

		// Status indicator
		statusIcon := "✔"
		statusColor := tcell.ColorGreen
		if svc.Status == "planned" {
			statusIcon = "○"
			statusColor = tcell.ColorGray
		}

		// Colors based on status
		nameColor := tcell.ColorWhite
		cmdColor := tcell.ColorDodgerBlue
		descColor := tcell.ColorLightGray
		prefixColor := tcell.ColorDarkGray
		if svc.Status == "planned" {
			nameColor = tcell.ColorGray
			cmdColor = tcell.ColorDarkGray
			descColor = tcell.ColorDarkGray
		}
		// Highlight expandable nodes
		if svc.HasChildren() {
			prefixColor = tcell.ColorYellow
		}

		// Tree prefix column
		v.table.SetCell(row, 0, tview.NewTableCell(prefix).
			SetTextColor(prefixColor).
			SetSelectable(true).
			SetExpansion(0))

		// Status column
		v.table.SetCell(row, 1, tview.NewTableCell(statusIcon+" ").
			SetTextColor(statusColor).
			SetSelectable(true).
			SetExpansion(0))

		// Service name
		v.table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("%-24s", svc.Name)).
			SetTextColor(nameColor).
			SetSelectable(true).
			SetExpansion(0))

		// Command shortcut
		cmdText := fmt.Sprintf(":%s", svc.Command)
		v.table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf("%-28s", cmdText)).
			SetTextColor(cmdColor).
			SetSelectable(true).
			SetExpansion(0))

		// Description
		v.table.SetCell(row, 4, tview.NewTableCell(svc.Description).
			SetTextColor(descColor).
			SetSelectable(true).
			SetExpansion(1))
	}

	// Restore selection
	if v.selected >= len(v.visible) {
		v.selected = len(v.visible) - 1
	}
	if v.selected < 0 {
		v.selected = 0
	}
	v.table.Select(v.selected, 0)
}

// buildTreePrefix constructs the tree drawing prefix for a node at any depth.
// Handles arbitrary nesting with proper continuation lines and expand indicators.
func (v *View) buildTreePrefix(svc *ServiceEntry) string {
	if svc.depth == 0 {
		// Top-level: just show expand indicator
		if svc.HasChildren() {
			if svc.Expanded {
				return "▼ "
			}
			return "▶ "
		}
		return "  "
	}

	// Build continuation lines by walking up the ancestor chain
	// We need to know for each ancestor level whether to draw │ or space
	continuations := make([]string, svc.depth-1)
	ancestor := svc.parent
	for i := svc.depth - 2; i >= 0; i-- {
		if ancestor != nil && !v.isLastChild(ancestor) {
			continuations[i] = "│ "
		} else {
			continuations[i] = "  "
		}
		if ancestor != nil {
			ancestor = ancestor.parent
		}
	}

	// Build the prefix: continuations + branch + expand indicator
	var prefix strings.Builder
	for _, cont := range continuations {
		prefix.WriteString(cont)
	}

	// Branch character
	if v.isLastChild(svc) {
		prefix.WriteString("└─")
	} else {
		prefix.WriteString("├─")
	}

	// Expand indicator for nested parents
	if svc.HasChildren() {
		if svc.Expanded {
			prefix.WriteString("▼")
		} else {
			prefix.WriteString("▶")
		}
	}

	return prefix.String()
}

// isLastChild returns true if this entry is the last child of its parent.
func (v *View) isLastChild(svc *ServiceEntry) bool {
	if svc.parent == nil {
		return false
	}
	children := svc.parent.Children
	return len(children) > 0 && children[len(children)-1] == svc
}

// handleInput processes keyboard events.
func (v *View) handleInput(event *tcell.EventKey) *tcell.EventKey {
	row, _ := v.table.GetSelection()
	if row < 0 || row >= len(v.visible) {
		return event
	}
	svc := v.visible[row]

	switch event.Key() {
	case tcell.KeyRight:
		// Expand or navigate
		if svc.HasChildren() && !svc.Expanded {
			svc.Expanded = true
			v.selected = row
			v.flatten()
			v.render()
			return nil
		} else if !svc.HasChildren() && svc.Status == "supported" {
			// Navigate to leaf
			v.navigator.Navigate(navigation.Route{Resource: svc.Command})
			return nil
		}
	case tcell.KeyLeft:
		// Collapse or go to parent
		if svc.HasChildren() && svc.Expanded {
			svc.Expanded = false
			v.selected = row
			v.flatten()
			v.render()
			return nil
		} else if svc.parent != nil {
			// Find parent in visible list and select it
			for i, e := range v.visible {
				if e == svc.parent {
					v.selected = i
					v.table.Select(i, 0)
					return nil
				}
			}
		}
	case tcell.KeyEnter:
		if svc.HasChildren() {
			// If parent has a default, navigate to it
			if svc.Default != "" && svc.Status == "supported" {
				v.navigator.Navigate(navigation.Route{Resource: svc.Default})
			} else {
				// Toggle expansion
				svc.Expanded = !svc.Expanded
				v.selected = row
				v.flatten()
				v.render()
			}
		} else if svc.Status == "supported" {
			v.navigator.Navigate(navigation.Route{Resource: svc.Command})
		} else {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]%s is not yet implemented", svc.Name))
		}
		return nil
	}

	// Vim-style navigation
	switch event.Rune() {
	case 'l':
		// Same as Right
		if svc.HasChildren() && !svc.Expanded {
			svc.Expanded = true
			v.selected = row
			v.flatten()
			v.render()
			return nil
		} else if !svc.HasChildren() && svc.Status == "supported" {
			v.navigator.Navigate(navigation.Route{Resource: svc.Command})
			return nil
		}
	case 'h':
		// Same as Left
		if svc.HasChildren() && svc.Expanded {
			svc.Expanded = false
			v.selected = row
			v.flatten()
			v.render()
			return nil
		} else if svc.parent != nil {
			for i, e := range v.visible {
				if e == svc.parent {
					v.selected = i
					v.table.Select(i, 0)
					return nil
				}
			}
		}
	}

	return event
}

// Shortcuts returns the shortcuts for this view.
func (v *View) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "→/l", Label: "expand/open"},
		{Key: "←/h", Label: "collapse"},
		{Key: "Enter", Label: "toggle/open"},
		{Key: ":", Label: "command"},
		{Key: "/", Label: "filter"},
	}
}

// FilterFields returns filterable fields.
func (v *View) FilterFields() []string {
	return []string{"name", "command", "status"}
}

// HandleFilter applies a filter to the service list.
func (v *View) HandleFilter(expression string) {
	// TODO: implement filtering for tree view
	_ = expression
}
