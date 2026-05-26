// Package eksview provides the EKS cluster and node group views.
package eksview

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/aws/eks"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// DetailView displays detailed information about an EKS cluster.
type DetailView struct {
	tabs      *components.TabbedView
	navigator Navigator
	name      string

	mu              sync.RWMutex
	cluster         *eks.Cluster
	nodeGroups      []eks.NodeGroup
	nodes           []ec2.Instance
	fargateProfiles []eks.FargateProfile

	// Tab content
	overviewTable   *tview.Table
	computeFlex     *tview.Flex
	nodeGroupsTable *components.SortableTable
	nodesTable      *components.SortableTable
	fargateTable    *components.SortableTable
	networkingTable *tview.Table
	accessTable     *tview.Table
	tagsTable       *components.SortableTable

	// Navigation targets for clickable fields
	navTargets map[string]navigation.Route
}

// NewDetailView creates a new EKS detail view.
func NewDetailView(navigator Navigator, clusterName string) *DetailView {
	v := &DetailView{
		navigator:  navigator,
		name:       clusterName,
		navTargets: make(map[string]navigation.Route),
	}

	// Overview tab
	v.overviewTable = tview.NewTable()
	v.overviewTable.SetBorders(false)
	v.overviewTable.SetSelectable(true, false)

	// Compute tab - 3 sections: Node Groups, Nodes, Fargate
	v.nodeGroupsTable = components.NewSortableTable(components.SortableTableConfig{
		Title: "Node Groups",
		Columns: []components.Column{
			{Title: "NAME", Field: "name", Expansion: 2},
			{Title: "STATUS", Field: "status", Expansion: 1},
			{Title: "CAPACITY", Field: "capacity", Expansion: 1},
			{Title: "INSTANCE TYPES", Field: "instances", Expansion: 2},
			{Title: "AMI TYPE", Field: "ami", Expansion: 1},
			{Title: "SCALING (min/desired/max)", Field: "scaling", Expansion: 1},
		},
		OnStatus: navigator.SetStatus,
	})

	v.nodesTable = components.NewSortableTable(components.SortableTableConfig{
		Title: "Nodes",
		Columns: []components.Column{
			{Title: "INSTANCE ID", Field: "instance", Expansion: 1},
			{Title: "NODE GROUP", Field: "nodegroup", Expansion: 1},
			{Title: "STATE", Field: "state", Expansion: 1},
			{Title: "TYPE", Field: "type", Expansion: 1},
			{Title: "AZ", Field: "az", Expansion: 1},
			{Title: "PRIVATE IP", Field: "ip", Expansion: 1},
		},
		OnStatus: navigator.SetStatus,
	})

	v.fargateTable = components.NewSortableTable(components.SortableTableConfig{
		Title: "Fargate Profiles",
		Columns: []components.Column{
			{Title: "NAME", Field: "name", Expansion: 2},
			{Title: "STATUS", Field: "status", Expansion: 1},
			{Title: "SELECTORS", Field: "selectors", Expansion: 3},
			{Title: "SUBNETS", Field: "subnets", Expansion: 2},
		},
		OnStatus: navigator.SetStatus,
	})

	// Stack the 3 tables vertically
	v.computeFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	v.computeFlex.AddItem(v.nodeGroupsTable.Table, 0, 1, true)
	v.computeFlex.AddItem(v.nodesTable.Table, 0, 1, false)
	v.computeFlex.AddItem(v.fargateTable.Table, 0, 1, false)

	// Networking tab
	v.networkingTable = tview.NewTable()
	v.networkingTable.SetBorders(false)
	v.networkingTable.SetSelectable(true, false)

	// Access tab
	v.accessTable = tview.NewTable()
	v.accessTable.SetBorders(false)
	v.accessTable.SetSelectable(true, false)

	// Tags tab
	v.tagsTable = components.NewSortableTable(components.SortableTableConfig{
		Title: "Tags",
		Columns: []components.Column{
			{Title: "KEY", Field: "key", Expansion: 1},
			{Title: "VALUE", Field: "value", Expansion: 2},
		},
		OnStatus: navigator.SetStatus,
	})

	pages := []components.TabPage{
		{Name: "Overview", Content: v.overviewTable},
		{Name: "Compute", Content: v.computeFlex},
		{Name: "Networking", Content: v.networkingTable},
		{Name: "Access", Content: v.accessTable},
		{Name: "Tags", Content: v.tagsTable.Table},
	}

	v.tabs = components.NewTabbedView(pages)
	v.tabs.SetExtraInput(v.handleInput)
	v.tabs.SetOnTabChanged(func(idx int, name string) {
		navigator.RefreshShortcuts()
	})

	return v
}

// Name returns the view identifier.
func (v *DetailView) Name() string {
	return "eks-detail"
}

// Render returns the tview primitive.
func (v *DetailView) Render() tview.Primitive {
	return v.tabs.Widget()
}

// Refresh reloads cluster data from AWS.
func (v *DetailView) Refresh(ctx context.Context) error {
	eksSvc := v.navigator.EKSService()
	ec2Svc := v.navigator.EC2Service()

	// Fetch cluster details
	cluster, err := eksSvc.GetCluster(ctx, v.name)
	if err != nil {
		return err
	}

	// Fetch node groups
	nodeGroups, err := eksSvc.ListAllNodeGroups(ctx, v.name)
	if err != nil {
		// Don't fail if node groups can't be fetched
		nodeGroups = nil
	}

	// Fetch Fargate profiles
	fargateProfiles, err := eksSvc.ListAllFargateProfiles(ctx, v.name)
	if err != nil {
		fargateProfiles = nil
	}

	// Fetch nodes (EC2 instances tagged for this cluster)
	// EKS tags instances with kubernetes.io/cluster/<cluster-name> = owned
	var nodes []ec2.Instance
	allInstances, err := ec2Svc.ListInstances(ctx, nil)
	if err == nil {
		for _, inst := range allInstances {
			// Check if instance belongs to this cluster
			if belongsToCluster(inst, v.name) {
				nodes = append(nodes, inst)
			}
		}
	}

	v.mu.Lock()
	v.cluster = cluster
	v.nodeGroups = nodeGroups
	v.nodes = nodes
	v.fargateProfiles = fargateProfiles
	v.mu.Unlock()

	v.rebuildOverview()
	v.rebuildCompute()
	v.rebuildNetworking()
	v.rebuildAccess()
	v.rebuildTags()

	return nil
}

// belongsToCluster checks if an EC2 instance belongs to the specified EKS cluster.
func belongsToCluster(inst ec2.Instance, clusterName string) bool {
	// Check for kubernetes.io/cluster/<cluster-name> tag
	clusterTag := fmt.Sprintf("kubernetes.io/cluster/%s", clusterName)
	for k, v := range inst.Tags {
		if k == clusterTag && (v == "owned" || v == "shared") {
			return true
		}
	}
	// Also check eks:cluster-name tag (used by EKS managed node groups)
	if v, ok := inst.Tags["eks:cluster-name"]; ok && v == clusterName {
		return true
	}
	return false
}

// Shortcuts returns tab-specific shortcuts.
func (v *DetailView) Shortcuts() []components.Shortcut {
	base := []components.Shortcut{
		{Key: "←/→", Label: "tabs"},
		{Key: "R", Label: "refresh"},
		{Key: "Esc", Label: "back"},
	}

	switch v.tabs.CurrentPageName() {
	case "Overview", "Networking":
		return append(base, components.Shortcut{Key: "Enter", Label: "navigate"})
	case "Compute":
		return append(base,
			components.Shortcut{Key: "s", Label: "sort-by"},
			components.Shortcut{Key: "d", Label: "sort-dir"},
		)
	case "Tags":
		return append(base,
			components.Shortcut{Key: "s", Label: "sort-by"},
			components.Shortcut{Key: "d", Label: "sort-dir"},
		)
	}
	return base
}

// FilterFields returns available filter fields.
func (v *DetailView) FilterFields() []string {
	return nil
}

// HandleFilter does nothing for detail view.
func (v *DetailView) HandleFilter(_ string) {}

// rebuildOverview populates the overview table.
func (v *DetailView) rebuildOverview() {
	v.mu.RLock()
	c := v.cluster
	v.mu.RUnlock()

	if c == nil {
		return
	}

	v.overviewTable.Clear()
	v.navTargets = make(map[string]navigation.Route)

	row := 0
	setRow := func(label, value string, nav *navigation.Route) {
		v.overviewTable.SetCell(row, 0,
			tview.NewTableCell(label).
				SetTextColor(tcell.ColorYellow).
				SetSelectable(false))

		cell := tview.NewTableCell(value).SetExpansion(1)
		if nav != nil {
			cell.SetTextColor(tcell.ColorDodgerBlue)
			value = value + " ↩"
			cell.SetText(value)
			v.navTargets[fmt.Sprintf("overview:%d", row)] = *nav
		}
		v.overviewTable.SetCell(row, 1, cell)
		row++
	}

	setHeader := func(title string) {
		v.overviewTable.SetCell(row, 0,
			tview.NewTableCell("[::b]"+title).
				SetTextColor(tcell.ColorWhite).
				SetSelectable(false))
		v.overviewTable.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
		row++
	}

	// Basic Info
	setHeader("Cluster Information")
	setRow("  Name", c.Name, nil)
	setRow("  ARN", c.ARN, nil)
	statusColor := "[white]"
	if c.Status == "ACTIVE" {
		statusColor = "[green]"
	} else if c.Status == "FAILED" {
		statusColor = "[red]"
	} else if c.Status == "CREATING" || c.Status == "UPDATING" {
		statusColor = "[yellow]"
	}
	setRow("  Status", statusColor+c.Status, nil)
	setRow("  Created", c.CreatedAt.Format("2006-01-02 15:04:05 MST"), nil)

	// Kubernetes Version
	setHeader("Kubernetes Version")
	setRow("  Kubernetes Version", c.Version, nil)
	setRow("  Platform Version", orDash(c.PlatformVersion), nil)
	if c.SupportType != "" {
		setRow("  Support Type", c.SupportType, nil)
	}

	// EKS Auto Mode / Compute Config
	setHeader("EKS Auto Mode")
	setRow("  Enabled", boolToYesNo(c.ComputeConfigEnabled), nil)
	if c.ComputeConfigEnabled {
		if len(c.NodePools) > 0 {
			setRow("  Node Pools", strings.Join(c.NodePools, ", "), nil)
		}
		if c.NodeRoleARN != "" {
			setRow("  Node Role ARN", c.NodeRoleARN, nil)
		}
		setRow("  Block Storage", boolToYesNo(c.BlockStorageEnabled), nil)
	}

	// Control Plane
	setHeader("Control Plane")
	if c.ControlPlaneInstanceType != "" {
		setRow("  Instance Type", c.ControlPlaneInstanceType, nil)
	} else {
		setRow("  Instance Type", "[gray]Managed by AWS", nil)
	}
	if c.ControlPlanePlacement != "" {
		setRow("  Placement Group", c.ControlPlanePlacement, nil)
	}

	// Zonal Shift
	setHeader("Availability")
	setRow("  ARC Zonal Shift", boolToYesNo(c.ZonalShiftEnabled), nil)

	// Encryption
	setHeader("Envelope Encryption")
	if c.EncryptionKeyARN != "" {
		setRow("  KMS Key ARN", c.EncryptionKeyARN, nil)
		if len(c.EncryptionResources) > 0 {
			setRow("  Encrypted Resources", strings.Join(c.EncryptionResources, ", "), nil)
		}
	} else {
		setRow("  Secrets Encryption", "[gray]Not configured", nil)
	}

	// API Server Endpoint
	setHeader("API Server Endpoint")
	setRow("  Endpoint URL", orDash(c.Endpoint), nil)
	setRow("  Public Access", boolToYesNo(c.EndpointPublicAccess), nil)
	setRow("  Private Access", boolToYesNo(c.EndpointPrivateAccess), nil)
	if c.EndpointPublicAccess && len(c.PublicAccessCidrs) > 0 {
		setRow("  Public Access CIDRs", strings.Join(c.PublicAccessCidrs, ", "), nil)
	}

	// OpenID Connect
	setHeader("OpenID Connect (OIDC)")
	if c.OIDCIssuer != "" {
		setRow("  Provider URL", c.OIDCIssuer, nil)
	} else {
		setRow("  Provider", "[gray]Not configured", nil)
	}

	// Authentication
	setHeader("Authentication & Access")
	if c.AuthenticationMode != "" {
		setRow("  Authentication Mode", c.AuthenticationMode, nil)
	}
	setRow("  Bootstrap Admin", boolToYesNo(c.BootstrapClusterCreatorAdminPermissions), nil)

	// IAM
	setHeader("IAM")
	setRow("  Cluster Role ARN", orDash(c.RoleARN), nil)

	// Certificate Authority
	setHeader("Certificate Authority")
	if c.CertificateAuthorityData != "" {
		setRow("  CA Data", c.CertificateAuthorityData, nil)
	} else {
		setRow("  CA Data", "[gray]Not available", nil)
	}

	// VPC (with navigation)
	setHeader("VPC Configuration")
	if c.VPCID != "" {
		setRow("  VPC ID", c.VPCID, &navigation.Route{Resource: "vpc-detail", ResourceID: c.VPCID})
	} else {
		setRow("  VPC ID", "-", nil)
	}
	setRow("  IP Family", orDash(c.IpFamily), nil)
	setRow("  Service IPv4 CIDR", orDash(c.ServiceIpv4Cidr), nil)
	if c.ServiceIpv6Cidr != "" {
		setRow("  Service IPv6 CIDR", c.ServiceIpv6Cidr, nil)
	}

	// Outposts (if configured)
	if len(c.OutpostARNs) > 0 {
		setHeader("Outposts")
		for _, arn := range c.OutpostARNs {
			setRow("  Outpost ARN", arn, nil)
		}
	}

	// Remote Networks (if configured)
	if len(c.RemoteNodeNetworks) > 0 || len(c.RemotePodNetworks) > 0 {
		setHeader("Remote Networks")
		if len(c.RemoteNodeNetworks) > 0 {
			setRow("  Remote Node CIDRs", strings.Join(c.RemoteNodeNetworks, ", "), nil)
		}
		if len(c.RemotePodNetworks) > 0 {
			setRow("  Remote Pod CIDRs", strings.Join(c.RemotePodNetworks, ", "), nil)
		}
	}
}

// rebuildCompute populates the compute tables: node groups, nodes, and Fargate profiles.
func (v *DetailView) rebuildCompute() {
	v.mu.RLock()
	nodeGroups := v.nodeGroups
	nodes := v.nodes
	fargateProfiles := v.fargateProfiles
	v.mu.RUnlock()

	// Node Groups table
	var ngRows []components.Row
	for _, ng := range nodeGroups {
		statusColor := statusToColor(ng.Status)
		instances := strings.Join(ng.InstanceTypes, ", ")
		if instances == "" {
			instances = "-"
		}
		scaling := fmt.Sprintf("%d / %d / %d", ng.MinSize, ng.DesiredSize, ng.MaxSize)

		ngRows = append(ngRows, components.Row{
			ID: ng.Name,
			Cells: []string{
				ng.Name,
				ng.Status,
				ng.CapacityType,
				instances,
				ng.AmiType,
				scaling,
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				statusColor,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
			},
		})
	}
	v.nodeGroupsTable.SetRows(ngRows)

	// Nodes table (EC2 instances)
	var nodeRows []components.Row
	for _, inst := range nodes {
		stateColor := tcell.ColorGreen
		if inst.State != "running" {
			stateColor = tcell.ColorYellow
		}
		if inst.State == "terminated" || inst.State == "stopped" {
			stateColor = tcell.ColorRed
		}

		// Try to get node group name from tags
		nodeGroup := "-"
		if ng, ok := inst.Tags["eks:nodegroup-name"]; ok {
			nodeGroup = ng
		}

		nodeRows = append(nodeRows, components.Row{
			ID: inst.InstanceID,
			Cells: []string{
				inst.InstanceID,
				nodeGroup,
				inst.State,
				inst.Type,
				inst.AZ,
				inst.PrivateIP,
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorLightGray,
				stateColor,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
			},
		})
	}
	v.nodesTable.SetRows(nodeRows)

	// Fargate profiles table
	var fpRows []components.Row
	for _, fp := range fargateProfiles {
		statusColor := statusToColor(fp.Status)
		
		// Build selector string: namespace (labels)
		var selectors []string
		for _, sel := range fp.Selectors {
			s := sel.Namespace
			if len(sel.Labels) > 0 {
				var labels []string
				for k, v := range sel.Labels {
					labels = append(labels, fmt.Sprintf("%s=%s", k, v))
				}
				s += " (" + strings.Join(labels, ", ") + ")"
			}
			selectors = append(selectors, s)
		}
		selectorStr := strings.Join(selectors, "; ")
		if selectorStr == "" {
			selectorStr = "-"
		}

		subnetsStr := strings.Join(fp.Subnets, ", ")
		if len(fp.Subnets) > 2 {
			subnetsStr = fmt.Sprintf("%s... (+%d)", fp.Subnets[0], len(fp.Subnets)-1)
		}
		if subnetsStr == "" {
			subnetsStr = "-"
		}

		fpRows = append(fpRows, components.Row{
			ID: fp.Name,
			Cells: []string{
				fp.Name,
				fp.Status,
				selectorStr,
				subnetsStr,
			},
			Colors: []tcell.Color{
				tcell.ColorWhite,
				statusColor,
				tcell.ColorLightGray,
				tcell.ColorLightGray,
			},
		})
	}
	v.fargateTable.SetRows(fpRows)
}

// rebuildNetworking populates the networking table.
func (v *DetailView) rebuildNetworking() {
	v.mu.RLock()
	c := v.cluster
	v.mu.RUnlock()

	if c == nil {
		return
	}

	v.networkingTable.Clear()

	row := 0
	setRow := func(label, value string, nav *navigation.Route) {
		v.networkingTable.SetCell(row, 0,
			tview.NewTableCell(label).
				SetTextColor(tcell.ColorYellow).
				SetSelectable(false))

		cell := tview.NewTableCell(value).SetExpansion(1)
		if nav != nil {
			cell.SetTextColor(tcell.ColorDodgerBlue)
			value = value + " ↩"
			cell.SetText(value)
			v.navTargets[fmt.Sprintf("networking:%d", row)] = *nav
		}
		v.networkingTable.SetCell(row, 1, cell)
		row++
	}

	setHeader := func(title string) {
		v.networkingTable.SetCell(row, 0,
			tview.NewTableCell("[::b]"+title).
				SetTextColor(tcell.ColorWhite).
				SetSelectable(false))
		v.networkingTable.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
		row++
	}

	// VPC
	setHeader("VPC Configuration")
	if c.VPCID != "" {
		setRow("  VPC ID", c.VPCID, &navigation.Route{Resource: "vpc-detail", ResourceID: c.VPCID})
	} else {
		setRow("  VPC ID", "-", nil)
	}

	// Subnets
	setHeader("Subnets")
	for _, subnetID := range c.SubnetIDs {
		setRow("  ", subnetID, &navigation.Route{Resource: "subnet-detail", ResourceID: subnetID})
	}
	if len(c.SubnetIDs) == 0 {
		setRow("  ", "(none)", nil)
	}

	// Security Groups
	setHeader("Security Groups")
	if c.ClusterSecurityGroup != "" {
		setRow("  Cluster SG", c.ClusterSecurityGroup, &navigation.Route{Resource: "sg-detail", ResourceID: c.ClusterSecurityGroup})
	}
	for _, sgID := range c.SecurityGroupIDs {
		setRow("  Additional SG", sgID, &navigation.Route{Resource: "sg-detail", ResourceID: sgID})
	}
	if c.ClusterSecurityGroup == "" && len(c.SecurityGroupIDs) == 0 {
		setRow("  ", "(none)", nil)
	}

	// API Server Endpoint
	setHeader("API Server Endpoint")
	setRow("  Endpoint URL", orDash(c.Endpoint), nil)
	setRow("  Public Access", boolToYesNo(c.EndpointPublicAccess), nil)
	setRow("  Private Access", boolToYesNo(c.EndpointPrivateAccess), nil)

	// Public Access CIDRs
	if c.EndpointPublicAccess && len(c.PublicAccessCidrs) > 0 {
		setHeader("Public Access Sources")
		for _, cidr := range c.PublicAccessCidrs {
			setRow("  ", cidr, nil)
		}
	}

	// Service IP Range
	setHeader("Cluster IP Configuration")
	setRow("  IP Family", orDash(c.IpFamily), nil)
	setRow("  Service IPv4 CIDR", orDash(c.ServiceIpv4Cidr), nil)
	if c.ServiceIpv6Cidr != "" {
		setRow("  Service IPv6 CIDR", c.ServiceIpv6Cidr, nil)
	}
}

func boolToYesNo(b bool) string {
	if b {
		return "[green]Yes"
	}
	return "[red]No"
}

// rebuildAccess populates the access table.
func (v *DetailView) rebuildAccess() {
	v.mu.RLock()
	c := v.cluster
	v.mu.RUnlock()

	if c == nil {
		return
	}

	v.accessTable.Clear()

	row := 0
	setRow := func(label, value string) {
		v.accessTable.SetCell(row, 0,
			tview.NewTableCell(label).
				SetTextColor(tcell.ColorYellow).
				SetSelectable(false))
		v.accessTable.SetCell(row, 1,
			tview.NewTableCell(value).SetExpansion(1))
		row++
	}

	setHeader := func(title string) {
		v.accessTable.SetCell(row, 0,
			tview.NewTableCell("[::b]"+title).
				SetTextColor(tcell.ColorWhite).
				SetSelectable(false))
		v.accessTable.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
		row++
	}

	// Identity
	setHeader("Identity & Authentication")
	setRow("  Cluster ARN", c.ARN)
	setRow("  IAM Role", orDash(c.RoleARN))
	setRow("  OIDC Issuer", orDash(c.OIDCIssuer))

	// Encryption
	setHeader("Encryption")
	if c.EncryptionKeyARN != "" {
		setRow("  KMS Key ARN", c.EncryptionKeyARN)
	} else {
		setRow("  Secrets Encryption", "[gray]Not configured")
	}

	// Logging
	setHeader("Control Plane Logging")
	if len(c.LoggingEnabled) > 0 {
		for _, logType := range c.LoggingEnabled {
			setRow("  "+logType, "[green]Enabled")
		}
	} else {
		setRow("  ", "[gray]No logging enabled")
	}

	// Log types that might not be enabled
	allLogTypes := []string{"api", "audit", "authenticator", "controllerManager", "scheduler"}
	enabledSet := make(map[string]bool)
	for _, lt := range c.LoggingEnabled {
		enabledSet[lt] = true
	}
	if len(c.LoggingEnabled) > 0 {
		for _, lt := range allLogTypes {
			if !enabledSet[lt] {
				setRow("  "+lt, "[red]Disabled")
			}
		}
	}
}

// rebuildTags populates the tags table.
func (v *DetailView) rebuildTags() {
	v.mu.RLock()
	c := v.cluster
	v.mu.RUnlock()

	if c == nil {
		return
	}

	rows := make([]components.Row, 0, len(c.Tags))
	for k, val := range c.Tags {
		rows = append(rows, components.Row{
			ID:    k,
			Cells: []string{k, val},
			Colors: []tcell.Color{
				tcell.ColorYellow,
				tcell.ColorWhite,
			},
		})
	}

	v.tagsTable.SetRows(rows)
}

// handleInput processes view-specific keys.
func (v *DetailView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		// Handle navigation for Overview and Networking tabs
		tabName := v.tabs.CurrentPageName()
		if tabName == "Overview" || tabName == "Networking" {
			var table *tview.Table
			if tabName == "Overview" {
				table = v.overviewTable
			} else {
				table = v.networkingTable
			}
			row, _ := table.GetSelection()
			key := fmt.Sprintf("%s:%d", strings.ToLower(tabName), row)
			if route, ok := v.navTargets[key]; ok {
				v.navigator.Navigate(route)
				return nil
			}
		}
	case tcell.KeyEscape:
		v.navigator.NavigateBack()
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
