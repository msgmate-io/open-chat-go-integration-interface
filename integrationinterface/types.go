package integrationinterface

import (
	"context"
	"io/fs"
	"net/http"
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

type Definition struct {
	Name           string
	ReadmeMarkdown string
	APIRoutes      []string
	APIRouteDocs   []APIRouteDoc
	FrontendRoutes []FrontendRoute
	FrontendPages  []FrontendPage
	FrontendAssets fs.FS
	ModelProviders []func() []interface{}
	RouteRegistrar func(v1Private *http.ServeMux, root *http.ServeMux)
	Functions      map[string]Function
}
