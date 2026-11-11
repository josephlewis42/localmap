package mapper

import (
	"log"
	"net"
	"net/http"
	"strconv"

	plugintypes "example.com/localmap/internal/plugins/types"
)

// MapPlugin is the primary plugin that renders maps.
type MapPlugin struct {
	ctx *plugintypes.PluginContext
}

func (MapPlugin) Info() *plugintypes.PluginInfo {
	return &plugintypes.PluginInfo{
		Name:        "map",
		Description: "Primary map rendering plugin.",
		Options:     nil,
	}
}

func (plugin *MapPlugin) Register(ctx *plugintypes.PluginContext) (bool, error) {
	plugin.ctx = ctx

	ctx.Mux.HandleFunc("/html", plugin.generateHTML)

	ctx.Mux.HandleFunc("/png", func(w http.ResponseWriter, r *http.Request) {
		opts := NewRenderOptions(r, ctx.Config)

		_, port, _ := net.SplitHostPort(ctx.Config.Addr)
		if port == "" {
			port = "8080"
		}
		htmlURL := "http://127.0.0.1:" + port + "/map/html"
		opts.HTMLURL = htmlURL

		cacheKey := opts.ToCacheKey()
		overlays := plugintypes.MapOverlays{}

		for _, plugin := range ctx.EnabledPlugins() {
			if err := plugin.AddMapLayers(opts, &overlays); err != nil {
				log.Printf("plugin %s AddMapLayers error: %v", plugin.Info().Name, err)
			}
		}

		data, err := RenderPNG(opts)
		if err != nil {
			log.Printf("render error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := ctx.Cache.Save(cacheKey, data); err != nil {
			log.Printf("cache save failed: %v", err)
		}

		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	return false, nil
}

func (plugin *MapPlugin) generateHTML(w http.ResponseWriter, r *http.Request) {
	opts := NewRenderOptions(r, plugin.ctx.Config)
	overlays := plugintypes.MapOverlays{}

	for _, plugin := range plugin.ctx.EnabledPlugins() {
		if err := plugin.AddMapLayers(opts, &overlays); err != nil {
			log.Printf("plugin %s AddMapLayers error: %v", plugin.Info().Name, err)
		}
	}

	data, err := browserHTML(opts, overlays)
	if err != nil {
		log.Printf("browserHTML error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(data))
}

func (MapPlugin) AddMapLayers(opts interface{}, overlays *plugintypes.MapOverlays) error {
	return nil
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
