// Package services provides the service listing view.
package services

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// ServiceEntry represents a navigable AWS service.
type ServiceEntry struct {
	Name        string
	Command     string
	Description string
	Status      string // "supported", "partial", "planned"
}

// Navigator is the interface for views to navigate.
type Navigator interface {
	Navigate(route navigation.Route)
}

// View displays the list of available services.
type View struct {
	table     *tview.Table
	navigator Navigator
	services  []ServiceEntry
}

// NewView creates a new services view.
func NewView(navigator Navigator) *View {
	v := &View{
		table:     tview.NewTable(),
		navigator: navigator,
		services: []ServiceEntry{
			{Name: "EC2", Command: "ec2", Description: "Elastic Compute Cloud - Virtual Servers", Status: "supported"},
			{Name: "ECR", Command: "ecr", Description: "Elastic Container Registry - Docker Images", Status: "supported"},
			{Name: "EKS", Command: "eks", Description: "Elastic Kubernetes Service - Managed Kubernetes", Status: "supported"},
			{Name: "VPC", Command: "vpc", Description: "Virtual Private Cloud - Networking", Status: "supported"},
			{Name: "Security Groups", Command: "sg", Description: "Security Groups - Firewall Rules", Status: "supported"},
			{Name: "Subnets", Command: "subnet", Description: "Subnets - Network Segments", Status: "supported"},
			{Name: "S3", Command: "s3", Description: "Simple Storage Service - Object Storage", Status: "planned"},
			{Name: "ECS", Command: "ecs", Description: "Elastic Container Service - Container Orchestration", Status: "planned"},
			{Name: "Lambda", Command: "lambda", Description: "Lambda - Serverless Functions", Status: "planned"},
			{Name: "RDS", Command: "rds", Description: "Relational Database Service", Status: "planned"},
			{Name: "CloudWatch", Command: "cloudwatch", Description: "CloudWatch - Monitoring & Logs", Status: "planned"},
			{Name: "IAM", Command: "iam", Description: "Identity & Access Management", Status: "planned"},
		},
	}

	v.table.SetTitle(" AWS Services ")
	v.table.SetBorder(true)
	v.table.SetBorderColor(tcell.ColorDodgerBlue)
	v.table.SetSelectable(true, false)
	v.table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	v.table.SetSelectedFunc(func(row, _ int) {
		if row < 0 || row >= len(v.services) {
			return
		}
		svc := v.services[row]
		if svc.Status != "planned" {
			v.navigator.Navigate(navigation.Route{Resource: svc.Command})
		}
	})

	return v
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
	v.table.Clear()
	for row, svc := range v.services {
		statusIcon := "[green]\u2714" // checkmark
		statusColor := tcell.ColorGreen
		if svc.Status == "planned" {
			statusIcon = "[gray]\u2022" // bullet
			statusColor = tcell.ColorGray
		} else if svc.Status == "partial" {
			statusIcon = "[yellow]\u25CB" // circle
			statusColor = tcell.ColorYellow
		}

		_ = statusIcon // used only for reference

		// Status column
		icon := "\u2714"
		if svc.Status == "planned" {
			icon = "\u2022"
		} else if svc.Status == "partial" {
			icon = "\u25CB"
		}
		v.table.SetCell(row, 0, tview.NewTableCell(icon).
			SetTextColor(statusColor).
			SetSelectable(true).
			SetExpansion(0))

		// Service name
		v.table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf(" %-18s", svc.Name)).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(true).
			SetExpansion(0))

		// Command shortcut
		cmdText := fmt.Sprintf(":%s", svc.Command)
		cmdColor := tcell.ColorDodgerBlue
		if svc.Status == "planned" {
			cmdColor = tcell.ColorDarkGray
		}
		v.table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf(" %-14s", cmdText)).
			SetTextColor(cmdColor).
			SetSelectable(true).
			SetExpansion(0))

		// Description
		descColor := tcell.ColorLightGray
		if svc.Status == "planned" {
			descColor = tcell.ColorDarkGray
		}
		v.table.SetCell(row, 3, tview.NewTableCell(" "+svc.Description).
			SetTextColor(descColor).
			SetSelectable(true).
			SetExpansion(1))
	}
	return nil
}

// Shortcuts returns the shortcuts for this view.
func (v *View) Shortcuts() []components.Shortcut {
	return []components.Shortcut{
		{Key: "Enter", Label: "open"},
		{Key: ":", Label: "command"},
		{Key: "/", Label: "filter"},
		{Key: "q", Label: "quit"},
	}
}

// FilterFields returns filterable fields.
func (v *View) FilterFields() []string {
	return []string{"name", "status"}
}

// HandleFilter applies a filter to the service list.
func (v *View) HandleFilter(expression string) {
	// Simple text match for now
	_ = expression
}
