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
	// Label is the human readable field label. Falls back to Key when empty.
	Label string
	// Type hints the settings UI renderer. One of
	// "string"|"bool"|"number"|"select"|"json"|"secret". When empty the type is
	// inferred from Sensitive/Key/value.
	Type string
	// Default is shown as the placeholder/default value when the runtime value
	// is unset.
	Default string
	// Placeholder is shown inside empty inputs.
	Placeholder string
	// Options restricts the allowed values when Type is "select".
	Options []string
	// Required marks the field as mandatory in the settings UI.
	Required bool
	// Group is the form section the field belongs to.
	Group string
	// Order sorts fields within their group.
	Order int
	// Advanced collapses the field under an "Advanced" section.
	Advanced bool
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
}
