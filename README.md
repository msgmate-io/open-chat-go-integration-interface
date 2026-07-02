# go-integration-interface

Small Go SDK for registering Open Chat integrations.

Integrations can contribute:

- DB models/migrations
- API route registration
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
