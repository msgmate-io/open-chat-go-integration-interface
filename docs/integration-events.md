# Generic integration events

The public interface transports integration events in one generic, versioned
envelope and never interprets provider payloads:

```go
type IntegrationEvent struct {
	SchemaVersion int             `json:"schema_version,omitempty"`
	Kind          string          `json:"kind"`
	Provider      string          `json:"provider,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}
```

* `schema_version` defaults to `1`; a newer version is rejected instead of being
  misinterpreted, so the envelope can grow without breaking older receivers.
* `kind` is required and is the only thing the core dispatches on.
* `provider` names the emitter (for example `github`).
* `payload` is opaque to the core and is decoded only by the sink that cares
  about the kind.

## Sink

An integration declares `Definition.EventSink` (`EventHandler`) and is
discovered through `EventSinks()`. `DispatchIntegrationEvent` validates the
envelope, fan-outs to every declared sink and joins their errors; with no sink
compiled in it is a no-op, so the extension is backwards compatible.

Sinks must be concurrency-safe and must not block the emitting provider: hand
the work to a queue and return.

## What belongs to an integration, not here

Domain payload schemas (a CI failure, a deployment, a review verdict), the HTTP
ingress route that accepts them (`Definition.RouteRegistrar`), the credentials
that gate them and the routing/dispatch behaviour are owned by the
emitting/consuming integration. Nothing in this module knows about repositories,
pull requests, branches, commits, fix sessions or development leads.

## Route contributions

Integrations that need an authenticated ingress endpoint register it in
`Definition.RouteRegistrar`, which the core calls with the private v1 mux and
the root mux at `backend/integrations.RegisterRoutes`:

```go
def.RouteRegistrar = func(v1Private *http.ServeMux, root *http.ServeMux) {
	v1Private.Handle("/api/v1/my-integration/event", myHandler())
}
```

The token, the payload limit and the payload schema stay inside the
integration; the core only mounts the route.
