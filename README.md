# go-integration-interface

Small Go SDK for registering Open Chat integrations.

Integrations can contribute:

- DB models/migrations
- API route registration
- frontend HTML routes under `/integrations/<integration_name>/...`
- static frontend HTML pages backed by integration-owned embedded assets
- callable integration functions

## Usage

```go
package myintegration

import (
	"context"
	"net/http"

	"github.com/msgmate-io/go-integration-interface/integrationinterface"
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
	})
}
```

Notes:

- Frontend routes are auto-registered by the backend for all compiled integrations.
- Frontend route paths must be under `/integrations/<integration_name>`.
- Frontend routes must not use `/api` prefixes.
- Frontend pages require `Definition.FrontendAssets` and `AssetPath` points to an HTML file in that filesystem.
