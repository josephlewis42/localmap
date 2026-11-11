package server

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"time"

	"example.com/localmap/internal/cache"
	"example.com/localmap/internal/config"
	"example.com/localmap/internal/plugins"
)

//go:embed landing_page.html
var landingPage []byte

// NewRouter creates the HTTP handler for the service.
func NewRouter(cfg config.Config) http.Handler {
	cacheManager := cache.NewManager(cfg.CacheDir, 5*time.Minute)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write(landingPage)
	})

	// Initialize plugins
	routes, err := plugins.InitializePlugins(context.Background(), cfg, cfg.CacheDir, mux)
	if err != nil {
		log.Fatalf("failed to initialize plugins: %v", err)
	}
	for _, r := range append([]string{"/healthz"}, routes...) {
		log.Printf("route: %s", r)
	}

	// Start cache manager cleanup in background
	go cacheManager.Start(context.Background())

	return mux
}
