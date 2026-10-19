package integrationinterface

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const ciFailureRelayTestPayload = `{
  "schema_version": 1,
  "kind": "ci_failure",
  "provider": "github",
  "repo": "https://github.com/msgmate-io/open-chat-go.git",
  "pr_number": 822,
  "branch": "refs/heads/ci-606-relay-payload-parity",
  "sha": "1f9c0ffee",
  "run_url": "https://github.com/msgmate-io/open-chat-go/actions/runs/1",
  "failing_jobs": ["test", "build"],
  "failure_logs": "--- FAIL: TestRelay (0.01s)\n    relay_test.go:42: boom\nFAIL",
  "attempt": 1,
  "severity": "error",
  "occurred_at": "2026-10-19T04:16:07Z"
}`

func ciTestUniqueName(prefix string, t *testing.T) string {
	return prefix + "-" + t.Name()
}

func TestParseCIFailureEventAcceptsRelayedPayload(t *testing.T) {
	event, err := ParseCIFailureEvent([]byte(ciFailureRelayTestPayload))
	if err != nil {
		t.Fatalf("ParseCIFailureEvent(relayed payload): %v", err)
	}
	if event.Repo != "https://github.com/msgmate-io/open-chat-go.git" {
		t.Fatalf("repo not decoded verbatim: %q", event.Repo)
	}
	if event.PRNumber != 822 || event.Attempt != 1 {
		t.Fatalf("unexpected target: %+v", event)
	}
	if len(event.FailingJobs) != 2 || event.FailingJobs[0] != "test" {
		t.Fatalf("failing jobs not decoded: %+v", event.FailingJobs)
	}
	if !strings.Contains(event.FailureLogs, "FAIL") {
		t.Fatalf("failure log excerpt lost: %q", event.FailureLogs)
	}
	if got, want := event.TargetKind(), CIFailureTargetPR; got != want {
		t.Fatalf("TargetKind = %q, want %q", got, want)
	}
	if got, want := event.FixSessionKey(), "msgmate-io/open-chat-go#pr-822"; got != want {
		t.Fatalf("FixSessionKey = %q, want %q", got, want)
	}
	if event.IsZero() {
		t.Fatal("parsed event must not be zero")
	}
}

func TestCIFailureEventValidation(t *testing.T) {
	cases := []struct {
		name    string
		event   CIFailureEvent
		wantErr string
	}{
		{name: "missing repo", event: CIFailureEvent{}, wantErr: "missing repo"},
		{name: "future schema", event: CIFailureEvent{Repo: "a/b", SchemaVersion: CIFailureSchemaVersion + 1}, wantErr: "unsupported"},
		{name: "negative schema", event: CIFailureEvent{Repo: "a/b", SchemaVersion: -1}, wantErr: "invalid"},
		{name: "unknown kind", event: CIFailureEvent{Repo: "a/b", Kind: "deploy_failure"}, wantErr: "unsupported"},
		{name: "negative pr", event: CIFailureEvent{Repo: "a/b", PRNumber: -1}, wantErr: "pr_number"},
		{name: "negative attempt", event: CIFailureEvent{Repo: "a/b", Attempt: -2}, wantErr: "attempt"},
		{name: "alias kind", event: CIFailureEvent{Repo: "a/b", Kind: CIFailureEventAliasKind}},
		{name: "repo normalizes", event: CIFailureEvent{Repo: "https://github.com/a/b.git"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.event.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
	if _, err := ParseCIFailureEvent(nil); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("ParseCIFailureEvent(nil) = %v, want empty-payload error", err)
	}
	if _, err := ParseCIFailureEvent([]byte("{")); err == nil || !strings.Contains(err.Error(), "invalid ci_failure payload") {
		t.Fatalf("ParseCIFailureEvent(broken) = %v, want decode error", err)
	}
}

func TestCIFailureEventFixSessionKeyPrefersMostSpecificTarget(t *testing.T) {
	base := CIFailureEvent{Repo: "msgmate-io/open-chat-go", Branch: "main", Sha: "ABC123"}
	if got, want := base.FixSessionKey(), "msgmate-io/open-chat-go#branch-main"; got != want {
		t.Fatalf("branch key = %q, want %q", got, want)
	}
	withPR := base
	withPR.PRNumber = 7
	if got, want := withPR.FixSessionKey(), "msgmate-io/open-chat-go#pr-7"; got != want {
		t.Fatalf("pr key = %q, want %q", got, want)
	}
	shaOnly := CIFailureEvent{Repo: "msgmate-io/open-chat-go", Sha: "ABC123"}
	if got, want := shaOnly.FixSessionKey(), "msgmate-io/open-chat-go#sha-abc123"; got != want {
		t.Fatalf("sha key = %q, want %q", got, want)
	}
	if got, want := shaOnly.TargetKind(), CIFailureTargetCommit; got != want {
		t.Fatalf("TargetKind = %q, want %q", got, want)
	}
}

func TestDispatchCIFailureEventReachesRegisteredSink(t *testing.T) {
	var got CIFailureEvent
	calls := 0
	MustRegister(Definition{
		Name: ciTestUniqueName("cirelaysink", t),
		CIFailureEventSink: func(_ context.Context, event CIFailureEvent) error {
			calls++
			got = event
			return nil
		},
	})
	event, err := ParseCIFailureEvent([]byte(ciFailureRelayTestPayload))
	if err != nil {
		t.Fatalf("ParseCIFailureEvent: %v", err)
	}
	if err := DispatchCIFailureEvent(context.Background(), event); err != nil {
		t.Fatalf("DispatchCIFailureEvent: %v", err)
	}
	if calls != 1 {
		t.Fatalf("sink called %d times, want 1", calls)
	}
	if got.FixSessionKey() != event.FixSessionKey() || got.FailureLogs != event.FailureLogs {
		t.Fatalf("sink received %+v, want %+v", got, event)
	}

	if err := DispatchCIFailureEvent(context.Background(), CIFailureEvent{}); err == nil {
		t.Fatal("DispatchCIFailureEvent must reject an invalid event")
	}
	if calls != 1 {
		t.Fatalf("sink must not be called for an invalid event, got %d calls", calls)
	}
}

func TestDispatchCIFailureEventPropagatesSinkError(t *testing.T) {
	boom := errors.New("boom")
	MustRegister(Definition{
		Name: ciTestUniqueName("cirelayfailsink", t),
		CIFailureEventSink: func(_ context.Context, _ CIFailureEvent) error {
			return boom
		},
	})
	event, err := ParseCIFailureEvent([]byte(ciFailureRelayTestPayload))
	if err != nil {
		t.Fatalf("ParseCIFailureEvent: %v", err)
	}
	err = DispatchCIFailureEvent(context.Background(), event)
	if !errors.Is(err, boom) {
		t.Fatalf("DispatchCIFailureEvent = %v, want wrapped boom", err)
	}
	if !strings.Contains(err.Error(), "cirelayfailsink-") {
		t.Fatalf("sink error is not attributed to its integration: %v", err)
	}
}

func TestCIFailureEventPayloadCarriesNoSecretMaterial(t *testing.T) {
	event, err := ParseCIFailureEvent([]byte(ciFailureRelayTestPayload))
	if err != nil {
		t.Fatalf("ParseCIFailureEvent: %v", err)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	low := strings.ToLower(string(encoded))
	for _, marker := range []string{"token", "secret", "password", "authorization", "ghs_", "private_key"} {
		if strings.Contains(low, marker) {
			t.Fatalf("relayed payload must stay secret-free, found %q in %s", marker, encoded)
		}
	}
}
