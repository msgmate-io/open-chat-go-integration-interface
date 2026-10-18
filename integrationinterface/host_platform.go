package integrationinterface

// This file extends the stable Host surface with the generic *platform*
// services the chat-backend integrations need beyond config reads, logging and
// request-scoped storage: writable runtime config and local-account
// provisioning. Neither is integration-specific; both are services the core
// owns. They are the last pieces that kept slack/matrix/signal/git importing
// `backend/runtimecfg` (writes) and `backend/server/util` (CreateUser).

// ---------------------------------------------------------------------------
// Runtime config writes
// ---------------------------------------------------------------------------

// ConfigWriter is the write side of the resolved runtime configuration. Reads
// live on Config (HostVersion 1). The chat-backend integrations also *write*
// values (the bridge host, the account-management email-verification flag),
// which previously required importing `backend/runtimecfg` directly.
type ConfigWriter interface {
	// SetValue inserts or replaces a single resolved value under its env key.
	// sensitive marks the value as sensitive so Config never reveals it to a
	// non-admin reader.
	SetValue(key, value string, sensitive bool)
	// DeleteValue removes a resolved value. Deleting an absent key is a no-op.
	DeleteValue(key string)
	// PublicBaseURL returns the trimmed PUBLIC_BASE_URL, or "" when unset. It is
	// the stable counterpart of the core's runtimecfg.PublicBaseURL helper.
	PublicBaseURL() string
}

// ---------------------------------------------------------------------------
// Local accounts
// ---------------------------------------------------------------------------

// UsersService creates and resolves local Open-Chat accounts. Integrations that
// provision a bridge/service account (Slack's bridge user, the matrix/signal
// bots) previously called the core's server/util.CreateUser directly.
//
// Create takes the storage handle (Storage.Handle()) rather than a concrete ORM
// type so this module stays ORM free: the core adapter type-asserts it.
type UsersService interface {
	// Create creates (or returns) a local user identified by username. It is
	// idempotent: when a user with that username or email already exists it is
	// returned unchanged and no write happens. A non-empty password is hashed;
	// an empty password creates an account with no usable password. The returned
	// User carries the resolved ID.
	Create(db interface{}, username, password string, isAdmin bool) (User, error)
	// ByUsername resolves an existing user by username or email. ok is false
	// when no such account exists.
	ByUsername(db interface{}, username string) (User, bool, error)
	// SetAutomated marks (or clears) the account's IsAutomated flag.
	SetAutomated(db interface{}, userID uint, automated bool) error
}
