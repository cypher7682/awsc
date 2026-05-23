# awsc - Full-screen AWS TUI (like K9s for AWS)

## Overview

Single-binary Go TUI for navigating AWS resources from the terminal. Vim-style keybindings, `:command` navigation, filtering, and resource management with confirmation gates on destructive ops.

## Build & Test

```bash
make build     # -> bin/awsc
make test      # go test ./...
make lint      # golangci-lint (if installed)
go build ./... # quick compile check
go test ./...  # all tests
```

## Project Structure

```
cmd/awsc/main.go          Entry point. Registers all views/factories, runs the app.
internal/
  aws/
    client.go             AWS SDK config wrapper (profile/region switching)
    ec2/service.go        EC2 service layer (list, describe, terminate, reboot, stop, start, SGs, VPCs, subnets)
    ecr/service.go        ECR service layer (repos, images, create, delete)
  config/
    config.go             AppConfig, LoadProfiles(), AWSRegions, INI parsing
  navigation/
    stack.go              Navigation stack (push/pop/back), Route struct
    commands.go           CommandRegistry for :command routing
  ui/
    app.go                Main TUI shell: layout, global input, navigate(), confirm system, completion wiring
    components/
      sort.go             SortableTable - generic reusable table with sort (Column/Row definitions)
      tabs.go             TabbedView - multi-page with left/right navigation and tab bar
      completion.go       CompletionList - popup widget for autocomplete above omnibox
      omnibox.go          Omnibox - command/filter input, GetCompletions(), profile/region lists
      header.go           Header bar - shortcuts display, breadcrumbs, profile/region context
    theme/                Color constants (if present)
    views/
      services/           Services landing page (list of AWS services)
      ec2/
        list.go           EC2 instances list (SortableTable, filter, select->detail)
        detail.go         EC2 instance detail (TabbedView: Overview, Networking, SGs, Tags)
        list_test.go      Tests for EC2 list view
      ecr/view.go         ECR repos + images views (SortableTable)
      sg/view.go          Security Groups list view
      vpc/view.go         VPC list view
      subnet/view.go      Subnet list view
```

## Key Patterns

### View interface

Every view implements `ui.View`:
```go
type View interface {
    Name() string                         // route key, e.g. "ec2", "ec2-detail"
    Render() tview.Primitive              // the widget to display
    Refresh(ctx context.Context) error    // load data from AWS
    Shortcuts() []components.Shortcut     // shortcut bar entries
    FilterFields() []string               // available filter fields
    HandleFilter(expression string)       // apply a filter
}
```

### Static views vs ViewFactories

- **Static views** are singletons registered once in `main.go` via `app.RegisterView(view)`. Good for list views that don't need construction-time params.
- **ViewFactories** are registered via `app.RegisterViewFactory(name, func(route) View)` for views that need route params (e.g., detail views needing a ResourceID). The factory is invoked on navigation; created views are cached by `name:resourceID`.

### Navigator interface

Views that need to trigger navigation, confirmations, or AWS calls accept a `Navigator` interface (defined in their own package, satisfied by `*App`):
```go
type Navigator interface {
    Navigate(route navigation.Route)
    NavigateBack()
    ShowConfirm(prompt string, onConfirm func())
    SetStatus(text string)
    EC2Service() *ec2.Service
    TviewApp() *tview.Application
    Context() context.Context
}
```

### SortableTable

Generic table component. Views declare `[]Column` (header, width, sort key) and provide `[]Row` data. The component handles:
- Rendering headers with sort indicators (up/down arrows)
- `s` key to cycle sort column, `d` key to toggle direction
- Selection highlighting

### TabbedView

Generic multi-page container. Pages are `[]TabPage{Name, Content}`. Handles:
- Left/right arrow for page switching (wraps around)
- Tab bar rendering with active highlight
- `SetExtraInput()` for view-specific key handling

### Confirmation gates

ALL destructive operations (terminate, delete, stop, reboot) MUST use `navigator.ShowConfirm(prompt, callback)`. This activates the omnibox in confirm mode (y/n). Never skip this.

### Shortcuts

Consistent across views:
- `s` = sort by (cycle column)
- `d` = sort direction (asc/desc toggle)
- `Del` = destructive action (terminate/delete)
- `Esc` = back
- `:` = command mode
- `/` = filter mode

EC2-specific: `r`=reboot, `x`=stop, `a`=start, `v`=goto VPC, `n`=goto subnet

### Service layer

Each AWS service has its own package under `internal/aws/`. Pattern:
- Define an interface (e.g., `EC2API`) wrapping the SDK calls you need
- Implement `Service` struct with the real SDK client
- Define domain structs (e.g., `Instance`, `SecurityGroup`) - don't leak SDK types to UI
- Tests use mock implementations of the interface

### Omnibox

Single `*tview.InputField` always in the draw tree. Modes:
- Command mode (`:` prefix): routes to views, switches profile/region
- Filter mode (`/` prefix): filters current view
- Confirm mode: y/n prompt for destructive ops

Important: tview's `SetFocus` doesn't reliably deliver keystrokes in the same frame. The global input handler calls `InputField.InputHandler()` directly to bypass focus routing.

### CompletionList

Custom popup that appears between the page content and omnibox (inserted via `rebuildLayout()`). Triggers for `profile=` and `region=` prefixes. Up/down to navigate, Enter/Tab to select, Esc to dismiss.

## Adding a New AWS Service

1. Create `internal/aws/<service>/service.go` with interface + implementation + domain structs
2. Create `internal/ui/views/<service>/view.go` implementing `ui.View`
3. Use `SortableTable` for list views
4. Register in `cmd/awsc/main.go`
5. Add `:command` route in the CommandRegistry
6. Add tests in `*_test.go` files alongside the code
7. Update README service coverage table

## Critical Notes

- **Never** use tview's autocomplete — we have a custom CompletionList for full UX control
- **rebuildLayout()** dynamically inserts/removes CompletionList from the Flex
- AWS calls MUST use `context.WithTimeout` (DefaultTimeout = 15s)
- Background AWS calls must use `app.QueueUpdateDraw()` to update UI from goroutines
- `orDash(s)` helper returns "-" for empty strings in detail views
