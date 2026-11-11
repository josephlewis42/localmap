package server

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"example.com/localmap/internal/cache"
	"example.com/localmap/internal/config"
)

func TestMapHandler(t *testing.T) {
	cfg := config.Config{CacheDir: t.TempDir()}
	srv := httptest.NewServer(NewRouter(cfg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/map?api_key=testkey&width_px=32&height_px=24")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Fatalf("unexpected content-type: %s", ct)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("empty body")
	}
}

func TestMapHandlerUnauthorized(t *testing.T) {
	cfg := config.Config{CacheDir: t.TempDir()}
	srv := httptest.NewServer(NewRouter(cfg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/map")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestMapHandlerUsesCache(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.Config{CacheDir: tmp}
	srv := httptest.NewServer(NewRouter(cfg))
	defer srv.Close()

	url := srv.URL + "/map?api_key=testkey&width_px=32&height_px=24"
	first, err := http.Get(url)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	defer first.Body.Close()
	firstBody, err := io.ReadAll(first.Body)
	if err != nil {
		t.Fatalf("read first body: %v", err)
	}

	cacheKey := cache.MakeKey(map[string]string{
		"width_px":   "32",
		"height_px":  "24",
		"profile":    "",
		"monochrome": "",
		"colors":     "",
		"depth":      "",
		"dither":     "",
	})
	cacheFile := filepath.Join(tmp, "render", cacheKeyHash(cacheKey)+".bin")
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("expected cached file to exist: %v", err)
	}

	second, err := http.Get(url)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	defer second.Body.Close()
	secondBody, err := io.ReadAll(second.Body)
	if err != nil {
		t.Fatalf("read second body: %v", err)
	}
	if string(firstBody) != string(secondBody) {
		t.Fatalf("expected second response to match first")
	}
}

func cacheKeyHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
