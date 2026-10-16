package integrationinterface

import (
	"net/http"
	"sync"
)

// HostVersion is the version of the Host capability contract. It is bumped
// whenever the shape of Host (or one of its sub-interfaces) changes; the core
// adapter reports the version it implements via Host.Version so integrations
// can gate behaviour on it.
//
// Version history:
//   - 1: initial Host/Config/Logger/Storage/User/HTTP surface.
//   - 2: HTTP gains DB, the request-scoped storage handle without requiring an
//     authenticated user (public/anonymous endpoints).
//   - 3: Storage gains the process-wide handle (Global); Chat, Workspace, Web,
//     Tokens and Tools capability surfaces were added.
const HostVersion = 3

// Value is a single resolved runtime configuration entry.
type Value struct {
	Value     string
	Sensitive bool
}

// Config exposes the resolved runtime configuration layer (the `backend/runtimecfg`
// values: OCI_* overrides, open-chat.json `integrations.<name>.*` and env).
type Config interface {
	// All returns a copy of every resolved runtime value keyed by its env key.
	All() map[string]Value
	// Get returns a single value by env key.
	Get(key string) (Value, bool)
}

// Logger is the host logging sink. Integrations log through the host instead of
// the standard library so the core can route, format and scope integration logs.
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// Storage is an opaque handle onto the host persistence layer. The interface
// module deliberately does not depend on the ORM: Handle returns the concrete
// persistence handle (today a *gorm.DB) which the integration passes to the ORM
// helpers it already uses. This keeps this module's dependency surface small and
// the interface stable while a richer data-access API is designed.
type Storage interface {
	// Handle returns the underlying persistence handle. The concrete type is
	// owned by the core adapter and documented there; callers type-assert before
	// use.
	Handle() interface{}
	// Global returns the process-wide persistence handle, for code that runs
	// outside an HTTP request (queue tasks, background workers, integration
	// hooks). It is the stable counterpart of the core's global DB accessor.
	Global() interface{}
}

// User is the authenticated principal attached to a request. It intentionally
// mirrors only the identity fields integrations need, so the core's user model
// stays an implementation detail.
type User interface {
	ID() uint
	Username() string
	Email() string
	IsAdmin() bool
}

// HTTP exposes request-scoped access to the host services. It does not carry an
// http.ServeMux: route registration stays on Definition, so this surface only
// covers capability lookups.
type HTTP interface {
	// DB returns the request-scoped storage handle without requiring an
	// authenticated user. It is the stable counterpart of the core's GetDB
	// helper for public/anonymous endpoints.
	DB(r *http.Request) (Storage, error)
	// DBAndUser returns the request-scoped storage handle and the authenticated
	// user. It is the stable counterpart of the core's GetDBAndUser helper.
	DBAndUser(r *http.Request) (Storage, User, error)
	// OptionalUser returns the authenticated user, or nil for anonymous
	// requests.
	OptionalUser(r *http.Request) User
}

// Host is the stable, versioned capability surface the core exposes to
// compiled-in integrations. It replaces direct imports of the core's
// runtime-config, database and server-util packages. The surface is minimal on
// purpose and does not expose fs.FS, http.ServeMux or gorm.DB.
type Host interface {
	// Version reports the HostVersion implemented by the adapter.
	Version() int
	Config() Config
	Logger() Logger
	HTTP() HTTP
	// Storage returns the process-wide persistence handle.
	Storage() Storage
	// Chat exposes the external chat backend lifecycle for runtime queries.
	Chat() Chat
	// Workspace exposes the shared workspace-lease registry.
	Workspace() Workspace
	// Web exposes the core's shared agent-facing web helpers.
	Web() Web
	// Tokens exposes access-token minting.
	Tokens() Tokens
	// Tools exposes the core tool-loop helpers (MCP discovery, tool init).
	Tools() Tools
}

var (
	hostMu sync.RWMutex
	host   Host
)

// SetHost installs the process-wide Host implementation. It is called once by
// the core adapter at startup. Passing nil clears the host (used by tests).
func SetHost(h Host) {
	hostMu.Lock()
	host = h
	hostMu.Unlock()
}

// GetHost returns the installed Host implementation, or nil when none is
// installed (unit tests, standalone tooling, or an integration built outside the
// core server).
func GetHost() Host {
	hostMu.RLock()
	defer hostMu.RUnlock()
	return host
}

// RequireHost returns the installed Host or panics when it is missing. Use it at
// request or bootstrap time in compiled-in integrations where the core server is
// guaranteed to have installed the adapter.
func RequireHost() Host {
	h := GetHost()
	if h == nil {
		panic("integrationinterface: host is not installed")
	}
	return h
}
