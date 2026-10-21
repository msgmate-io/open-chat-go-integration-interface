package integrationinterface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// This file defines the *generic* integration event contract. The public
// interface carries no domain vocabulary: it ships one versioned,
// provider-neutral envelope the core validates and fans out by kind, plus the
// handler an integration implements to consume it.
//
// Domain specific payload schemas (for example a CI failure), the HTTP ingress
// route that accepts them and the credentials that gate them belong to the
// emitting/consuming integration: it owns its payload, contributes its route
// through Definition.RouteRegistrar and declares its sink here. The core only
// validates the envelope and dispatches it, so adding a new event type never
// requires a core or public-interface change.

const (
	// EventSchemaVersion is the highest envelope version this module
	// understands. An event advertising a newer version must be rejected
	// instead of being guessed at.
	EventSchemaVersion = 1
	// EventKindIntegration is the kind of a wrapped, provider-agnostic event.
	EventKindIntegration = "integration_event"
)

// IntegrationEvent is the generic, versioned, secret-free envelope every
// integration event travels in. Payload stays opaque to the core: only the
// sink that registered for Kind decodes it.
type IntegrationEvent struct {
	SchemaVersion int             `json:"schema_version,omitempty"`
	Kind          string          `json:"kind"`
	Provider      string          `json:"provider,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

// CanonicalKind is the lower-cased, trimmed kind used for dispatch.
func (e IntegrationEvent) CanonicalKind() string {
	return strings.ToLower(strings.TrimSpace(e.Kind))
}

// Validate rejects an envelope the core cannot dispatch. It never inspects the
// provider specific payload.
func (e IntegrationEvent) Validate() error {
	if e.SchemaVersion < 0 {
		return fmt.Errorf("invalid integration_event schema_version %d", e.SchemaVersion)
	}
	if e.SchemaVersion > EventSchemaVersion {
		return fmt.Errorf("unsupported integration_event schema_version %d (max %d)", e.SchemaVersion, EventSchemaVersion)
	}
	if e.CanonicalKind() == "" {
		return errors.New("integration_event is missing kind")
	}
	if len(e.Payload) == 0 {
		return errors.New("integration_event is missing payload")
	}
	return nil
}

// IsZero reports whether the envelope carries no event at all.
func (e IntegrationEvent) IsZero() bool { return e.CanonicalKind() == "" }

// ParseIntegrationEvent decodes and validates a generic envelope.
func ParseIntegrationEvent(raw []byte) (IntegrationEvent, error) {
	var event IntegrationEvent
	if len(raw) == 0 {
		return event, errors.New("empty integration_event payload")
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return event, fmt.Errorf("invalid integration_event payload: %w", err)
	}
	if err := event.Validate(); err != nil {
		return event, err
	}
	return event, nil
}

// EventHandler consumes one generic integration event and returns an optional,
// JSON-encodable result. Handlers own the interpretation of the payload; they
// must be concurrency-safe and must not block the emitting provider.
type EventHandler func(ctx context.Context, event IntegrationEvent) (any, error)

// EventSinkRef pairs an integration name with its declared generic event sink.
type EventSinkRef struct {
	IntegrationName string
	Sink            EventHandler
}

// EventSinks returns the generic event sinks declared by registered
// integrations, ordered by integration name. Compiled-out integrations
// contribute nothing, so event handling always mirrors the integration profile
// of the build.
func EventSinks() []EventSinkRef {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]EventSinkRef, 0, len(names))
	for _, name := range names {
		def := definitions[name]
		if def.EventSink == nil {
			continue
		}
		out = append(out, EventSinkRef{IntegrationName: name, Sink: def.EventSink})
	}
	return out
}

// DispatchIntegrationEvent validates the envelope and hands it to every
// declared sink, joining their errors. With no sink declared it is a no-op, so
// the extension is backwards compatible by construction.
func DispatchIntegrationEvent(ctx context.Context, event IntegrationEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	var errs []error
	for _, ref := range EventSinks() {
		if ref.Sink == nil {
			continue
		}
		if _, err := ref.Sink(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", ref.IntegrationName, err))
		}
	}
	return errors.Join(errs...)
}
