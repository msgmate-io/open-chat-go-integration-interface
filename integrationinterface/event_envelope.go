package integrationinterface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// This file defines the generic event surface shared by the core and the
// integrations: a versioned envelope whose payload stays opaque to the core,
// and a kind-keyed sink that an integration registers on its Definition.
// Nothing here is specific to an integration, a provider or an event kind.

// EventSchemaVersion is the highest envelope schema version this module
// understands. A payload advertising a newer version is rejected by the core
// instead of being misinterpreted; payload-level versions belong to the sink.
const EventSchemaVersion = 1

// IntegrationEvent is the versioned envelope an external system's event is
// delivered in. The core treats Payload as opaque bytes: only the integration
// that registered the sink for Kind interprets it.
type IntegrationEvent struct {
	SchemaVersion int             `json:"schema_version,omitempty"`
	Kind          string          `json:"kind"`
	Provider      string          `json:"provider,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

// NormalizeKind lower-cases and trims an event kind so registration and lookup
// agree on the key.
func NormalizeKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}

// KindKey is the normalised lookup key of the event kind.
func (e IntegrationEvent) KindKey() string { return NormalizeKind(e.Kind) }

// Validate rejects an envelope the core must not fan out: a missing kind or a
// schema version it cannot understand. Payload validation is the sink's job.
func (e IntegrationEvent) Validate() error {
	if NormalizeKind(e.Kind) == "" {
		return errors.New("integration event is missing kind")
	}
	if e.SchemaVersion < 0 {
		return fmt.Errorf("invalid integration event schema_version %d", e.SchemaVersion)
	}
	if e.SchemaVersion > EventSchemaVersion {
		return fmt.Errorf("unsupported integration event schema_version %d (max %d)", e.SchemaVersion, EventSchemaVersion)
	}
	return nil
}

// EventSinkFunc handles one event of the kind it was registered for. handled
// reports whether the sink processed the event; a kind without a registered
// sink is reported by the dispatcher as handled=false with a nil error. Sinks
// must be concurrency-safe and must not block the emitting caller.
type EventSinkFunc func(ctx context.Context, event IntegrationEvent) (handled bool, err error)

// EventSinkRef pairs an integration name and event kind with a registered sink.
type EventSinkRef struct {
	IntegrationName string
	Kind            string
	Sink            EventSinkFunc
}

// EventSinks returns the event sinks declared by registered integrations,
// ordered by integration name then kind. Compiled-out integrations contribute
// nothing, so the fan-out always mirrors the integration profile of the build.
func EventSinks() []EventSinkRef {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]EventSinkRef, 0)
	for _, name := range names {
		def := definitions[name]
		kinds := make([]string, 0, len(def.EventSinks))
		for kind := range def.EventSinks {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		for _, kind := range kinds {
			sink := def.EventSinks[kind]
			if sink == nil {
				continue
			}
			out = append(out, EventSinkRef{IntegrationName: name, Kind: kind, Sink: sink})
		}
	}
	return out
}

// EventSinkForKind returns the sink registered for an event kind. A kind is
// owned by exactly one integration; the first registration in order wins.
func EventSinkForKind(kind string) (EventSinkRef, bool) {
	key := NormalizeKind(kind)
	if key == "" {
		return EventSinkRef{}, false
	}
	for _, ref := range EventSinks() {
		if NormalizeKind(ref.Kind) == key {
			return ref, true
		}
	}
	return EventSinkRef{}, false
}

// DispatchEvent fans an event out to the sink registered for its kind. An
// unknown kind is not an error: it returns handled=false with a nil error, so a
// build without the owning integration keeps working and simply does not handle
// that kind.
func DispatchEvent(ctx context.Context, event IntegrationEvent) (handled bool, err error) {
	if err := event.Validate(); err != nil {
		return false, err
	}
	ref, ok := EventSinkForKind(event.Kind)
	if !ok {
		return false, nil
	}
	return ref.Sink(ctx, event)
}
