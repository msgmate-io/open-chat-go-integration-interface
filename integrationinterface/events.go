package integrationinterface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// This file defines the generic, integration-neutral event contract: a
// versioned envelope an external system hands Open-Chat through an integration,
// plus the handler an integration implements to consume it. The payload is
// opaque to the core: the emitting side and the consuming integration own its
// schema, the core only validates the envelope and fans out by kind.

// IntegrationEventSchemaVersion is the highest envelope version this module
// understands. An event carrying a newer version must be rejected instead of
// being guessed at.
const IntegrationEventSchemaVersion = 1

// IntegrationEvent is the generic, versioned, secret-free envelope for an event
// relayed through an integration. Payload is the opaque, kind-specific document
// owned by the producer and the consuming integration.
//
// Emitters must keep Payload small and free of environment dumps, tokens and
// other secret material.
type IntegrationEvent struct {
	SchemaVersion int             `json:"schema_version,omitempty"`
	Kind          string          `json:"kind"`
	Provider      string          `json:"provider,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

// NormalizedKind is the lower-cased, trimmed kind used for registration and
// lookup.
func (e IntegrationEvent) NormalizedKind() string {
	return strings.ToLower(strings.TrimSpace(e.Kind))
}

// Validate rejects an envelope that must not be dispatched: no kind and a
// schema version this module cannot understand.
func (e IntegrationEvent) Validate() error {
	if e.NormalizedKind() == "" {
		return errors.New("integration event is missing kind")
	}
	if e.SchemaVersion < 0 {
		return fmt.Errorf("invalid integration event schema_version %d", e.SchemaVersion)
	}
	if e.SchemaVersion > IntegrationEventSchemaVersion {
		return fmt.Errorf("unsupported integration event schema_version %d (max %d)", e.SchemaVersion, IntegrationEventSchemaVersion)
	}
	return nil
}

// ParseIntegrationEvent decodes and validates a relayed event envelope.
func ParseIntegrationEvent(raw []byte) (IntegrationEvent, error) {
	var event IntegrationEvent
	if len(raw) == 0 {
		return event, errors.New("empty integration event payload")
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return event, fmt.Errorf("invalid integration event payload: %w", err)
	}
	if err := event.Validate(); err != nil {
		return event, err
	}
	return event, nil
}

// EventHandler receives one dispatched integration event. Implementations must
// be safe for concurrent use and must not block on slow work: hand the work to a
// queue and return.
type EventHandler func(ctx context.Context, event IntegrationEvent) error

// DispatchIntegrationEvent validates an event and hands it to every registered
// handler (see IntegrationEventHandlers). With no handler registered it is a
// no-op, so an emitter never fails just because no consumer is compiled in. The
// errors of all handlers are joined.
func DispatchIntegrationEvent(ctx context.Context, event IntegrationEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	var errs []error
	kind := event.NormalizedKind()
	for _, ref := range IntegrationEventHandlers() {
		if ref.Sink == nil {
			continue
		}
		if ref.Kind != "" && ref.Kind != kind {
			continue
		}
		if err := ref.Sink(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", ref.IntegrationName, err))
		}
	}
	return errors.Join(errs...)
}
