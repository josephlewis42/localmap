package mapper

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	plugintypes "example.com/localmap/internal/plugins/types"
)

func renderWithBrowser(opts RenderOptions) ([]byte, error) {
	exePath := os.Getenv("CHROME_PATH")
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("window-size", fmt.Sprintf("%d,%d", opts.Width, opts.Height)),
		// maplibre-gl requires WebGL; without software rendering the canvas
		// stays blank (or, on some chromium builds, the WebGL context fails
		// to initialize entirely and maplibre-gl throws during construction).
		chromedp.Flag("use-gl", "angle"),
		chromedp.Flag("use-angle", "swiftshader"),
		chromedp.Flag("enable-unsafe-swiftshader", true),
	)
	if exePath != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(exePath))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *runtime.EventExceptionThrown:
			log.Printf("browser render: js exception: %s", e.ExceptionDetails.Error())
		case *runtime.EventConsoleAPICalled:
			if e.Type == "error" || e.Type == "warning" {
				log.Printf("browser render: console.%s: %v", e.Type, e.Args)
			}
		}
	})

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var buf []byte
	err := chromedp.Run(ctx,
		runtime.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return emulation.SetDeviceMetricsOverride(int64(opts.Width), int64(opts.Height), 1.0, false).Do(ctx)
		}),
		chromedp.Navigate(opts.HTMLURL),
		chromedp.Poll(`window.status === 'ready'`, nil),
		chromedp.FullScreenshot(&buf, 100),
	)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func browserHTML(opts RenderOptions, overlays plugintypes.MapOverlays) (string, error) {
	sourcesJSON, err := json.Marshal(overlays.Sources)
	if err != nil {
		return "", fmt.Errorf("marshal sources: %w", err)
	}
	layersJSON, err := json.Marshal(overlays.Layers)
	if err != nil {
		return "", fmt.Errorf("marshal layers: %w", err)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>localmap</title>
<script src="https://unpkg.com/maplibre-gl/dist/maplibre-gl.js"></script>
<link href="https://unpkg.com/maplibre-gl/dist/maplibre-gl.css" rel="stylesheet" />
<style>
html, body, #map { margin: 0; padding: 0; width: 100%%; height: 100%%; }
body { background: #fff; }
</style>
</head>
<body>
<div id="map"></div>
<script>
  // Registered first so a synchronous throw below (e.g. WebGL init failure)
  // can't prevent window.status from ever being set.
  setTimeout(() => { window.status = 'ready'; }, 7000);
  try {
    const map = new maplibregl.Map({
      style: 'https://tiles.openfreemap.org/styles/positron',
      center: [%f, %f],
      zoom: %f,
      container: 'map',
    });
    map.on('load', () => {
      map.resize();
      const sources = %s || [];
      sources.forEach(s => map.addSource(s.id, {type: 'geojson', data: s.data}));
      const layers = %s || [];
      layers.forEach(l => map.addLayer(l));
    });
    map.on('idle', () => { window.status = 'ready'; });
    map.on('error', e => console.error('maplibre error', e && e.error));
  } catch (e) {
    console.error('map init failed', e);
  }
</script>
</body>
</html>`, opts.CenterLon, opts.CenterLat, opts.Zoom, sourcesJSON, layersJSON), nil
}
