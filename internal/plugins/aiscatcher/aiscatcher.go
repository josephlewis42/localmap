package aiscatcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	plugintypes "example.com/localmap/internal/plugins/types"
)

// ShipsFull is the response from /api/ships_full.json
type ShipsFull struct {
	Count int `json:"count"`
	Ships []struct {
		Mmsi        int         `json:"mmsi"`
		Lat         float64     `json:"lat"`
		Lon         float64     `json:"lon"`
		Count       int         `json:"count"`
		Ppm         float64     `json:"ppm"`
		GroupMask   int         `json:"group_mask"`
		Heading     *float64    `json:"heading"`
		Cog         *float64    `json:"cog"`
		Speed       *float64    `json:"speed"`
		Shiptype    int         `json:"shiptype"`
		Shipclass   int         `json:"shipclass"`
		Validated   int         `json:"validated"`
		MsgType     int         `json:"msg_type"`
		Channels    int         `json:"channels"`
		Country     string      `json:"country"`
		Status      int         `json:"status"`
		Draught     interface{} `json:"draught"`
		EtaMonth    interface{} `json:"eta_month"`
		EtaDay      interface{} `json:"eta_day"`
		EtaHour     interface{} `json:"eta_hour"`
		EtaMinute   interface{} `json:"eta_minute"`
		Imo         interface{} `json:"imo"`
		Callsign    string      `json:"callsign"`
		Shipname    string      `json:"shipname"`
		Destination string      `json:"destination"`
		Eni         string      `json:"eni"`
		Vendorid    string      `json:"vendorid"`
		Model       interface{} `json:"model"`
		Serial      interface{} `json:"serial"`
		Repeat      int         `json:"repeat"`
		LastSignal  int         `json:"last_signal"`
	} `json:"ships"`
	Error bool `json:"error"`
}

// AISCatcherPlugin fetches ship positions from an AISCatcher instance and plots them on the map.
type AISCatcherPlugin struct {
	baseURL string
}

func (p *AISCatcherPlugin) Info() *plugintypes.PluginInfo {
	return &plugintypes.PluginInfo{
		Name:        "aiscatcher",
		Description: "Fetches AIS ship positions from an AISCatcher instance and plots them on the map.",
		Options: []plugintypes.PluginOpt{
			{
				Name:  "LM_AISCATCHER_URL",
				Usage: "Base URL of the AISCatcher instance (e.g. http://192.168.1.100:8100). Plugin is disabled if not set.",
			},
		},
	}
}

func (p *AISCatcherPlugin) Register(ctx *plugintypes.PluginContext) (bool, error) {
	url, ok := ctx.Env["LM_AISCATCHER_URL"]
	if !ok || url == "" {
		return false, nil
	}
	p.baseURL = strings.TrimRight(url, "/")
	return true, nil
}

func (p *AISCatcherPlugin) AddMapLayers(_ interface{}, overlays *plugintypes.MapOverlays) error {
	resp, err := http.Get(p.baseURL + "/api/ships_full.json") //nolint:noctx
	if err != nil {
		return fmt.Errorf("aiscatcher: fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("aiscatcher: server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("aiscatcher: read body failed: %w", err)
	}

	var data ShipsFull
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("aiscatcher: parse failed: %w", err)
	}

	features := make([]map[string]any, 0, len(data.Ships))
	for _, ship := range data.Ships {
		label := ship.Shipname
		if label == "" {
			label = ship.Callsign
		}

		props := map[string]any{
			"mmsi":     ship.Mmsi,
			"name":     ship.Shipname,
			"callsign": ship.Callsign,
		}
		if label != "" {
			props["label"] = label
		}

		// Last seen more than 30 mins ago, skip
		if ship.LastSignal > (30 * 60) {
			continue
		}

		// AIS heading 511 means "not available"; COG 360.0 means "not available".
		if ship.Heading != nil && *ship.Heading >= 0 && *ship.Heading < 360 {
			props["bearing"] = *ship.Heading
		} else if ship.Cog != nil && *ship.Cog >= 0 && *ship.Cog < 360 {
			props["bearing"] = *ship.Cog
		}

		features = append(features, map[string]any{
			"type": "Feature",
			"geometry": map[string]any{
				"type":        "Point",
				"coordinates": []float64{ship.Lon, ship.Lat},
			},
			"properties": props,
		})
	}

	fc := map[string]any{
		"type":     "FeatureCollection",
		"features": features,
	}

	geojsonData, err := json.Marshal(fc)
	if err != nil {
		return fmt.Errorf("aiscatcher: marshal geojson: %w", err)
	}

	overlays.Sources = append(overlays.Sources, plugintypes.GeoJSONSource{
		ID:   "aiscatcher",
		Data: json.RawMessage(geojsonData),
	})

	overlays.Layers = append(overlays.Layers,
		// Ships with a known bearing: rotated triangle symbol.
		plugintypes.MapLayer{
			ID:     "aiscatcher-headed",
			Type:   "symbol",
			Source: "aiscatcher",
			Filter: []any{"has", "bearing"},
			Layout: map[string]any{
				"text-field":              "▲",
				"text-size":               14,
				"text-rotate":             []any{"get", "bearing"},
				"text-rotation-alignment": "map",
				"text-allow-overlap":      true,
				"text-ignore-placement":   true,
			},
			Paint: map[string]any{
				"text-color":      "#0066cc",
				"text-halo-color": "#ffffff",
				"text-halo-width": 1,
			},
		},
		// Ships without a known bearing: circle dot.
		plugintypes.MapLayer{
			ID:     "aiscatcher-unheaded",
			Type:   "circle",
			Source: "aiscatcher",
			Filter: []any{"!", []any{"has", "bearing"}},
			Paint: map[string]any{
				"circle-radius":       5,
				"circle-color":        "#0066cc",
				"circle-opacity":      0.8,
				"circle-stroke-width": 1,
				"circle-stroke-color": "#ffffff",
			},
		},
		// Ship name labels (shipname preferred, callsign fallback), lower-right of the symbol.
		plugintypes.MapLayer{
			ID:     "aiscatcher-labels",
			Type:   "symbol",
			Source: "aiscatcher",
			Filter: []any{"has", "label"},
			Layout: map[string]any{
				"text-field":            []any{"get", "label"},
				"text-size":             11,
				"text-anchor":           "top-left",
				"text-offset":           []float64{0.6, 0.6},
				"text-allow-overlap":    false,
				"text-ignore-placement": false,
			},
			Paint: map[string]any{
				"text-color":      "#003399",
				"text-halo-color": "#ffffff",
				"text-halo-width": 1.5,
			},
		},
	)

	return nil
}
