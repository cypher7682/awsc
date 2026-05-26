package ec2view

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpriestnall/awsc/internal/aws/cloudwatch"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// DetailView displays details for a single EC2 instance across multiple tabs.
type DetailView struct {
	tabs       *components.TabbedView
	navigator  Navigator
	instanceID string
	instance   *ec2.Instance
	sgs        []ec2.SecurityGroup

	// Page widgets
	overviewTable  *tview.Table
	networkTable   *tview.Table
	sgList         *tview.Table
	sgRules        *tview.Table
	tagsTable      *tview.Table
	selectedSGIdx  int

	// Navigation targets for selectable rows (indexed by "tab:row")
	navTargets map[string]navigation.Route

	// Monitoring tab
	monitoringPage *tview.Flex
	charts         map[cloudwatch.EC2MetricName]*components.Chart
}

// NewDetailView creates a new EC2 detail view.
func NewDetailView(navigator Navigator, instanceID string) *DetailView {
	v := &DetailView{
		navigator:  navigator,
		instanceID: instanceID,
		navTargets: make(map[string]navigation.Route),
	}

	// --- Overview page ---
	v.overviewTable = tview.NewTable()
	v.overviewTable.SetBorders(false)
	v.overviewTable.SetSelectable(true, false)
	v.overviewTable.SetBorder(true)
	v.overviewTable.SetTitle(" Overview ")
	v.overviewTable.SetBorderColor(tcell.ColorDodgerBlue)
	v.overviewTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDarkSlateGray).
		Foreground(tcell.ColorWhite))
	v.overviewTable.SetSelectedFunc(func(row, _ int) {
		v.navigateRow("overview", row)
	})

	// --- Networking page ---
	v.networkTable = tview.NewTable()
	v.networkTable.SetBorders(false)
	v.networkTable.SetSelectable(true, false)
	v.networkTable.SetBorder(true)
	v.networkTable.SetTitle(" Networking ")
	v.networkTable.SetBorderColor(tcell.ColorDodgerBlue)
	v.networkTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDarkSlateGray).
		Foreground(tcell.ColorWhite))
	v.networkTable.SetSelectedFunc(func(row, _ int) {
		v.navigateRow("network", row)
	})

	// --- Security Groups page (split: SG list top, rules bottom) ---
	v.sgList = tview.NewTable()
	v.sgList.SetBorders(false)
	v.sgList.SetSelectable(true, false)
	v.sgList.SetBorder(true)
	v.sgList.SetTitle(" Security Groups ")
	v.sgList.SetBorderColor(tcell.ColorDodgerBlue)
	v.sgList.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))
	v.sgList.SetSelectionChangedFunc(v.onSGSelectionChanged)

	v.sgRules = tview.NewTable()
	v.sgRules.SetBorders(false)
	v.sgRules.SetSelectable(true, false)
	v.sgRules.SetBorder(true)
	v.sgRules.SetTitle(" Inbound Rules ")
	v.sgRules.SetBorderColor(tcell.ColorGray)
	v.sgRules.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDarkSlateGray).
		Foreground(tcell.ColorWhite))

	sgPage := tview.NewFlex().SetDirection(tview.FlexRow)
	sgPage.AddItem(v.sgList, 0, 1, true)
	sgPage.AddItem(v.sgRules, 0, 1, false)

	// --- Tags page ---
	v.tagsTable = tview.NewTable()
	v.tagsTable.SetBorders(false)
	v.tagsTable.SetSelectable(true, false)
	v.tagsTable.SetBorder(true)
	v.tagsTable.SetTitle(" Tags ")
	v.tagsTable.SetBorderColor(tcell.ColorDodgerBlue)
	v.tagsTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDodgerBlue).
		Foreground(tcell.ColorWhite))

	// --- Monitoring page (CloudWatch charts) ---
	chartColors := map[cloudwatch.EC2MetricName]tcell.Color{
		cloudwatch.MetricCPUUtilization:    tcell.ColorDodgerBlue,
		cloudwatch.MetricNetworkIn:         tcell.ColorGreen,
		cloudwatch.MetricNetworkOut:        tcell.ColorDarkCyan,
		cloudwatch.MetricDiskReadBytes:     tcell.ColorYellow,
		cloudwatch.MetricDiskWriteBytes:    tcell.ColorOrange,
		cloudwatch.MetricStatusCheckFailed: tcell.ColorRed,
	}

	v.charts = make(map[cloudwatch.EC2MetricName]*components.Chart)
	// Layout: 3 rows of 2 charts each
	v.monitoringPage = tview.NewFlex().SetDirection(tview.FlexRow)
	metrics := cloudwatch.DefaultEC2Metrics
	for i := 0; i < len(metrics); i += 2 {
		row := tview.NewFlex().SetDirection(tview.FlexColumn)
		for j := 0; j < 2 && i+j < len(metrics); j++ {
			m := metrics[i+j]
			color := chartColors[m]
			if color == 0 {
				color = tcell.ColorWhite
			}
			chart := components.NewChart(string(m), cloudwatch.MetricUnit[m], "3h", color)
			v.charts[m] = chart
			row.AddItem(chart, 0, 1, false)
		}
		v.monitoringPage.AddItem(row, 0, 1, false)
	}

	// Build tabbed view
	v.tabs = components.NewTabbedView([]components.TabPage{
		{Name: "Overview", Content: v.overviewTable},
		{Name: "Networking", Content: v.networkTable},
		{Name: "Security Groups", Content: sgPage},
		{Name: "Monitoring", Content: v.monitoringPage},
		{Name: "Tags", Content: v.tagsTable},
	})
	v.tabs.SetExtraInput(v.handleInput)
	v.tabs.SetOnTabChanged(func(_ int, _ string) {
		v.navigator.RefreshShortcuts()
	})

	return v
}

// Name returns the view identifier.
func (v *DetailView) Name() string {
	return "ec2-detail"
}

// Render returns the tview primitive.
func (v *DetailView) Render() tview.Primitive {
	return v.tabs.Widget()
}

// Refresh reloads instance detail data from AWS.
func (v *DetailView) Refresh(ctx context.Context) error {
	svc := v.navigator.EC2Service()

	instance, err := svc.GetInstance(ctx, v.instanceID)
	if err != nil {
		return err
	}
	v.instance = instance

	// Load security groups
	if len(instance.SecurityGroupIDs) > 0 {
		sgs, err := svc.ListSecurityGroups(ctx, instance.SecurityGroupIDs)
		if err != nil {
			return err
		}
		v.sgs = sgs
	}

	v.renderOverview()
	v.renderNetworking()
	v.renderSGList()
	v.renderTags()

	// Fetch CloudWatch metrics (non-blocking — charts render as data arrives)
	go v.fetchMetrics()

	return nil
}

// fetchMetrics loads CloudWatch data for all default EC2 metrics.
func (v *DetailView) fetchMetrics() {
	cwSvc := v.navigator.CloudWatchService()
	if cwSvc == nil {
		return
	}

	ctx, cancel := context.WithTimeout(v.navigator.Context(), 30*time.Second)
	defer cancel()

	// 3 hours of data, 5-minute periods = ~36 data points per metric
	results, err := cwSvc.GetEC2Metrics(ctx, v.instanceID, cloudwatch.DefaultEC2Metrics, 300, 3*time.Hour)
	if err != nil {
		// If auth error, trigger background auto-login (user can refresh after)
		v.navigator.HandleAuthError(err)
		if err != nil {
			// Show a friendly error on each chart widget
			errMsg := simplifyCloudWatchError(err)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				for _, chart := range v.charts {
					chart.SetError(errMsg)
				}
				v.navigator.SetStatus(fmt.Sprintf("[yellow]CloudWatch: %s", errMsg))
			})
			return
		}
	}

	v.navigator.TviewApp().QueueUpdateDraw(func() {
		for _, result := range results {
			metricName := cloudwatch.EC2MetricName(result.Label)
			chart, ok := v.charts[metricName]
			if !ok {
				continue
			}
			datapoints := make([]components.ChartDatapoint, len(result.Datapoints))
			for i, dp := range result.Datapoints {
				datapoints[i] = components.ChartDatapoint{
					Value: dp.Value,
					Label: dp.Timestamp.Format("15:04"),
				}
			}
			chart.SetData(datapoints)
		}
	})
}

// Shortcuts returns context-specific shortcuts based on the active tab.
func (v *DetailView) Shortcuts() []components.Shortcut {
	common := []components.Shortcut{
		{Key: "\u2190/\u2192", Label: "tabs"},
		{Key: "c", Label: "connect"},
	}
	nav := []components.Shortcut{
		{Key: "v", Label: "goto VPC"},
		{Key: "n", Label: "goto subnet"},
	}
	instanceActions := []components.Shortcut{
		{Key: "Del", Label: "terminate"},
		{Key: "r", Label: "reboot"},
		{Key: "x", Label: "stop"},
		{Key: "a", Label: "start"},
	}
	back := []components.Shortcut{
		{Key: "Esc", Label: "back"},
	}

	switch v.tabs.CurrentPage() {
	case 4: // Tags
		return append(common, append([]components.Shortcut{
			{Key: "e", Label: "edit tag"},
			{Key: "+", Label: "add tag"},
			{Key: "Del", Label: "delete tag"},
		}, back...)...)
	case 0: // Overview
		return append(common, append(instanceActions, append(nav, back...)...)...)
	case 1: // Networking
		return append(common, append(instanceActions, append(nav, back...)...)...)
	case 2: // Security Groups
		return append(common, append(instanceActions, back...)...)
	case 3: // Monitoring
		return append(common, append(instanceActions, back...)...)
	default:
		return append(common, append(instanceActions, back...)...)
	}
}

// FilterFields returns available filter fields.
func (v *DetailView) FilterFields() []string {
	return nil
}

// HandleFilter applies a filter.
func (v *DetailView) HandleFilter(_ string) {}

// --- Render methods ---

// navigateRow handles Enter on a navigable row.
func (v *DetailView) navigateRow(tab string, row int) {
	key := fmt.Sprintf("%s:%d", tab, row)
	if route, ok := v.navTargets[key]; ok {
		v.navigator.Navigate(route)
	}
}

// setNavRow writes a key-value row into a table, with optional navigation target.
// If route is non-nil, appends a ↩ indicator to show it's navigable.
func (v *DetailView) setNavRow(table *tview.Table, tab string, row int, label, value string, route *navigation.Route) {
	cell0 := tview.NewTableCell("  " + label).
		SetTextColor(tcell.ColorDodgerBlue).
		SetSelectable(route != nil)
	table.SetCell(row, 0, cell0)

	displayValue := value
	valueColor := tcell.ColorWhite
	if route != nil {
		displayValue = value + " [gray]↩[-]"
		valueColor = tcell.ColorLightCyan
		v.navTargets[fmt.Sprintf("%s:%d", tab, row)] = *route
	}

	cell1 := tview.NewTableCell(displayValue).
		SetTextColor(valueColor).
		SetExpansion(1).
		SetSelectable(route != nil)
	table.SetCell(row, 1, cell1)
}

func (v *DetailView) renderOverview() {
	if v.instance == nil {
		return
	}
	inst := v.instance
	v.overviewTable.Clear()

	row := 0

	v.setNavRow(v.overviewTable, "overview", row, "Instance ID:", inst.InstanceID, nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "Name:", orDash(inst.Name), nil)
	row++

	// State row (custom coloring)
	v.overviewTable.SetCell(row, 0, tview.NewTableCell("  State:").SetTextColor(tcell.ColorDodgerBlue).SetSelectable(false))
	v.overviewTable.SetCell(row, 1, tview.NewTableCell(inst.State).SetTextColor(stateColor(inst.State)).SetSelectable(false))
	row++

	v.setNavRow(v.overviewTable, "overview", row, "Type:", inst.Type, nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "Platform:", orDash(inst.Platform), nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "AMI:", orDash(inst.AMI), nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "Key Name:", orDash(inst.KeyName), nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "AZ:", orDash(inst.AZ), nil)
	row++
	v.setNavRow(v.overviewTable, "overview", row, "Launch Time:", inst.LaunchTime.Format("2006-01-02 15:04:05 UTC"), nil)
	row++

	// Navigable: VPC
	if inst.VPCID != "" {
		v.setNavRow(v.overviewTable, "overview", row, "VPC:", inst.VPCID, &navigation.Route{Resource: "vpc-detail", ResourceID: inst.VPCID})
		row++
	}
	// Navigable: Subnet
	if inst.SubnetID != "" {
		v.setNavRow(v.overviewTable, "overview", row, "Subnet:", inst.SubnetID, &navigation.Route{Resource: "subnet-detail", ResourceID: inst.SubnetID})
		row++
	}

	v.overviewTable.SetTitle(fmt.Sprintf(" Overview: %s ", orDash(inst.Name)))
}

func (v *DetailView) renderNetworking() {
	if v.instance == nil {
		return
	}
	inst := v.instance
	v.networkTable.Clear()

	row := 0

	// Navigable: VPC
	if inst.VPCID != "" {
		v.setNavRow(v.networkTable, "network", row, "VPC ID:", inst.VPCID, &navigation.Route{Resource: "vpc-detail", ResourceID: inst.VPCID})
	} else {
		v.setNavRow(v.networkTable, "network", row, "VPC ID:", "-", nil)
	}
	row++

	// Navigable: Subnet
	if inst.SubnetID != "" {
		v.setNavRow(v.networkTable, "network", row, "Subnet ID:", inst.SubnetID, &navigation.Route{Resource: "subnet-detail", ResourceID: inst.SubnetID})
	} else {
		v.setNavRow(v.networkTable, "network", row, "Subnet ID:", "-", nil)
	}
	row++

	v.setNavRow(v.networkTable, "network", row, "AZ:", orDash(inst.AZ), nil)
	row++
	v.setNavRow(v.networkTable, "network", row, "Private IP:", orDash(inst.PrivateIP), nil)
	row++
	v.setNavRow(v.networkTable, "network", row, "Public IP:", orDash(inst.PublicIP), nil)
	row++

	// Separator
	v.networkTable.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	v.networkTable.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	row++

	// Security Groups - each navigable
	v.networkTable.SetCell(row, 0, tview.NewTableCell("  Security Groups:").SetTextColor(tcell.ColorDodgerBlue).SetSelectable(false))
	v.networkTable.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d attached", len(inst.SecurityGroupIDs))).SetTextColor(tcell.ColorWhite).SetSelectable(false))
	row++

	for _, sgID := range inst.SecurityGroupIDs {
		sgName := ""
		for _, sg := range v.sgs {
			if sg.GroupID == sgID {
				sgName = sg.GroupName
				break
			}
		}
		label := fmt.Sprintf("%s (%s)", sgID, sgName)
		v.setNavRow(v.networkTable, "network", row, "  ├─", label, &navigation.Route{Resource: "sg-detail", ResourceID: sgID})
		row++
	}

	v.networkTable.SetTitle(" Networking ")
}

func (v *DetailView) renderSGList() {
	v.sgList.Clear()

	headers := []string{"SG ID", "NAME", "INBOUND", "OUTBOUND", "DESCRIPTION"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.sgList.SetCell(0, col, cell)
	}

	for row, sg := range v.sgs {
		desc := sg.Description
		if len(desc) > 50 {
			desc = desc[:50] + "..."
		}
		v.sgList.SetCell(row+1, 0, tview.NewTableCell(sg.GroupID).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.sgList.SetCell(row+1, 1, tview.NewTableCell(sg.GroupName).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.sgList.SetCell(row+1, 2, tview.NewTableCell(fmt.Sprintf("%d", len(sg.IngressRules))).SetTextColor(tcell.ColorYellow).SetExpansion(1))
		v.sgList.SetCell(row+1, 3, tview.NewTableCell(fmt.Sprintf("%d", len(sg.EgressRules))).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.sgList.SetCell(row+1, 4, tview.NewTableCell(desc).SetTextColor(tcell.ColorGray).SetExpansion(1))
	}

	v.sgList.SetTitle(fmt.Sprintf(" Security Groups (%d) ", len(v.sgs)))

	// Render rules for first SG
	if len(v.sgs) > 0 {
		v.selectedSGIdx = 0
		v.renderSGRules(0)
	}
}

func (v *DetailView) renderSGRules(sgIdx int) {
	v.sgRules.Clear()

	if sgIdx < 0 || sgIdx >= len(v.sgs) {
		return
	}
	sg := v.sgs[sgIdx]

	headers := []string{"DIRECTION", "PROTOCOL", "PORTS", "SOURCE/DEST", "DESCRIPTION"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.sgRules.SetCell(0, col, cell)
	}

	row := 1
	for _, rule := range sg.IngressRules {
		ports := formatPorts(rule.FromPort, rule.ToPort)
		v.sgRules.SetCell(row, 0, tview.NewTableCell("[green]INBOUND").SetExpansion(1))
		v.sgRules.SetCell(row, 1, tview.NewTableCell(rule.Protocol).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.sgRules.SetCell(row, 2, tview.NewTableCell(ports).SetTextColor(tcell.ColorYellow).SetExpansion(1))
		v.sgRules.SetCell(row, 3, tview.NewTableCell(rule.Source).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.sgRules.SetCell(row, 4, tview.NewTableCell(orDash(rule.Description)).SetTextColor(tcell.ColorGray).SetExpansion(1))
		row++
	}
	for _, rule := range sg.EgressRules {
		ports := formatPorts(rule.FromPort, rule.ToPort)
		v.sgRules.SetCell(row, 0, tview.NewTableCell("[red]OUTBOUND").SetExpansion(1))
		v.sgRules.SetCell(row, 1, tview.NewTableCell(rule.Protocol).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		v.sgRules.SetCell(row, 2, tview.NewTableCell(ports).SetTextColor(tcell.ColorYellow).SetExpansion(1))
		v.sgRules.SetCell(row, 3, tview.NewTableCell(rule.Source).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.sgRules.SetCell(row, 4, tview.NewTableCell(orDash(rule.Description)).SetTextColor(tcell.ColorGray).SetExpansion(1))
		row++
	}

	v.sgRules.SetTitle(fmt.Sprintf(" Rules: %s (%d) ", sg.GroupName, row-1))
}

func (v *DetailView) renderTags() {
	v.tagsTable.Clear()

	headers := []string{"KEY", "VALUE"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorDodgerBlue).
			SetSelectable(false).
			SetExpansion(1)
		v.tagsTable.SetCell(0, col, cell)
	}

	if v.instance == nil {
		return
	}

	row := 1
	for k, val := range v.instance.Tags {
		v.tagsTable.SetCell(row, 0, tview.NewTableCell(k).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.tagsTable.SetCell(row, 1, tview.NewTableCell(val).SetTextColor(tcell.ColorLightGray).SetExpansion(1))
		row++
	}

	v.tagsTable.SetTitle(fmt.Sprintf(" Tags (%d) ", row-1))
}

// --- Event handlers ---

func (v *DetailView) onSGSelectionChanged(row, _ int) {
	if row <= 0 {
		return
	}
	idx := row - 1
	if idx >= len(v.sgs) {
		return
	}
	v.selectedSGIdx = idx
	v.renderSGRules(idx)
}

func (v *DetailView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if v.instance == nil {
		return event
	}

	// Tags tab specific keys (tab index 4)
	onTagsTab := v.tabs.CurrentPage() == 4

	// Delete key: terminate instance OR delete tag depending on active tab
	if event.Key() == tcell.KeyDelete {
		if onTagsTab {
			v.deleteSelectedTag()
			return nil
		}
		name := v.instance.Name
		if name == "" {
			name = v.instanceID
		}
		instanceID := v.instanceID
		v.navigator.ShowConfirm(fmt.Sprintf("Terminate %s?", name), func() {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]Terminating %s...", instanceID))
			go func() {
				err := v.navigator.EC2Service().TerminateInstance(v.navigator.Context(), instanceID)
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					if err != nil {
						v.navigator.SetStatus(fmt.Sprintf("[red]Failed: %s", err.Error()))
					} else {
						v.navigator.SetStatus(fmt.Sprintf("[green]Terminated %s", instanceID))
					}
				})
			}()
		})
		return nil
	}

	switch event.Rune() {
	case 'e':
		if onTagsTab {
			v.editSelectedTag()
			return nil
		}
	case '+':
		if onTagsTab {
			v.addNewTag()
			return nil
		}
	case 'c':
		v.navigator.RunEC2ConnectCmd(v.instanceID)
		return nil
	case 'v':
		if v.instance.VPCID != "" {
			v.navigator.Navigate(navigation.Route{
				Resource:   "vpc-detail",
				ResourceID: v.instance.VPCID,
			})
		}
		return nil
	case 'n':
		if v.instance.SubnetID != "" {
			v.navigator.Navigate(navigation.Route{
				Resource:   "subnet-detail",
				ResourceID: v.instance.SubnetID,
			})
		}
		return nil
	case 'r':
		name := v.instance.Name
		if name == "" {
			name = v.instanceID
		}
		instanceID := v.instanceID
		v.navigator.ShowConfirm(fmt.Sprintf("Reboot %s?", name), func() {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]Rebooting %s...", instanceID))
			go func() {
				err := v.navigator.EC2Service().RebootInstance(v.navigator.Context(), instanceID)
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					if err != nil {
						v.navigator.SetStatus(fmt.Sprintf("[red]Failed: %s", err.Error()))
					} else {
						v.navigator.SetStatus(fmt.Sprintf("[green]Rebooting %s", instanceID))
					}
				})
			}()
		})
		return nil
	case 'x':
		name := v.instance.Name
		if name == "" {
			name = v.instanceID
		}
		instanceID := v.instanceID
		v.navigator.ShowConfirm(fmt.Sprintf("Stop %s?", name), func() {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]Stopping %s...", instanceID))
			go func() {
				err := v.navigator.EC2Service().StopInstance(v.navigator.Context(), instanceID)
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					if err != nil {
						v.navigator.SetStatus(fmt.Sprintf("[red]Failed: %s", err.Error()))
					} else {
						v.navigator.SetStatus(fmt.Sprintf("[green]Stopping %s", instanceID))
					}
				})
			}()
		})
		return nil
	case 'a':
		instanceID := v.instanceID
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Starting %s...", instanceID))
		go func() {
			err := v.navigator.EC2Service().StartInstance(v.navigator.Context(), instanceID)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed: %s", err.Error()))
				} else {
					v.navigator.SetStatus(fmt.Sprintf("[green]Starting %s", instanceID))
				}
			})
		}()
		return nil
	}

	return event
}

// --- Tag editing ---

// selectedTagKey returns the key of the currently selected tag row.
func (v *DetailView) selectedTagKey() string {
	row, _ := v.tagsTable.GetSelection()
	if row <= 0 { // header row
		return ""
	}
	cell := v.tagsTable.GetCell(row, 0)
	if cell == nil {
		return ""
	}
	return cell.Text
}

// editSelectedTag prompts the user to edit the value of the selected tag.
func (v *DetailView) editSelectedTag() {
	key := v.selectedTagKey()
	if key == "" {
		return
	}
	currentValue := v.instance.Tags[key]
	instanceID := v.instanceID

	v.navigator.ShowInput(fmt.Sprintf("Tag '%s'", key), currentValue, func(newValue string) {
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Updating tag %s...", key))
		go func() {
			err := v.navigator.EC2Service().CreateTag(v.navigator.Context(), instanceID, key, newValue)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed to update tag: %s", err.Error()))
				} else {
					v.instance.Tags[key] = newValue
					v.renderTags()
					v.navigator.SetStatus(fmt.Sprintf("[green]Tag '%s' updated", key))
				}
			})
		}()
	})
}

// addNewTag prompts for a new tag key, then value.
func (v *DetailView) addNewTag() {
	instanceID := v.instanceID

	v.navigator.ShowInput("New tag key", "", func(key string) {
		if key == "" {
			v.navigator.SetStatus("[gray]Cancelled")
			return
		}
		v.navigator.ShowInput(fmt.Sprintf("Value for '%s'", key), "", func(value string) {
			v.navigator.SetStatus(fmt.Sprintf("[yellow]Creating tag %s...", key))
			go func() {
				err := v.navigator.EC2Service().CreateTag(v.navigator.Context(), instanceID, key, value)
				v.navigator.TviewApp().QueueUpdateDraw(func() {
					if err != nil {
						v.navigator.SetStatus(fmt.Sprintf("[red]Failed to create tag: %s", err.Error()))
					} else {
						if v.instance.Tags == nil {
							v.instance.Tags = make(map[string]string)
						}
						v.instance.Tags[key] = value
						v.renderTags()
						v.navigator.SetStatus(fmt.Sprintf("[green]Tag '%s' created", key))
					}
				})
			}()
		})
	})
}

// deleteSelectedTag confirms and deletes the selected tag.
func (v *DetailView) deleteSelectedTag() {
	key := v.selectedTagKey()
	if key == "" {
		return
	}
	instanceID := v.instanceID

	v.navigator.ShowConfirm(fmt.Sprintf("Delete tag '%s'?", key), func() {
		v.navigator.SetStatus(fmt.Sprintf("[yellow]Deleting tag %s...", key))
		go func() {
			err := v.navigator.EC2Service().DeleteTag(v.navigator.Context(), instanceID, key)
			v.navigator.TviewApp().QueueUpdateDraw(func() {
				if err != nil {
					v.navigator.SetStatus(fmt.Sprintf("[red]Failed to delete tag: %s", err.Error()))
				} else {
					delete(v.instance.Tags, key)
					v.renderTags()
					v.navigator.SetStatus(fmt.Sprintf("[green]Tag '%s' deleted", key))
				}
			})
		}()
	})
}

// --- Helpers ---

func stateColorName(state string) string {
	switch state {
	case "running":
		return "green"
	case "stopped":
		return "red"
	case "terminated", "shutting-down":
		return "gray"
	case "pending", "stopping":
		return "yellow"
	default:
		return "white"
	}
}

// formatPorts returns a human-readable port range string.
func formatPorts(from, to int32) string {
	if from == 0 && to == 0 {
		return "All"
	}
	if from == to {
		return fmt.Sprintf("%d", from)
	}
	return fmt.Sprintf("%d-%d", from, to)
}

// simplifyCloudWatchError returns a user-friendly error message.
func simplifyCloudWatchError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no EC2 IMDS role found") ||
		strings.Contains(msg, "get credentials") ||
		strings.Contains(msg, "failed to refresh cached credentials"):
		return "AWS credentials not configured (run aws sso login or set credentials)"
	case strings.Contains(msg, "context deadline exceeded"):
		return "Request timed out"
	case strings.Contains(msg, "AccessDenied") ||
		strings.Contains(msg, "UnauthorizedAccess"):
		return "Access denied (missing cloudwatch:GetMetricData permission)"
	default:
		// Truncate long messages
		if len(msg) > 80 {
			return msg[:80] + "..."
		}
		return msg
	}
}
