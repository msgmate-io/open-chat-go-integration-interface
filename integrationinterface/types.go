package integrationinterface

import (
	"context"
	"net/http"
)

type Function func(ctx context.Context, payload map[string]interface{}) (interface{}, error)

type Definition struct {
	Name           string
	APIRoutes      []string
	ModelProviders []func() []interface{}
	RouteRegistrar func(v1Private *http.ServeMux, root *http.ServeMux)
	Functions      map[string]Function
}
