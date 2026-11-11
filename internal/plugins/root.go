package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"example.com/localmap/internal/cache"
	"example.com/localmap/internal/config"
	plugintypes "example.com/localmap/internal/plugins/types"

	// Imports for plugins.
	"example.com/localmap/internal/plugins/aiscatcher"
	"example.com/localmap/internal/plugins/cachedmap"
	"example.com/localmap/internal/plugins/geojson"
	"example.com/localmap/internal/plugins/mapper"
)

var allPlugins = []plugintypes.Plugin{
	&geojson.GeoJSONPlugin{},
	&aiscatcher.AISCatcherPlugin{},
	&mapper.MapPlugin{},
	&cachedmap.CachedMapPlugin{},
}

var enabled []plugintypes.Plugin

func Enabled() []plugintypes.Plugin {
	return enabled
}

// trackingMux wraps http.ServeMux and records each pattern as it is registered.
type trackingMux struct {
	mux    *http.ServeMux
	prefix string
	routes *[]string
}

func (t *trackingMux) Handle(pattern string, handler http.Handler) {
	*t.routes = append(*t.routes, t.prefix+pattern)
	t.mux.Handle(pattern, handler)
}

func (t *trackingMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	*t.routes = append(*t.routes, t.prefix+pattern)
	t.mux.HandleFunc(pattern, handler)
}

// Child creates a sub-mux rooted at pattern. Requests are prefix-stripped
// before reaching the child, so plugins register paths relative to their root.
func (t *trackingMux) Child(pattern string) *trackingMux {
	pattern = "/" + strings.Trim(pattern, "/")
	child := &trackingMux{
		mux:    http.NewServeMux(),
		prefix: strings.TrimSuffix(t.prefix, "/") + pattern,
		routes: t.routes,
	}
	t.mux.Handle(pattern+"/", http.StripPrefix(pattern, child.mux))
	return child
}

// InitializePlugins initializes all registered plugins and returns the full
// set of routes registered across all plugin muxes.
func InitializePlugins(ctx context.Context, cfg config.Config, baseCache string, mux *http.ServeMux) ([]string, error) {
	enabled = nil
	root := &trackingMux{mux: mux, prefix: "", routes: &[]string{}}

	for _, plugin := range allPlugins {
		pluginInfo := plugin.Info()
		name := pluginInfo.Name
		cacheDir := filepath.Join(baseCache, name)

		pluginCtx := &plugintypes.PluginContext{
			Ctx:            ctx,
			Mux:            root.Child(name),
			CacheDir:       cacheDir,
			Config:         cfg,
			Env:            filterEnv(name),
			Cache:          cache.New(cacheDir, 5*60*60*time.Second), // 5 hours
			Logger:         slog.With("plugin", name),
			EnabledPlugins: Enabled,
		}

		enable, err := plugin.Register(pluginCtx)
		if err != nil {
			return nil, fmt.Errorf("plugin %s failed to register: %w", name, err)
		}
		if enable {
			enabled = append(enabled, plugin)
		}
	}

	routes := *root.routes
	sort.Strings(routes)
	return routes, nil
}

// filterEnv returns environment variables matching LM_{PLUGIN_NAME}_* pattern.
func filterEnv(pluginName string) map[string]string {
	prefix := fmt.Sprintf("LM_%s_", strings.ToUpper(pluginName))
	result := make(map[string]string)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	}
	return result
}
