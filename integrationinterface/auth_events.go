package integrationinterface

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// AuthEvent captures a single authentication attempt (successful or failed).
// It intentionally carries no secrets (no passwords, session tokens or one-time
// codes) so it is safe for integrations to persist or forward for audit and
// login-history purposes.
type AuthEvent struct {
	// DB is the request-scoped database handle so integration-owned sinks can
	// persist events without holding global state.
	DB *gorm.DB
	// UserID is set when the attempt resolved to a known account.
	UserID uint
	// Identifier is the submitted login identifier (email or username).
	Identifier string
	// Method is the authentication method used: password, two_factor,
	// recovery_code, google, impersonation, token or session.
	Method string
	// Success reports whether the attempt authenticated a session.
	Success bool
	// FailureReason is a short, non-sensitive reason when Success is false.
	FailureReason string
	// IP is the (proxy-aware) client address.
	IP string
	// UserAgent is the raw User-Agent header.
	UserAgent string
	// OccurredAt is the event timestamp in UTC.
	OccurredAt time.Time
}

// AuthEventFunc receives authentication events. Implementations must be safe for
// concurrent use, must not block for long, and must never change the
// authentication outcome. Errors are logged by the server and otherwise
// ignored.
type AuthEventFunc func(ctx context.Context, event AuthEvent) error
