package integrationinterface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// This file defines the CI-error relay contract (ci#607): a first-class,
// versioned, secret-free event an external CI system hands Open-Chat, plus the
// sink an integration (agent chooser / development lead) implements to consume
// it.
//
// The CI side only emits the payload; it never dispatches or routes. The core
// fans the event out to the registered sinks and the receiving integration
// resolves the target repo to its development-lead interaction plus the
// candidate agent interactions. Integrations that do not implement the
// capability are simply absent from CIFailureEventSinks, so the extension is
// backwards compatible.

const (
	// CIFailureSchemaVersion is the highest relay schema version this module
	// understands. An event carrying a newer version must be rejected instead of
	// being guessed at.
	CIFailureSchemaVersion = 1
	// CIFailureEventKind is the canonical relay event kind.
	CIFailureEventKind = "ci_failure"
	// CIFailureEventAliasKind is the accepted alias of the event kind.
	CIFailureEventAliasKind = "ci_error"
	// CIFailureTargetPR, CIFailureTargetBranch and CIFailureTargetCommit name
	// the most specific target a failure was attributed to (see TargetKind).
	CIFailureTargetPR     = "pr"
	CIFailureTargetBranch = "branch"
	CIFailureTargetCommit = "commit"
)

// CIFailureEvent is the small, versioned, secret-free payload a CI system relays
// through an integration. Only Repo is mandatory; the fix session is keyed by
// the most specific target available (pull request, else branch, else commit).
type CIFailureEvent struct {
	SchemaVersion int      `json:"schema_version,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	Provider      string   `json:"provider,omitempty"`
	Repo          string   `json:"repo"`
	PRNumber      int      `json:"pr_number,omitempty"`
	Branch        string   `json:"branch,omitempty"`
	Sha           string   `json:"sha,omitempty"`
	RunURL        string   `json:"run_url,omitempty"`
	FailingJobs   []string `json:"failing_jobs,omitempty"`
	// FailureLogs carries a truncated, printable excerpt of the failed job
	// logs. Emitters must keep it small (kilobytes) and free of environment
	// dumps, tokens and other secret material.
	FailureLogs string `json:"failure_logs,omitempty"`
	Attempt     int    `json:"attempt,omitempty"`
	Severity    string `json:"severity,omitempty"`
	OccurredAt  string `json:"occurred_at,omitempty"`
}

// Validate rejects an event that must not be relayed: no repo, a schema version
// this module cannot understand, an unknown kind or impossible counters.
func (e CIFailureEvent) Validate() error {
	if normalizeCIRepo(e.Repo) == "" {
		return errors.New("ci_failure event is missing repo")
	}
	if e.SchemaVersion < 0 {
		return fmt.Errorf("invalid ci_failure schema_version %d", e.SchemaVersion)
	}
	if e.SchemaVersion > CIFailureSchemaVersion {
		return fmt.Errorf("unsupported ci_failure schema_version %d (max %d)", e.SchemaVersion, CIFailureSchemaVersion)
	}
	kind := strings.ToLower(strings.TrimSpace(e.Kind))
	if kind != "" && kind != CIFailureEventKind && kind != CIFailureEventAliasKind {
		return fmt.Errorf("unsupported ci_failure kind %q", e.Kind)
	}
	if e.PRNumber < 0 {
		return fmt.Errorf("invalid ci_failure pr_number %d", e.PRNumber)
	}
	if e.Attempt < 0 {
		return fmt.Errorf("invalid ci_failure attempt %d", e.Attempt)
	}
	return nil
}

// ParseCIFailureEvent decodes and validates a relayed CI-failure payload.
func ParseCIFailureEvent(raw []byte) (CIFailureEvent, error) {
	var event CIFailureEvent
	if len(raw) == 0 {
		return event, errors.New("empty ci_failure payload")
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return event, fmt.Errorf("invalid ci_failure payload: %w", err)
	}
	if err := event.Validate(); err != nil {
		return event, err
	}
	return event, nil
}

// TargetKind reports which target the failure was attributed to.
func (e CIFailureEvent) TargetKind() string {
	switch {
	case e.PRNumber > 0:
		return CIFailureTargetPR
	case normalizeCIBranch(e.Branch) != "":
		return CIFailureTargetBranch
	default:
		return CIFailureTargetCommit
	}
}

// FixSessionKey is the dedupe/attempt-cap identity of a failure: one fix session
// per repo+PR, else repo+branch, else repo+commit.
func (e CIFailureEvent) FixSessionKey() string {
	repo := normalizeCIRepo(e.Repo)
	switch e.TargetKind() {
	case CIFailureTargetPR:
		return fmt.Sprintf("%s#pr-%d", repo, e.PRNumber)
	case CIFailureTargetBranch:
		return fmt.Sprintf("%s#branch-%s", repo, normalizeCIBranch(e.Branch))
	default:
		return fmt.Sprintf("%s#sha-%s", repo, strings.ToLower(strings.TrimSpace(e.Sha)))
	}
}

// CIFailureEventFunc receives one relayed CI failure. Implementations must be
// safe for concurrent use and must not block on slow work: the emitting CI run
// waits on the relay, so hand the work to a queue and return.
type CIFailureEventFunc func(ctx context.Context, event CIFailureEvent) error

// DispatchCIFailureEvent validates and hands a relayed CI-failure event to every
// registered sink (see CIFailureEventSinks). With no sink registered it is a
// no-op, so a relay never fails just because no consumer is compiled in. The
// errors of all sinks are joined.
func DispatchCIFailureEvent(ctx context.Context, event CIFailureEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	var errs []error
	for _, ref := range CIFailureEventSinks() {
		if ref.Sink == nil {
			continue
		}
		if err := ref.Sink(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", ref.IntegrationName, err))
		}
	}
	return errors.Join(errs...)
}

// normalizeCIRepo canonicalizes a repository identity (owner/name) so relay
// events match interaction tags regardless of the URL form used by the emitter.
func normalizeCIRepo(repo string) string {
	v := strings.ToLower(strings.TrimSpace(repo))
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "git@github.com:", "github.com/"} {
		v = strings.TrimPrefix(v, prefix)
	}
	v = strings.TrimSuffix(v, ".git")
	return strings.Trim(v, "/")
}

func normalizeCIBranch(branch string) string {
	v := strings.TrimSpace(branch)
	v = strings.TrimPrefix(v, "refs/heads/")
	v = strings.TrimPrefix(v, "refs/pull/")
	return v
}

// IsZero reports whether the event carries no relay payload at all (no repo).
func (e CIFailureEvent) IsZero() bool {
	return normalizeCIRepo(e.Repo) == ""
}
