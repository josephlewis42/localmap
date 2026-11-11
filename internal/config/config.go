package config

import (
	"os"
	"strconv"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	CacheDir string
	Addr     string
	MapLatDD float64
	MapLonDD float64
	MapZoom  float64
}

// LoadFromEnv reads config from environment variables and returns a Config.
func LoadFromEnv() Config {
	cfg := Config{
		CacheDir: os.Getenv("LM_CACHE_DIR"),
		Addr:     os.Getenv("LM_ADDR"),
		MapLatDD: parseFloat(os.Getenv("LM_MAP_LAT_DD"), 0.0),
		MapLonDD: parseFloat(os.Getenv("LM_MAP_LON_DD"), 0.0),
		MapZoom:  parseFloat(os.Getenv("LM_MAP_ZOOM"), 12),
	}
	if cfg.CacheDir == "" {
		cfg.CacheDir = "/tmp"
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	return cfg
}

func parseFloat(value string, def float64) float64 {
	if value == "" {
		return def
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return def
	}
	return f
}
