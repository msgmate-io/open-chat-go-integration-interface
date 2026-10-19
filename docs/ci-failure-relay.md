# CI-failure relay contract (ci#607)

The base integration interface owns the *contract* for relaying an external CI
failure into Open-Chat; it never owns dispatch or routing. The CI side only
emits a payload, the core fans it out, and the receiving integration (agent
chooser / development lead) resolves what to do with it.

## Event

`integrationinterface.CIFailureEvent` (JSON, `schema_version: 1`):

| field | required | meaning |
| --- | --- | --- |
| `schema_version` | no | relay schema version, max `CIFailureSchemaVersion` |
| `kind` | no | `ci_failure` (alias `ci_error`) |
| `provider` | no | CI provider id, e.g. `github` |
| `repo` | **yes** | `owner/name` or any GitHub URL/git form |
| `pr_number` | no | pull request the failure was attributed to |
| `branch`, `sha` | no | fallback targets when there is no PR |
| `run_url` | no | link to the failing run |
| `failing_jobs` | no | job names |
| `failure_logs` | no | small (kilobyte) failed-step excerpt, secret-free |
| `attempt`, `severity`, `occurred_at` | no | relay bookkeeping |

The payload carries **no secrets**: emitters must send a truncated log excerpt
and never environment dumps, tokens or credentials.

## Sink

An integration implements `Definition.CIFailureEventSink` (`CIFailureEventFunc`)
and is discovered via `CIFailureEventSinks()`. `DispatchCIFailureEvent` validates
the event, fan-outs to every registered sink and joins their errors; with no sink
compiled in it is a no-op, so the extension is backwards compatible.

Sinks must be concurrency-safe and must not block the emitting CI run: hand the
work to a queue and return.

## Identity

`TargetKind()` picks the most specific target (PR, else branch, else commit) and
`FixSessionKey()` derives the dedupe / attempt-cap identity
(`owner/name#pr-<n>` / `#branch-<name>` / `#sha-<sha>`). Repos are normalized so
the emitter's URL form matches interaction `repo:` tags.
