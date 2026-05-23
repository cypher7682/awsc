// Package ui provides the TUI application shell for awsc.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	awsclient "github.com/tpriestnall/awsc/internal/aws"
	"github.com/tpriestnall/awsc/internal/aws/cloudwatch"
	"github.com/tpriestnall/awsc/internal/aws/ec2"
	"github.com/tpriestnall/awsc/internal/aws/ecr"
	"github.com/tpriestnall/awsc/internal/config"
	"github.com/tpriestnall/awsc/internal/navigation"
	"github.com/tpriestnall/awsc/internal/ui/components"
)

// DefaultTimeout is the maximum time for any single AWS API call.
const DefaultTimeout = 15 * time.Second

// View is the interface that all resource views must implement.
type View interface {
	// Name returns the view identifier.
	Name() string
	// Render returns the tview.Primitive to display.
	Render() tview.Primitive
	// Refresh reloads data from AWS.
	Refresh(ctx context.Context) error
	// Shortcuts returns the context-specific shortcuts for this view.
	Shortcuts() []components.Shortcut
	// FilterFields returns the available filter fields for this view.
	FilterFields() []string
	// HandleFilter applies a filter expression.
	HandleFilter(expression string)
}

// ViewFactory creates a view dynamically based on the route (e.g., detail views
// that need a ResourceID at construction time).
type ViewFactory func(route navigation.Route) View

// App is the main TUI application.
type App struct {
	tviewApp   *tview.Application
	pages      *tview.Pages
	header     *components.Header
	omnibox    *components.Omnibox
	completion *components.CompletionList
	layout     *tview.Flex

	// Core state
	config   *config.AppConfig
	client   *awsclient.Client
	nav      *navigation.Stack
	commands *navigation.CommandRegistry

	// Services
	ec2Service  *ec2.Service
	ecrService  *ecr.Service
	cwService   *cloudwatch.Service

	// Views
	views        map[string]View
	viewFactories map[string]ViewFactory
	currentView  View

	// Confirmation callback
	confirmCallback func()

	// Context
	ctx    context.Context
	cancel context.CancelFunc
}

// NewApp creates a new application instance.
func NewApp(cfg *config.AppConfig) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client, err := awsclient.NewClient(ctx, cfg.Profile, cfg.Region)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating AWS client: %w", err)
	}

	tviewApp := tview.NewApplication()

	app := &App{
		tviewApp:   tviewApp,
		pages:      tview.NewPages(),
		header:     components.NewHeader(),
		omnibox:    components.NewOmnibox(),
		completion: components.NewCompletionList(),
		config:     cfg,
		client:     client,
		nav:        navigation.NewStack(),
		commands:   navigation.NewCommandRegistry(),
		views:      make(map[string]View),
		viewFactories: make(map[string]ViewFactory),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Initialize services
	awsCfg := client.Config()
	app.ec2Service = ec2.NewService(awsCfg)
	app.ecrService = ecr.NewService(awsCfg)
	app.cwService = cloudwatch.NewService(awsCfg)

	// Set up header context
	app.header.SetContext(cfg.Profile, cfg.Region)

	// Set up omnibox handler
	app.omnibox.SetHandler(app)
	app.omnibox.SetProfiles(config.LoadProfiles())
	app.omnibox.SetRegions(config.AWSRegions)

	// Wire completion list: when user picks an item, fill omnibox + submit
	app.completion.SetOnPick(func(text string) {
		app.omnibox.InputField.SetText(text)
		app.rebuildLayout()
		// Auto-submit the selected command
		app.omnibox.HandleInput()
	})

	// Build layout
	app.layout = tview.NewFlex().SetDirection(tview.FlexRow)
	app.layout.AddItem(app.header.Widget(), components.HeaderHeight, 0, false)
	app.layout.AddItem(app.pages, 0, 1, true)
	app.layout.AddItem(app.omnibox, 1, 0, false)

	// Set up global input handling
	tviewApp.SetInputCapture(app.globalInputHandler)

	return app, nil
}

// Run starts the TUI application.
func (a *App) Run() error {
	defer a.cancel()

	// Navigate to initial route
	a.navigate(a.nav.Current())

	a.tviewApp.SetRoot(a.layout, true)
	a.tviewApp.EnableMouse(false)
	return a.tviewApp.Run()
}

// Stop cleanly shuts down the application.
func (a *App) Stop() {
	a.cancel()
	a.tviewApp.Stop()
}

// rebuildLayout reconstructs the flex layout to show/hide the completion list.
func (a *App) rebuildLayout() {
	a.layout.Clear()
	a.layout.AddItem(a.header.Widget(), components.HeaderHeight, 0, false)
	if a.completion.Visible() {
		a.layout.AddItem(a.pages, 0, 1, true)
		a.layout.AddItem(a.completion.Widget(), a.completion.DesiredHeight(10), 0, false)
		a.layout.AddItem(a.omnibox, 1, 0, false)
	} else {
		a.layout.AddItem(a.pages, 0, 1, true)
		a.layout.AddItem(a.omnibox, 1, 0, false)
	}
}

// RegisterView adds a view to the application.
func (a *App) RegisterView(view View) {
	a.views[view.Name()] = view
}

// RegisterViewFactory registers a factory for dynamically-created views
// (e.g., detail views that require a ResourceID at construction time).
func (a *App) RegisterViewFactory(name string, factory ViewFactory) {
	a.viewFactories[name] = factory
}

// EC2Service returns the EC2 service instance.
func (a *App) EC2Service() *ec2.Service {
	return a.ec2Service
}

// ECRService returns the ECR service instance.
func (a *App) ECRService() *ecr.Service {
	return a.ecrService
}

// CloudWatchService returns the CloudWatch service instance.
func (a *App) CloudWatchService() *cloudwatch.Service {
	return a.cwService
}

// Navigate pushes a new route and renders the corresponding view.
func (a *App) Navigate(route navigation.Route) {
	a.nav.Push(route)
	a.navigate(route)
}

// NavigateBack goes back in history.
func (a *App) NavigateBack() {
	if a.nav.Back() {
		a.navigate(a.nav.Current())
	}
}

// TviewApp returns the underlying tview application (for views that need it).
func (a *App) TviewApp() *tview.Application {
	return a.tviewApp
}

// Context returns the application context.
func (a *App) Context() context.Context {
	return a.ctx
}

// Config returns the application config.
func (a *App) Config() *config.AppConfig {
	return a.config
}

// ShowConfirm activates the omnibox in confirm mode with a callback.
func (a *App) ShowConfirm(prompt string, onConfirm func()) {
	a.confirmCallback = onConfirm
	a.omnibox.SetConfirmPrompt(prompt)
}

// SetStatus updates the omnibox status text.
func (a *App) SetStatus(text string) {
	a.omnibox.SetStatus(text)
}

// navigate renders the view for the given route.
func (a *App) navigate(route navigation.Route) {
	viewName := route.Resource
	view, ok := a.views[viewName]
	if !ok {
		// Try factory for dynamic views (e.g., detail views needing a ResourceID)
		factory, hasFactory := a.viewFactories[viewName]
		if hasFactory {
			view = factory(route)
			// Cache it so back-navigation reuses the same instance
			a.views[viewName+":"+route.ResourceID] = view
		} else {
			// Also check cached dynamic views
			view, ok = a.views[viewName+":"+route.ResourceID]
			if !ok {
				a.omnibox.SetStatus(fmt.Sprintf("[red]Unknown resource: %s", viewName))
				return
			}
		}
	}

	a.currentView = view
	a.header.SetResource(route.String())
	a.header.SetShortcuts(view.Shortcuts())
	a.omnibox.SetFields(view.FilterFields())
	a.omnibox.SetStatus(fmt.Sprintf("[yellow]Loading %s...", route.String()))

	// Show a loading placeholder immediately
	loading := tview.NewTextView()
	loading.SetTextAlign(tview.AlignCenter)
	loading.SetDynamicColors(true)
	loading.SetText(fmt.Sprintf("\n\n\n[yellow]Loading %s...\n\n[gray]Press Esc to cancel, Ctrl+C to quit", route.String()))
	a.pages.RemovePage("current")
	a.pages.AddAndSwitchToPage("current", loading, true)

	// Refresh data in background with timeout
	go func() {
		timeoutCtx, timeoutCancel := context.WithTimeout(a.ctx, DefaultTimeout)
		defer timeoutCancel()

		err := view.Refresh(timeoutCtx)
		if err != nil {
			a.tviewApp.QueueUpdateDraw(func() {
				errView := tview.NewTextView()
				errView.SetTextAlign(tview.AlignCenter)
				errView.SetDynamicColors(true)
				errView.SetText(fmt.Sprintf("\n\n\n[red]Error loading %s:\n\n[white]%s\n\n[gray]Press Esc to go back, : to try another command", route.String(), err.Error()))
				a.pages.RemovePage("current")
				a.pages.AddAndSwitchToPage("current", errView, true)
				a.omnibox.SetStatus(fmt.Sprintf("[red]Error: %s", err.Error()))
			})
			return
		}
		a.tviewApp.QueueUpdateDraw(func() {
			a.pages.RemovePage("current")
			a.pages.AddAndSwitchToPage("current", view.Render(), true)
			a.omnibox.SetStatus(fmt.Sprintf("[green]%s loaded", route.String()))
		})
	}()
}

// OnCommand handles command input from the omnibox.
func (a *App) OnCommand(command string) {
	// Handle special commands
	if command == "quit" || command == "q" {
		a.Stop()
		return
	}

	// Handle region= commands
	if strings.HasPrefix(command, "region=") {
		region := strings.TrimPrefix(command, "region=")
		err := a.config.SetRegion(region)
		if err != nil {
			a.omnibox.SetStatus(fmt.Sprintf("[red]%s", err.Error()))
			return
		}
		// Reload AWS client with new region
		err = a.client.SetRegion(a.ctx, region)
		if err != nil {
			a.omnibox.SetStatus(fmt.Sprintf("[red]Failed to set region: %s", err.Error()))
			return
		}
		a.ec2Service = ec2.NewService(a.client.Config())
		a.ecrService = ecr.NewService(a.client.Config())
		a.header.SetContext(a.config.Profile, a.config.Region)
		a.omnibox.SetStatus(fmt.Sprintf("[green]Region set to %s", region))
		// Refresh current view
		if a.currentView != nil {
			a.navigate(a.nav.Current())
		}
		return
	}

	// Handle region command (show picker)
	if command == "region" {
		a.showRegionPicker()
		return
	}

	// Handle profile= commands
	if strings.HasPrefix(command, "profile=") {
		profile := strings.TrimPrefix(command, "profile=")
		err := a.client.SetProfile(a.ctx, profile)
		if err != nil {
			a.omnibox.SetStatus(fmt.Sprintf("[red]Failed to set profile: %s", err.Error()))
			return
		}
		a.config.SetProfile(profile)
		a.ec2Service = ec2.NewService(a.client.Config())
		a.ecrService = ecr.NewService(a.client.Config())
		a.header.SetContext(a.config.Profile, a.config.Region)
		a.omnibox.SetStatus(fmt.Sprintf("[green]Profile set to %s", profile))
		if a.currentView != nil {
			a.navigate(a.nav.Current())
		}
		return
	}

	// Try to resolve as a navigation command
	route, ok := a.commands.Resolve(command)
	if ok {
		a.Navigate(route)
		return
	}

	a.omnibox.SetStatus(fmt.Sprintf("[red]Unknown command: %s", command))
}

// OnFilter handles filter input from the omnibox.
func (a *App) OnFilter(filter string) {
	if a.currentView != nil {
		a.currentView.HandleFilter(filter)
		if filter == "" {
			a.omnibox.SetStatus("[green]Filter cleared")
		} else {
			a.omnibox.SetStatus(fmt.Sprintf("[yellow]Filter: %s", filter))
		}
	}
}

// OnConfirm handles confirmation input from the omnibox.
func (a *App) OnConfirm(confirmed bool) {
	if confirmed && a.confirmCallback != nil {
		cb := a.confirmCallback
		a.confirmCallback = nil
		cb()
	} else {
		a.confirmCallback = nil
		a.omnibox.SetStatus("[gray]Cancelled")
	}
}

// globalInputHandler handles application-wide key events.
func (a *App) globalInputHandler(event *tcell.EventKey) *tcell.EventKey {
	// Ctrl+C always quits - no matter what
	if event.Key() == tcell.KeyCtrlC {
		a.Stop()
		return nil
	}

	// If omnibox is active, route input directly to the InputField.
	// We bypass tview's focus-routing entirely because it doesn't reliably
	// deliver keystrokes to newly-focused widgets within the same frame.
	if a.omnibox.Mode() != components.OmniboxModeIdle {
		switch event.Key() {
		case tcell.KeyEscape:
			a.completion.Hide()
			a.omnibox.Deactivate()
			a.rebuildLayout()
			return nil

		case tcell.KeyEnter:
			// If completion popup is visible, accept the selection
			if a.completion.Visible() {
				a.completion.Accept()
				return nil
			}
			// Otherwise submit the command/filter
			a.omnibox.HandleInput()
			return nil

		case tcell.KeyUp:
			if a.completion.Visible() {
				a.completion.MoveUp()
				return nil
			}

		case tcell.KeyDown:
			if a.completion.Visible() {
				a.completion.MoveDown()
				return nil
			}

		case tcell.KeyTab, tcell.KeyBacktab:
			// Tab also accepts completion if visible
			if a.completion.Visible() {
				a.completion.Accept()
				return nil
			}

		default:
			// Deliver keystroke to the InputField, then update completions
			handler := a.omnibox.Input().InputHandler()
			if handler != nil {
				handler(event, func(p tview.Primitive) {})
			}
			// After text changes, update the completion list
			a.updateCompletions()
			return nil
		}
	}

	// Global shortcuts
	switch event.Key() {
	case tcell.KeyEscape:
		a.NavigateBack()
		return nil
	}

	switch event.Rune() {
	case ':':
		a.omnibox.Activate(components.OmniboxModeCommand)
		return nil
	case '/':
		a.omnibox.Activate(components.OmniboxModeFilter)
		return nil
	case 'q':
		a.Stop()
		return nil
	}

	return event
}

// updateCompletions refreshes the completion popup based on current omnibox text.
func (a *App) updateCompletions() {
	// Only show completions in command mode for profile=/region= prefixes
	if a.omnibox.Mode() != components.OmniboxModeCommand {
		if a.completion.Visible() {
			a.completion.Hide()
			a.rebuildLayout()
		}
		return
	}

	text := a.omnibox.InputField.GetText()
	items := a.omnibox.GetCompletions(text)

	if len(items) == 0 {
		if a.completion.Visible() {
			a.completion.Hide()
			a.rebuildLayout()
		}
		return
	}

	a.completion.Show(items)
	a.rebuildLayout()
}

// showRegionPicker displays a region selection modal.
func (a *App) showRegionPicker() {
	list := tview.NewList()
	list.SetTitle(" Select Region ")
	list.SetBorder(true)
	list.SetBorderColor(tcell.ColorDodgerBlue)

	for _, region := range config.AWSRegions {
		r := region // capture
		list.AddItem(r, "", 0, func() {
			a.pages.RemovePage("region-picker")
			a.OnCommand("region=" + r)
		})
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.pages.RemovePage("region-picker")
			return nil
		}
		return event
	})

	// Center the list
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, 20, 0, true).
			AddItem(nil, 0, 1, false), 40, 0, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddAndSwitchToPage("region-picker", modal, true)
	a.tviewApp.SetFocus(list)
}
