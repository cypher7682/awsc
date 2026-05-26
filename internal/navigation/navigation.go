// Package navigation provides the navigation stack and routing for awsc.
package navigation

import "fmt"

// Route represents a navigable destination in the TUI.
type Route struct {
	Resource   string            // e.g., "ec2", "ecr", "services", "sg", "vpc", "subnet"
	ResourceID string            // e.g., "i-abc123", "sg-123"
	Params     map[string]string // additional parameters
}

// String returns a human-readable representation of the route.
func (r Route) String() string {
	if r.ResourceID != "" {
		return fmt.Sprintf("%s/%s", r.Resource, r.ResourceID)
	}
	return r.Resource
}

// Stack maintains a navigation history for back/forward traversal.
type Stack struct {
	routes  []Route
	current int
}

// NewStack creates a new navigation stack with the services route as home.
func NewStack() *Stack {
	return &Stack{
		routes: []Route{
			{Resource: "services"},
		},
		current: 0,
	}
}

// Current returns the current route.
func (s *Stack) Current() Route {
	if s.current >= 0 && s.current < len(s.routes) {
		return s.routes[s.current]
	}
	return Route{Resource: "services"}
}

// Push navigates to a new route, discarding any forward history.
func (s *Stack) Push(route Route) {
	// Discard forward history
	s.routes = s.routes[:s.current+1]
	s.routes = append(s.routes, route)
	s.current++
}

// Back navigates to the previous route. Returns false if already at the beginning.
func (s *Stack) Back() bool {
	if s.current > 0 {
		s.current--
		return true
	}
	return false
}

// Forward navigates to the next route. Returns false if already at the end.
func (s *Stack) Forward() bool {
	if s.current < len(s.routes)-1 {
		s.current++
		return true
	}
	return false
}

// CanGoBack returns true if there is a previous route in the stack.
func (s *Stack) CanGoBack() bool {
	return s.current > 0
}

// CanGoForward returns true if there is a next route in the stack.
func (s *Stack) CanGoForward() bool {
	return s.current < len(s.routes)-1
}

// Depth returns the number of routes in the stack.
func (s *Stack) Depth() int {
	return len(s.routes)
}

// Breadcrumb returns a slice of route strings from root to current.
func (s *Stack) Breadcrumb() []string {
	crumbs := make([]string, 0, s.current+1)
	for i := 0; i <= s.current; i++ {
		crumbs = append(crumbs, s.routes[i].String())
	}
	return crumbs
}

// CommandRegistry maps command strings to route generators.
type CommandRegistry struct {
	commands map[string]func(args string) Route
}

// NewCommandRegistry creates a new command registry with default commands.
func NewCommandRegistry() *CommandRegistry {
	cr := &CommandRegistry{
		commands: make(map[string]func(args string) Route),
	}
	cr.registerDefaults()
	return cr
}

// Register adds a command to the registry.
func (cr *CommandRegistry) Register(name string, handler func(args string) Route) {
	cr.commands[name] = handler
}

// Resolve attempts to resolve a command string to a route.
func (cr *CommandRegistry) Resolve(command string) (Route, bool) {
	handler, ok := cr.commands[command]
	if ok {
		return handler(""), true
	}
	return Route{}, false
}

// AvailableCommands returns a list of all registered command names.
func (cr *CommandRegistry) AvailableCommands() []string {
	cmds := make([]string, 0, len(cr.commands))
	for name := range cr.commands {
		cmds = append(cmds, name)
	}
	return cmds
}

func (cr *CommandRegistry) registerDefaults() {
	cr.commands["services"] = func(_ string) Route {
		return Route{Resource: "services"}
	}
	cr.commands["ec2"] = func(_ string) Route {
		return Route{Resource: "ec2"}
	}
	cr.commands["ecr"] = func(_ string) Route {
		return Route{Resource: "ecr"}
	}
	cr.commands["eks"] = func(_ string) Route {
		return Route{Resource: "eks"}
	}
	cr.commands["sg"] = func(_ string) Route {
		return Route{Resource: "sg"}
	}
	cr.commands["vpc"] = func(_ string) Route {
		return Route{Resource: "vpc"}
	}
	cr.commands["subnet"] = func(_ string) Route {
		return Route{Resource: "subnet"}
	}
	cr.commands["subnets"] = func(_ string) Route {
		return Route{Resource: "subnet"}
	}
}
