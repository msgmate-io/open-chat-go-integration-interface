package integrationinterface

import (
	"context"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// This file extends the stable Host surface with the generic capabilities the
// remaining coded integrations need beyond config/logging/HTTP: external chat
// backends, workspace leases, agent web tools, access-token minting and
// tool-init access. None of these are integration-specific; they are platform
// services the core owns and exposes to compiled-in integrations so the
// integrations stop importing backend/... packages directly.

// ---------------------------------------------------------------------------
// External chat backends
// ---------------------------------------------------------------------------

// ChatBackendKey is the shared-config key that routes bot reply processing to
// an external chat backend (eg "opencode"). It mirrors the core's
// chatstate.ChatBackendKey.
const ChatBackendKey = "chat_backend"

// ChatMessage is the stable, integration-facing view of the incoming chat
// message a chat backend generates a reply for.
type ChatMessage struct {
	ChatUUID   string
	SenderUUID string
	Text       string
	Reasoning  []string
	MetaData   *map[string]interface{}
}

// BotUser is the identity of the bot a chat backend replies as. It carries the
// UUID (used to build an API client) on top of the identity-only User surface.
type BotUser interface {
	ID() uint
	UUID() string
}

// MessageWriter publishes live websocket frames for a chat backend request. It
// replaces the core's *websocket.WebSocketHandler: the core hands the backend a
// writer bound to the request's live handler and the backend only composes and
// sends frames.
type MessageWriter interface {
	// SendMessage fans a raw frame out to a user and the frame's chat channel.
	SendMessage(receiverUUID string, frame []byte)
	// StartPartialMessage/EndPartialMessage/NewPartialMessage build the
	// corresponding partial-message frames.
	StartPartialMessage(chatUUID, senderUUID, sessionID string) []byte
	EndPartialMessage(chatUUID, senderUUID, sessionID string) []byte
	NewPartialMessage(chatUUID, senderUUID, sessionID string, seq int64, text string, reasoning []string, metaData *map[string]interface{}) []byte
}

// ChatBackendRequest is the stable request passed to a registered external chat
// backend instead of the core's msgmate.ChatBackendRequest.
type ChatBackendRequest struct {
	DB       *gorm.DB
	APIHost  string
	BotUser  BotUser
	Messages MessageWriter
	Message  ChatMessage
	Config   map[string]interface{}
}

// ChatBackendFunc generates a bot reply for a chat using an external backend.
type ChatBackendFunc func(ctx context.Context, req ChatBackendRequest) error

// BackendState is the live activity level an external chat backend reports for
// a chat.
type BackendState string

const (
	BackendStateIdle    BackendState = "idle"
	BackendStateRunning BackendState = "running"
)

// BackendStateFunc reports the live activity of an external chat backend for a
// single chat. ok is false when the backend has no state information.
type BackendStateFunc func(chatUUID string) (BackendState, bool)

// BackendInterruptFunc requests cancellation of in-flight work owned by an
// external chat backend for a chat. It returns true when the backend owned and
// cancelled live work.
type BackendInterruptFunc func(chatUUID string) bool

// ChatBackendSpec declares an integration's external chat backend to the core.
// Integrations register it from their package init (no Host required); the core
// drains the registry after installing the Host adapter and wires the declared
// pieces into its own chat-backend/chat-state registries. Any field may be nil:
// a backend that only contributes live state/interrupt handling leaves Backend
// unset, and vice versa.
type ChatBackendSpec struct {
	// Name maps to the chat shared-config "chat_backend" value.
	Name             string
	Backend          ChatBackendFunc
	StateProvider    BackendStateFunc
	InterruptHandler BackendInterruptFunc
}

var (
	chatSpecMu sync.RWMutex
	chatSpecs  = map[string]ChatBackendSpec{}
)

// RegisterChatBackend records a chat backend spec. It is safe to call from an
// integration's package init: it does not touch the Host.
func RegisterChatBackend(spec ChatBackendSpec) {
	name := strings.ToLower(strings.TrimSpace(spec.Name))
	if name == "" {
		return
	}
	spec.Name = name
	chatSpecMu.Lock()
	chatSpecs[name] = spec
	chatSpecMu.Unlock()
}

// ChatBackends returns the registered chat backend specs, sorted by name.
func ChatBackends() []ChatBackendSpec {
	chatSpecMu.RLock()
	defer chatSpecMu.RUnlock()
	out := make([]ChatBackendSpec, 0, len(chatSpecs))
	for _, spec := range chatSpecs {
		out = append(out, spec)
	}
	// stable order
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Name > out[j].Name; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// Chat is the request-time accessor for the core's external chat backend
// lifecycle. Registration goes through RegisterChatBackend (init-time); this
// surface is for runtime queries (live generation state).
type Chat interface {
	// HasActiveGeneration reports whether a native generation is in flight for
	// the chat.
	HasActiveGeneration(chatUUID string) bool
	// CancelActiveGeneration aborts the chat's in-flight native generation and
	// reports whether one was cancelled.
	CancelActiveGeneration(chatUUID string) bool
}

// ---------------------------------------------------------------------------
// Workspace leases
// ---------------------------------------------------------------------------

// Workspace lease kinds.
const (
	WorkspaceKindWorktree = "worktree"
	WorkspaceKindShared   = "shared"
	WorkspaceKindSandbox  = "sandbox"
)

// WorkspaceLeaseRequest describes a workspace-lease acquisition/refresh.
type WorkspaceLeaseRequest struct {
	Key           string
	OwnerChatUUID string
	OwnerUserID   uint
	OwnerLabel    string
	Kind          string
	Branch        string
	Path          string
}

// Workspace is the shared, advisory workspace-lease registry used by the coding
// integrations to avoid concurrent interactions clobbering the same checkout.
// Acquire is best-effort: it never fails on contention, it only refreshes the
// caller's lease.
type Workspace interface {
	// CanonicalKey normalizes a workspace directory into a stable lease key.
	CanonicalKey(path string) string
	// Acquire records (or refreshes) a lease on req.Key using db.
	Acquire(db *gorm.DB, req WorkspaceLeaseRequest) error
	// Status returns a JSON-friendly busy/owner description of a workspace
	// relative to the asking chat using db.
	Status(db *gorm.DB, key, excludeChatUUID string) map[string]interface{}
}

// ---------------------------------------------------------------------------
// Agent web tools
// ---------------------------------------------------------------------------

// WebFetchResult is a stable, agent-facing fetched document.
type WebFetchResult struct {
	URL         string
	Status      int
	ContentType string
	Title       string
	Content     string
	Truncated   bool
}

// WebSearchResult is a stable, agent-facing web search hit.
type WebSearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// Web exposes the core's shared agent-facing web helpers (readable fetch, keyless
// search) so integrations reuse one implementation instead of importing the core.
type Web interface {
	Fetch(ctx context.Context, rawURL string, maxBytes int) (WebFetchResult, error)
	Search(ctx context.Context, query string, limit int, site string) ([]WebSearchResult, error)
	DefaultSearchLimit() int
}

// ---------------------------------------------------------------------------
// Access tokens
// ---------------------------------------------------------------------------

// Token audiences and scopes integrations may mint.
const (
	// AudienceMCPInteraction marks the long-lived, default-deny credential the
	// coding integrations mint for the built-in Open-Chat MCP server.
	AudienceMCPInteraction = "mcp-interaction"
	// AudienceCoderTUI marks the short-lived, default-deny credential the coder
	// integration mints for the Open-Chat TUI forwarded into a coding sandbox.
	AudienceCoderTUI = "coder-tui"
	// ScopeInteractionsMCP grants read access to the read-only interaction
	// history MCP server.
	ScopeInteractionsMCP = "interactions:mcp"
)

// Tokens exposes the core's access-token minting so integrations own their
// token lifecycle without importing backend/database.
type Tokens interface {
	// GenerateRaw returns a fresh raw credential, its prefix and its hash.
	GenerateRaw() (raw, prefix, hash string, err error)
	// MintScoped creates a short-lived, audience-bound token for a user and
	// returns the raw credential exactly once. A non-positive ttl creates a
	// non-expiring token.
	MintScoped(db *gorm.DB, userID uint, name, audience string, scopes []string, ttl time.Duration, parentTokenID uint) (raw string, err error)
	// PruneMismatched removes a user's tokens stored under name whose audience
	// does not match audience, or which have expired. It backs the "mint once,
	// cache, replace on drift" flow.
	PruneMismatched(db *gorm.DB, userID uint, name, audience string) error
}

// ---------------------------------------------------------------------------
// Tool init + MCP tools
// ---------------------------------------------------------------------------

// Reserved tool-init keys shared between the core and integrations.
const (
	// ToolRuntimeContextKey is the private init-map key through which the core
	// injects the executing context. Integrations read it to honour cancellation
	// and deadlines.
	ToolRuntimeContextKey = "_ctx"
	// TrustedToolRuntimeKey is the reserved tool-init key under which the core
	// injects trusted caller context (eg the executing bot identity).
	TrustedToolRuntimeKey = "_runtime"
)

// Tools exposes the core tool-loop helpers an integration needs: MCP server
// discovery and persisted tool-init lookup.
type Tools interface {
	// DiscoverMCPTools lists the tools an MCP server exposes from its resolved
	// config/auth.
	DiscoverMCPTools(config map[string]interface{}, auth map[string]interface{}) ([]map[string]interface{}, error)
	// GetToolInitData returns the persisted tool-init payload for a chat and
	// tool, or nil when none is stored.
	GetToolInitData(db *gorm.DB, chatID uint, toolName string) (map[string]interface{}, error)
}
