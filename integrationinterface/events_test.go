package integrationinterface

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseIntegrationEventAcceptsGenericEnvelope(t *testing.T) {
	raw := []byte(`{"schema_version":1,"kind":"Deploy-Finished","provider":"example","payload":{"release":"1.2.3"}}`)
	event, err := ParseIntegrationEvent(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got := event.CanonicalKind(); got != "deploy-finished" {
		t.Fatalf("expected canonical kind %q, got %q", "deploy-finished", got)
	}
	if event.Provider != "example" {
		t.Fatalf("unexpected provider %q", event.Provider)
	}
	var payload struct {
		Release string `json:"release"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("payload stays opaque but decodable: %v", err)
	}
	if payload.Release != "1.2.3" {
		t.Fatalf("unexpected payload %q", payload.Release)
	}
}

func TestIntegrationEventValidateRejectsUnusableEnvelopes(t *testing.T) {
	cases := map[string]IntegrationEvent{
		"missing kind":        {Payload: json.RawMessage(`{"a":1}`)},
		"blank kind":          {Kind: "   ", Payload: json.RawMessage(`{"a":1}`)},
		"missing payload":     {Kind: "deploy_finished"},
		"unsupported version": {SchemaVersion: EventSchemaVersion + 1, Kind: "deploy_finished", Payload: json.RawMessage(`{"a":1}`)},
		"negative version":    {SchemaVersion: -1, Kind: "deploy_finished", Payload: json.RawMessage(`{"a":1}`)},
	}
	for name, event := range cases {
		if err := event.Validate(); err == nil {
			t.Fatalf("%s: expected validation error", name)
		}
	}
}

func TestParseIntegrationEventRejectsMalformedJSON(t *testing.T) {
	if _, err := ParseIntegrationEvent([]byte("{")); err == nil {
		t.Fatal("expected malformed payload error")
	}
}

func TestDispatchIntegrationEventFansOutToRegisteredSink(t *testing.T) {
	integrationName := uniqueName("eventsink")
	var seen []string
	if err := Register(Definition{
		Name: integrationName,
		EventSink: func(_ context.Context, event IntegrationEvent) (any, error) {
			seen = append(seen, event.CanonicalKind())
			return map[string]string{"handled": event.CanonicalKind()}, nil
		},
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	var found *EventSinkRef
	sinks := EventSinks()
	for i := range sinks {
		if sinks[i].IntegrationName == integrationName {
			found = &sinks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected generic event sink for %q", integrationName)
	}

	event := IntegrationEvent{Kind: "deploy_finished", Payload: json.RawMessage(`{"release":"1.2.3"}`)}
	if err := DispatchIntegrationEvent(context.Background(), event); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if len(seen) != 1 || seen[0] != "deploy_finished" {
		t.Fatalf("expected one dispatched event, got %v", seen)
	}
}

func TestDispatchIntegrationEventWithoutSinkIsNoop(t *testing.T) {
	event := IntegrationEvent{Kind: uniqueName("unhandled"), Payload: json.RawMessage(`{}`)}
	if err := DispatchIntegrationEvent(context.Background(), event); err != nil {
		t.Fatalf("expected no-op dispatch, got %v", err)
	}
}

func TestDispatchIntegrationEventValidatesBeforeFanOut(t *testing.T) {
	if err := DispatchIntegrationEvent(context.Background(), IntegrationEvent{}); err == nil {
		t.Fatal("expected envelope validation error")
	}
}

func TestDispatchIntegrationEventJoinsSinkErrors(t *testing.T) {
	integrationName := uniqueName("eventsinkerr")
	boom := errors.New("sink exploded")
	if err := Register(Definition{
		Name: integrationName,
		EventSink: func(_ context.Context, _ IntegrationEvent) (any, error) {
			return nil, boom
		},
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	err := DispatchIntegrationEvent(context.Background(), IntegrationEvent{
		Kind:    uniqueName("kind"),
		Payload: json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("expected joined sink error")
	}
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), integrationName) {
		t.Fatalf("expected error attributed to %q, got %v", integrationName, err)
	}
}
