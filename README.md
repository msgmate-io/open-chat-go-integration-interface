# go-integration-interface

Small Go SDK for registering Open Chat integrations.

Integrations can contribute:

- DB models/migrations
- API route registration
- frontend HTML routes under `/integrations/<integration_name>/...`
- static frontend HTML pages backed by integration-owned embedded assets
- tool definitions (using go-tool-interface)
- callable integration functions
- default bot bootstrap configs (applied at server startup)

## Usage

```go
package myintegration

import (
	"context"
	"net/http"

	"github.com/msgmate-io/go-integration-interface/integrationinterface"
	"github.com/msgmate-io/go-tool-interface/toolinterface"
)

func init() {
	integrationinterface.MustRegister(integrationinterface.Definition{
		Name: "my_integration",
		ModelProviders: []func() []interface{}{
			func() []interface{} { return []interface{}{} },
		},
		RouteRegistrar: func(v1Private *http.ServeMux, root *http.ServeMux) {
			_ = root
			v1Private.HandleFunc("GET /integrations/my-integration/health", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
		},
		FrontendRoutes: []integrationinterface.FrontendRoute{
			{
				Route:       "/integrations/my_integration",
				Public:      true,
				Description: "Public integration landing page",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte("<html><body><h1>My Integration</h1></body></html>"))
				},
			},
		},
		FrontendPages: []integrationinterface.FrontendPage{
			{
				Route:       "/integrations/my_integration/servers",
				Public:      false,
				Description: "Bundled servers page",
				AssetPath:   "servers/index.html",
			},
		},
		FrontendAssets: myEmbeddedAssetsFS,
		Functions: map[string]integrationinterface.Function{
			"health": func(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
				_ = ctx
				_ = payload
				return map[string]interface{}{"ok": true}, nil
			},
		},
		ToolDefinitions: []toolinterface.Definition{
			{
				Name:        "my_integration_echo",
				Description: "Echo a message",
				InputType: struct {
					Message string `json:"message"`
				}{},
				RequiredParams: []string{"message"},
				Parameters: map[string]interface{}{
					"message": map[string]interface{}{"type": "string"},
				},
				Run: func(input interface{}, init map[string]interface{}) (string, error) {
					_ = init
					in := input.(struct {
						Message string `json:"message"`
					})
					return in.Message, nil
				},
			},
		},
		BotBootstrapConfigs: []integrationinterface.BotBootstrapConfig{
			{
				PrimaryOwner: "admin",
				Bot: integrationinterface.BotIdentityConfig{
					Username: "my-integration-bot",
					Password: "random",
					Name:     "my_integration_bot",
				},
				DefaultSharedConfig: map[string]interface{}{
					"tools": []string{"my_integration_echo"},
				},
			},
		},
	})
}
```

## Runtime env vars and settings UI

Integrations can declare `RuntimeEnvVars` to expose configuration through the
admin **Integration Settings** page. Keys must use the `OCI_` prefix.

```go
RuntimeEnvVars: []integrationinterface.RuntimeEnvVar{
    {
        Key:         "OCI_MY_INTEGRATION_API_KEY",
        Sensitive:   true,
        Description: "API key used to talk to the upstream service.",
        Label:       "API key",
        Type:        "secret",
        Required:    true,
        Group:       "Connection",
        Order:       10,
    },
    {
        Key:         "OCI_MY_INTEGRATION_ENABLED",
        Type:        "bool",
        Default:     "false",
        Group:       "Connection",
    },
},
RuntimeConfigAliases: []integrationinterface.RuntimeConfigAlias{
    {JSONKey: "api_key", EnvKey: "OCI_MY_INTEGRATION_API_KEY"},
},
```

Every metadata field other than `Key`/`Sensitive`/`Description` is optional and
backwards compatible. When metadata is omitted the settings UI infers it:

- `Sensitive` fields render as `secret`.
- Keys containing `ENABLE`/`ENABLED` (or whose current value parses as a bool)
  render as `bool`.
- Keys ending in `_JSON`, `_SPEC`, `_YAML`, `_PRIVATE_KEY`, `BOOTSTRAP` or
  `KUBECONFIG` (or multi-line values) render as `json`.
- Everything else renders as `string`.

Aliases map a JSON config key under `integrations.<name>.<json_key>` to a
declared env key, so settings writes land in the alias entry when present and
otherwise under `env.<ENVKEY>`.

Notes:

- Frontend routes are auto-registered by the backend for all compiled integrations.
- Frontend route paths must be under `/integrations/<integration_name>`.
- Frontend routes must not use `/api` prefixes.
- Frontend pages require `Definition.FrontendAssets` and `AssetPath` points to an HTML file in that filesystem.
- Tool names are global across all integrations; duplicate tool names fail registration.
- Integration bot bootstrap configs reuse the same schema as `open-chat.json`/`open-chat.yaml bootstrap.bots`.
- Integration bot configs are intended as defaults; user-provided bot bootstrap config should take precedence.
