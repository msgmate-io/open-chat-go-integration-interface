package integrationinterface

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDispatchEventUnknownKindIsNotAnError(t *testing.T) {
	handled, err := DispatchEvent(context.Background(), IntegrationEvent{
		SchemaVersion: EventSchemaVersion,
		Kind:          "not_registered_kind",
		Provider:      "example",
		Payload:       json.RawMessage(`{"anything":true}`),
	})
	if err != nil {
		t.Fatalf("DispatchEvent(unknown kind) error = %v, want nil", err)
	}
	if handled {
		t.Fatal("DispatchEvent(unknown kind) handled = true, want false")
	}
}

func TestDispatchEventFansOutToRegisteredKind(t *testing.T) {
	var (
		gotKind    string
		gotPayload string
	)
	MustRegister(Definition{
		Name: "event_sink_test_integration",
		EventSinks: map[string]EventSinkFunc{
			"ci_failure": func(_ context.Context, event IntegrationEvent) (bool, error) {
				gotKind = event.Kind
				gotPayload = string(event.Payload)
				return true, nil
			},
		},
	})

	handled, err := DispatchEvent(context.Background(), IntegrationEvent{
		SchemaVersion: EventSchemaVersion,
		Kind:          "CI_Failure",
		Provider:      "github",
		Payload:       json.RawMessage(`{"repo":"msgmate-io/open-chat-go"}`),
	})
	if err != nil {
		t.Fatalf("DispatchEvent(registered kind) error = %v, want nil", err)
	}
	if !handled {
		t.Fatal("DispatchEvent(registered kind) handled = false, want true")
	}
	if gotKind != "CI_Failure" {
		t.Fatalf("sink saw kind %q, want the raw kind", gotKind)
	}
	if gotPayload != `{"repo":"msgmate-io/open-chat-go"}` {
		t.Fatalf("sink saw payload %q, want the opaque payload", gotPayload)
	}
}

func TestEventSinkForKindNormalizesKind(t *testing.T) {
	MustRegister(Definition{
		Name: "event_sink_normalize_test_integration",
		EventSinks: map[string]EventSinkFunc{
			"ci_failure": func(context.Context, IntegrationEvent) (bool, error) { return true, nil },
		},
	})

	ref, ok := EventSinkForKind("  CI_Failure ")
	if !ok {
		t.Fatal("EventSinkForKind(normalized kind) not found")
	}
	if ref.IntegrationName != "event_sink_normalize_test_integration" {
		t.Fatalf("EventSinkForKind integration = %q, want the registering integration", ref.IntegrationName)
	}
	if _, ok := EventSinkForKind("   "); ok {
		t.Fatal("EventSinkForKind(blank kind) found a sink, want none")
	}
}

func TestDispatchEventRejectsUnsupportedSchemaVersion(t *testing.T) {
	handled, err := DispatchEvent(context.Background(), IntegrationEvent{
		SchemaVersion: EventSchemaVersion + 1,
		Kind:          "ci_failure",
	})
	if err == nil {
		t.Fatal("DispatchEvent(future schema version) error = nil, want an error")
	}
	if handled {
		t.Fatal("DispatchEvent(future schema version) handled = true, want false")
	}

	if _, err := DispatchEvent(context.Background(), IntegrationEvent{}); err == nil {
		t.Fatal("DispatchEvent(missing kind) error = nil, want an error")
	}
}

func TestEventSinksAreDeterministicallyOrdered(t *testing.T) {
	MustRegister(Definition{
		Name: "event_sink_order_a",
		EventSinks: map[string]EventSinkFunc{
			"b_kind": func(context.Context, IntegrationEvent) (bool, error) { return true, nil },
			"a_kind": func(context.Context, IntegrationEvent) (bool, error) { return true, nil },
		},
	})
	MustRegister(Definition{
		Name: "event_sink_order_b",
		EventSinks: map[string]EventSinkFunc{
			"a_kind": func(context.Context, IntegrationEvent) (bool, error) { return true, nil },
		},
	})

	refs := EventSinks()
	seen := make([]string, 0, len(refs))
	for _, ref := range refs {
		seen = append(seen, ref.IntegrationName+"/"+ref.Kind)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i-1] > seen[i] {
			t.Fatalf("EventSinks() not ordered: %v", seen)
		}
	}
}
