package integrationinterface

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/msgmate-io/go-tool-interface/toolinterface"
	"gorm.io/gorm"
)

type Function func(ctx context.Context, payload map[string]interface{}) (interface{}, error)

type FrontendRoute struct {
	Route       string
	Public      bool
	Description string
	Handler     http.HandlerFunc
}

type FrontendPage struct {
	Route       string
	Public      bool
	Description string
	AssetPath   string
}

type APIRouteParameter struct {
	Name        string
	In          string
	Type        string
	Required    bool
	Description string
}

type APIRouteDoc struct {
	Route        string
	Summary      string
	Description  string
	RequiredAuth []string
	Parameters   []APIRouteParameter
}

type RuntimeEnvVar struct {
	Key         string
	Sensitive   bool
	Description string
}

type RuntimeConfigAlias struct {
	JSONKey     string
	EnvKey      string
	Description string
}

type Migration struct {
	Name string
	Run  func(db *gorm.DB) error
}

type BotIdentityConfig struct {
	Username    string `json:"username"`
	Email       string `json:"email,omitempty"`
	Password    string `json:"password"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsPublic    *bool  `json:"is_public,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

type BotBootstrapConfig struct {
	PrimaryOwner            string                 `json:"primary_owner"`
	AdditionalOwners        []string               `json:"additional_owners,omitempty"`
	Bot                     BotIdentityConfig      `json:"bot"`
	DefaultSharedConfig     map[string]interface{} `json:"default_shared_config"`
	AllowedModelBackends    []string               `json:"allowed_model_backends,omitempty"`
	InheritDefaultBotModels bool                   `json:"inherit_default_bot_models,omitempty"`
	OverwriteIfExists       bool                   `json:"overwrite_if_exists,omitempty"`
}

// BootstrapSources carries server startup inputs to an integration's optional
// Bootstrap hook. It exposes raw, integration-scoped data so each integration
// owns its own decoding instead of the core server hardcoding per-integration
// bootstrap wiring.
type BootstrapSources struct {
	// DB is the initialized database handle.
	DB *gorm.DB
	// AdminUsername is the bootstrap admin user the integration should grant
	// ownership to by default.
	AdminUsername string
	// Config is the `bootstrap.<name>` section of the resolved open-chat
	// config for this integration, if present.
	Config map[string]interface{}
	// Integrations is the full `integrations.<name>` config section, useful for
	// runtime env aliases declared by the integration.
	Integrations map[string]map[string]interface{}
	// Env looks up an environment variable (OCI_* runtime config overrides).
	Env func(key string) (string, bool)
}

// BootstrapFunc is invoked once at server startup after the database and admin
// user are ready. Returning an error aborts startup.
type BootstrapFunc func(ctx context.Context, sources BootstrapSources) error

type Definition struct {
	Name                 string
	AdminOnly            bool
	UserAccessible       bool
	ReadmeMarkdown       string
	APIRoutes            []string
	APIRouteDocs         []APIRouteDoc
	FrontendRoutes       []FrontendRoute
	FrontendPages        []FrontendPage
	FrontendAssets       fs.FS
	ModelProviders       []func() []interface{}
	RouteRegistrar       func(v1Private *http.ServeMux, root *http.ServeMux)
	Functions            map[string]Function
	SharedConfigDefaults func(current map[string]interface{}) map[string]interface{}
	ToolDefinitions      []toolinterface.Definition
	RuntimeEnvVars       []RuntimeEnvVar
	RuntimeConfigAliases []RuntimeConfigAlias
	Migrations           []Migration
	BotBootstrapConfigs  []BotBootstrapConfig
	Bootstrap            BootstrapFunc
}
