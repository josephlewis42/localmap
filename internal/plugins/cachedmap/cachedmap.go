package cachedmap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	plugintypes "example.com/localmap/internal/plugins/types"
)

const defaultTTL = 60 * time.Second
const retryInterval = 5 * time.Second

// CachedMapPlugin periodically pre-renders a map PNG and serves it from memory.
type CachedMapPlugin struct {
	mu    sync.RWMutex
	image []byte
}

func (p *CachedMapPlugin) Info() *plugintypes.PluginInfo {
	return &plugintypes.PluginInfo{
		Name:        "cachedmap",
		Description: "Periodically pre-renders a map PNG and serves it from cache.",
		Options: []plugintypes.PluginOpt{
			{
				Name:    "LM_CACHEDMAP_QUERY",
				Usage:   "URL query string passed to /map/png (e.g. profile=trmnl_v1_alias).",
				Default: "",
			},
			{
				Name:    "LM_CACHEDMAP_TTL",
				Usage:   "How often to regenerate the cached map (Go duration, e.g. 60s).",
				Default: "60s",
			},
		},
	}
}

func (p *CachedMapPlugin) Register(ctx *plugintypes.PluginContext) (bool, error) {
	ttl := defaultTTL
	if raw := ctx.Env["LM_CACHEDMAP_TTL"]; raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return false, fmt.Errorf("invalid LM_CACHEDMAP_TTL %q: %w", raw, err)
		}
		ttl = d
	}

	vals, _ := url.ParseQuery(ctx.Env["LM_CACHEDMAP_QUERY"])
	if vals == nil {
		vals = url.Values{}
	}

	_, port, _ := net.SplitHostPort(ctx.Config.Addr)
	if port == "" {
		port = "8080"
	}
	mapURL := "http://127.0.0.1:" + port + "/map/png"
	if len(vals) > 0 {
		mapURL += "?" + vals.Encode()
	}

	ctx.Mux.HandleFunc("/png", func(w http.ResponseWriter, r *http.Request) {
		p.mu.RLock()
		data := p.image
		p.mu.RUnlock()

		if data == nil {
			http.Error(w, "map not yet generated", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	go p.runLoop(ctx.Ctx, ctx.Logger, mapURL, ttl)

	return false, nil
}

func (p *CachedMapPlugin) runLoop(ctx context.Context, logger *slog.Logger, mapURL string, ttl time.Duration) {
	// Retry quickly until the first successful render (server may not be ready yet).
	for !p.generate(logger, mapURL) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryInterval):
		}
	}

	ticker := time.NewTicker(ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.generate(logger, mapURL)
		}
	}
}

// generate fetches the /map/png endpoint and stores the result. Returns true on success.
func (p *CachedMapPlugin) generate(logger *slog.Logger, mapURL string) bool {
	resp, err := http.Get(mapURL) //nolint:noctx
	if err != nil {
		logger.Error("cachedmap: fetch failed", "error", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("cachedmap: server returned non-200", "status", resp.StatusCode)
		return false
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("cachedmap: read body failed", "error", err)
		return false
	}

	p.mu.Lock()
	p.image = data
	p.mu.Unlock()
	return true
}

func (p *CachedMapPlugin) AddMapLayers(_ interface{}, _ *plugintypes.MapOverlays) error {
	return nil
}
