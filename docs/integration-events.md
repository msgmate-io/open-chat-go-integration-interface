# Integration event contract

The base integration interface owns the *contract* for relaying an external
event into Open-Chat; it never owns dispatch or routing. The emitting side
publishes a small envelope, the core validates it and fans it out by `kind`, and
the integration that registered a handler for that `kind` interprets the opaque
payload.

The envelope is deliberately integration-neutral: it carries no CI, GitHub, PR,
branch, commit or development-lead vocabulary. Whatever a `kind` means is owned
end to end by the emitting side and the consuming integration.

## Envelope

`integrationinterface.IntegrationEvent` (JSON):

| field | required | meaning |
| --- | --- | --- |
| `schema_version` | no | envelope version, max `IntegrationEventSchemaVersion` (`1`); defaults to `1` |
| `kind` | **yes** | lower-cased event kind, namespaced by the owning integration, e.g. `myintegration.notice` |
| `provider` | no | free-form origin id, e.g. `github`, usable for routing/metrics |
| `payload` | no | opaque, kind-specific JSON document owned by producer + consuming integration |

Emitters must keep `payload` small and secret-free: never send environment
dumps, tokens, kubeconfigs or raw credentials; send a truncated excerpt instead.

## Validation and fan-out

- `ParseIntegrationEvent(raw)` decodes an envelope and validates it. A missing
  `kind` or a `schema_version` below `1` or above `IntegrationEventSchemaVersion`
  is rejected with a descriptive error; unknown *kinds* are not, because the core
  never interprets them.
- `DispatchIntegrationEvent(ctx, event)` hands a validated event to every
  registered handler and joins their errors into one (so one failing consumer
  cannot silently swallow another's failure). With no handler registered it is a
  no-op, so the extension is backwards compatible by construction.

## Handlers

An integration implements `integrationinterface.EventHandler`
(`func(ctx, event) error`) and registers it under its own integration name and
`kind` via `RegisterIntegrationEventHandler(...)`. Registrations are discovered
through `IntegrationEventHandlers()`, which mirrors the `Definitions()` lookup
pattern and is consulted per dispatch.

Handlers must be concurrency-safe and must not block the emitting side: hand the
work to a queue and return. A kind-specific schema is versioned with the
envelope's `schema_version`; bump `IntegrationEventSchemaVersion` only for
changes to the envelope itself.

## Example

```go
func init() {
	integrationinterface.RegisterIntegrationEventHandler("myintegration", "myintegration.notice",
		func(ctx context.Context, event integrationinterface.IntegrationEvent) error {
			// The payload schema belongs to this integration, not to the core.
			return queue.Publish(ctx, event.Payload)
		})
}
```

## Core requirements

The core only needs to (a) expose a trusted ingress the owning integration
contributes, (b) call `ParseIntegrationEvent` on the raw body, and (c) call
`DispatchIntegrationEvent`. It must not know any kind-specific field, and a build
with no consumer integration compiled in still compiles and simply drops events.
