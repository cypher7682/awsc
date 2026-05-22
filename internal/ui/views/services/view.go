// Package services provides the service listing view.
package services

import (
	"context"

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
	list      *tview.List
	navigator Navigator
	services  []ServiceEntry
}

// NewView creates a new services view.
func NewView(navigator Navigator) *View {
	v := &View{
		list:      tview.NewList(),
		navigator: navigator,
		services: []ServiceEntry{
			{Name: "EC2", Command: "ec2", Description: "Elastic Compute Cloud - Virtual Servers", Status: "supported"},
			{Name: "ECR", Command: "ecr", Description: "Elastic Container Registry - Docker Images", Status: "supported"},
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

	v.list.SetTitle(" AWS Services ")
	v.list.SetBorder(true)
	v.list.SetBorderColor(tcell.ColorDodgerBlue)
	v.list.SetSelectedBackgroundColor(tcell.ColorDodgerBlue)
	v.list.SetSelectedTextColor(tcell.ColorWhite)
	v.list.ShowSecondaryText(true)
	v.list.SetHighlightFullLine(true)

	return v
}

// Name returns the view identifier.
func (v *View) Name() string {
	return "services"
}

// Render returns the tview primitive.
func (v *View) Render() tview.Primitive {
	return v.list
}

// Refresh reloads the service list.
func (v *View) Refresh(_ context.Context) error {
	v.list.Clear()
	for _, svc := range v.services {
		s := svc // capture
		statusIcon := "[green]\u2714" // checkmark
		if s.Status == "planned" {
			statusIcon = "[gray]\u2022" // bullet
		} else if s.Status == "partial" {
			statusIcon = "[yellow]\u25CB" // circle
		}

		mainText := statusIcon + "[-] " + s.Name
		secondaryText := "    " + s.Description

		v.list.AddItem(mainText, secondaryText, 0, func() {
			if s.Status != "planned" {
				v.navigator.Navigate(navigation.Route{Resource: s.Command})
			}
		})
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
