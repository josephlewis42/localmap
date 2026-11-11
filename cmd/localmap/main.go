package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"example.com/localmap/internal/config"
	"example.com/localmap/internal/server"
)

func main() {
	cfg := config.LoadFromEnv()

	router := server.NewRouter(cfg)

	addr := cfg.Addr
	displayAddr := addr
	if strings.HasPrefix(addr, ":") {
		displayAddr = fmt.Sprintf("localhost%s", addr)
	}

	log.Printf("starting localmap on http://%s (cache=%s)", displayAddr, cfg.CacheDir)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
