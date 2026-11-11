package registry

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"example.com/localmap/internal/cache"
	"example.com/localmap/internal/config"
)

// Mux is the subset of http.ServeMux that plugins use to register routes.
type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// GeoJSONSource is a MapLibre GeoJSON data source.
type GeoJSONSource struct {
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}

// MapLayer is a MapLibre layer definition.
type MapLayer struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Source string         `json:"source"`
	Filter []any          `json:"filter,omitempty"`
	Layout map[string]any `json:"layout,omitempty"`
	Paint  map[string]any `json:"paint,omitempty"`
}

// MapOverlays holds GeoJSON sources and layers collected from plugins.
type MapOverlays struct {
	Sources []GeoJSONSource
	Layers  []MapLayer
}

// PluginContext provides context and utilities to plugins during registration.
type PluginContext struct {
	// Ctx is a context that signals when the server is shutting down.
	Ctx context.Context

	// Mux is the HTTP router. Routes registered here will be served under /{plugin_name}/path
	Mux Mux

	// CacheDir is the plugin-specific cache directory.
	CacheDir string

	// Config is the global server configuration.
	Config config.Config

	// Env contains environment variables filtered to LM_{PLUGIN_NAME}_* prefix.
	Env map[string]string

	// Cache is a cache instance for this plugin.
	Cache *cache.Cache

	// Logger is a structured logger pre-configured with the plugin name.
	Logger *slog.Logger

	EnabledPlugins func() []Plugin
}

// Plugin is a component that can register routes and contribute map data.
type Plugin interface {
	// Info returns metadata about the plugin including its documentation and configurable options.
	Info() *PluginInfo

	// Register is called during server initialization. The plugin should set up routes on
	// ctx.Mux and listen to ctx.Ctx for shutdown signals. If it returns an error, the
	// server will not start.
	Register(ctx *PluginContext) (enabled bool, err error)

	// AddMapLayers collects map layers and sources to be rendered. The plugin appends
	// its sources and layers to overlays.
	AddMapLayers(opts interface{}, overlays *MapOverlays) error
}

// PluginInfo contains metadata about a plugin.
type PluginInfo struct {
	Name        string
	Description string
	Options     []PluginOpt
}

// PluginOpt describes a configurable option for a plugin.
type PluginOpt struct {
	Name    string
	Usage   string
	Default string
}
