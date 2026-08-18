package integrationinterface

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/msgmate-io/go-tool-interface/toolinterface"
)

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func TestRegisterRegistersToolDefinitions(t *testing.T) {
	integrationName := uniqueName("integration")
	toolName := uniqueName("tool")

	err := Register(Definition{
		Name: integrationName,
		ToolDefinitions: []toolinterface.Definition{
			{
				Name:        toolName,
				Description: "test tool",
				InputType:   struct{}{},
				Parameters:  map[string]interface{}{},
				Run: func(input interface{}, init map[string]interface{}) (string, error) {
					_ = input
					_ = init
					return "ok", nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if !toolinterface.Has(toolName) {
		t.Fatalf("expected tool %q to be registered", toolName)
	}
}

func TestRegisterRejectsDuplicateToolAcrossIntegrations(t *testing.T) {
	toolName := uniqueName("duplicate_tool")

	firstIntegration := uniqueName("integration")
	if err := Register(Definition{
		Name: firstIntegration,
		ToolDefinitions: []toolinterface.Definition{
			{
				Name:        toolName,
				Description: "first",
				InputType:   struct{}{},
				Parameters:  map[string]interface{}{},
				Run: func(input interface{}, init map[string]interface{}) (string, error) {
					_ = input
					_ = init
					return "ok", nil
				},
			},
		},
	}); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	secondIntegration := uniqueName("integration")
	err := Register(Definition{
		Name: secondIntegration,
		ToolDefinitions: []toolinterface.Definition{
			{
				Name:        toolName,
				Description: "second",
				InputType:   struct{}{},
				Parameters:  map[string]interface{}{},
				Run: func(input interface{}, init map[string]interface{}) (string, error) {
					_ = input
					_ = init
					return "ok", nil
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected duplicate tool registration error")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected duplicate tool error, got: %v", err)
	}
}
