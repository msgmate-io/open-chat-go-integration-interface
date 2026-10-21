package integrationinterface

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/msgmate-io/go-tool-interface/toolinterface"
)

var (
	mu          sync.RWMutex
	definitions = map[string]Definition{}
)

func Register(def Definition) error {
	name := strings.ToLower(strings.TrimSpace(def.Name))
	if name == "" {
		return fmt.Errorf("integration definition requires a non-empty name")
	}

	tools := make([]toolinterface.Definition, 0, len(def.ToolDefinitions))
	seenToolNames := map[string]struct{}{}
	for idx, toolDef := range def.ToolDefinitions {
		toolName := strings.TrimSpace(toolDef.Name)
		if toolName == "" {
			return fmt.Errorf("integration definition %q tool_definitions[%d] requires a non-empty name", name, idx)
		}
		if toolDef.Run == nil {
			return fmt.Errorf("integration definition %q tool %q requires a run function", name, toolName)
		}
		if _, exists := seenToolNames[toolName]; exists {
			return fmt.Errorf("integration definition %q has duplicate tool name %q", name, toolName)
		}
		if toolinterface.Has(toolName) {
			return fmt.Errorf("integration definition %q tool %q already registered", name, toolName)
		}
		toolDef.Name = toolName
		seenToolNames[toolName] = struct{}{}
		tools = append(tools, toolDef)
	}

	mu.Lock()
	defer mu.Unlock()
	if _, exists := definitions[name]; exists {
		return fmt.Errorf("integration definition '%s' already registered", name)
	}

	for _, toolDef := range tools {
		if err := toolinterface.Register(toolDef); err != nil {
			return fmt.Errorf("integration definition %q failed to register tool %q: %w", name, toolDef.Name, err)
		}
	}

	def.Name = name
	def.ToolDefinitions = tools
	definitions[name] = def
	return nil
}

func MustRegister(def Definition) {
	if err := Register(def); err != nil {
		panic(err)
	}
}

func List() []Definition {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Definition, 0, len(names))
	for _, name := range names {
		result = append(result, definitions[name])
	}
	return result
}

// AuthEventSinkRef pairs an integration name with its registered auth event
// sink.
type AuthEventSinkRef struct {
	IntegrationName string
	Sink            AuthEventFunc
}

// AuthEventSinks returns the auth event sinks declared by registered
// integrations, ordered by integration name.
func AuthEventSinks() []AuthEventSinkRef {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]AuthEventSinkRef, 0, len(names))
	for _, name := range names {
		def := definitions[name]
		if def.AuthEventSink == nil {
			continue
		}
		out = append(out, AuthEventSinkRef{IntegrationName: name, Sink: def.AuthEventSink})
	}
	return out
}

// CLICommandRef pairs an integration name with a CLI command it contributes.
type CLICommandRef struct {
	IntegrationName string
	Command         CLICommand
}

// CLICommands returns the CLI commands contributed by registered
// integrations, ordered by integration name and declaration order. The core
// CLI appends them to the root command; compiled-out integrations contribute
// nothing.
func CLICommands() []CLICommandRef {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]CLICommandRef, 0)
	for _, name := range names {
		def := definitions[name]
		for _, cmd := range def.CLICommands {
			if cmd.Command == nil {
				continue
			}
			out = append(out, CLICommandRef{IntegrationName: name, Command: cmd})
		}
	}
	return out
}

func Get(name string) (Definition, bool) {
	mu.RLock()
	defer mu.RUnlock()
	name = strings.ToLower(strings.TrimSpace(name))
	def, ok := definitions[name]
	return def, ok
}

func Has(name string) bool {
	_, ok := Get(name)
	return ok
}

// EventHandlerRef pairs an integration name with its registered
// integration relay sink (ci#607).
type EventHandlerRef struct {
	IntegrationName string
	Sink            EventHandler
}

// IntegrationEventHandlers returns the integration relay sinks declared by registered
// integrations, ordered by integration name. Compiled-out integrations
// contribute nothing, so the relay always mirrors the integration profile of
// the build.
func IntegrationEventHandlers() []EventHandlerRef {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]EventHandlerRef, 0, len(names))
	for _, name := range names {
		def := definitions[name]
		if def.EventHandler == nil {
			continue
		}
		out = append(out, EventHandlerRef{IntegrationName: name, Sink: def.EventHandler})
	}
	return out
}
