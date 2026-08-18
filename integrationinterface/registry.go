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
