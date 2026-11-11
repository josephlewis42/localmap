package geojson

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	plugintypes "example.com/localmap/internal/plugins/types"
)

// GeoJSONPlugin fetches GeoJSON from a URL and overlays it on the map.
type GeoJSONPlugin struct {
	url string
}

func (p *GeoJSONPlugin) Info() *plugintypes.PluginInfo {
	return &plugintypes.PluginInfo{
		Name:        "geojson",
		Description: "Fetches GeoJSON from a URL and plots it on the map.",
		Options: []plugintypes.PluginOpt{
			{
				Name:  "LM_GEOJSON_URL",
				Usage: "URL to fetch GeoJSON from. Plugin is disabled if not set.",
			},
		},
	}
}

func (p *GeoJSONPlugin) Register(ctx *plugintypes.PluginContext) (bool, error) {
	url, ok := ctx.Env["LM_GEOJSON_URL"]
	if !ok || url == "" {
		return false, nil
	}
	p.url = url
	return true, nil
}

func (p *GeoJSONPlugin) AddMapLayers(_ interface{}, overlays *plugintypes.MapOverlays) error {
	resp, err := http.Get(p.url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("geojson: fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("geojson: server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("geojson: read body failed: %w", err)
	}

	if !json.Valid(body) {
		return fmt.Errorf("geojson: response is not valid JSON")
	}

	overlays.Sources = append(overlays.Sources, plugintypes.GeoJSONSource{
		ID:   "geojson",
		Data: json.RawMessage(body),
	})

	// Three layers cover all GeoJSON geometry types.
	overlays.Layers = append(overlays.Layers,
		plugintypes.MapLayer{
			ID:     "geojson-fill",
			Type:   "fill",
			Source: "geojson",
			Filter: []any{"==", "$type", "Polygon"},
			Paint:  map[string]any{"fill-color": "#0080ff", "fill-opacity": 0.4},
		},
		plugintypes.MapLayer{
			ID:     "geojson-line",
			Type:   "line",
			Source: "geojson",
			Filter: []any{"==", "$type", "LineString"},
			Paint:  map[string]any{"line-color": "#0080ff", "line-width": 2},
		},
		plugintypes.MapLayer{
			ID:     "geojson-circle",
			Type:   "circle",
			Source: "geojson",
			Filter: []any{"==", "$type", "Point"},
			Paint: map[string]any{
				"circle-radius":       6,
				"circle-color":        "#0080ff",
				"circle-opacity":      0.8,
				"circle-stroke-width": 1,
				"circle-stroke-color": "#ffffff",
			},
		},
	)

	return nil
}
