package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config represents the runtime configuration for regcraft.
type Config struct {
	// ListenAddr is the host:port for the HTTP server.
	ListenAddr string `json:"listenAddr"`

	// CacheDir is the root directory on disk for content-addressed storage.
	CacheDir string `json:"cacheDir"`

	// QuotaBytes is the maximum disk space allowed before LRU eviction.
	QuotaBytes int64 `json:"quotaBytes"`

	// DefaultUpstream is the default registry URL (Docker Hub).
	DefaultUpstream string `json:"defaultUpstream"`

	// UpstreamMirrors maps registry domains to their base upstream endpoints.
	UpstreamMirrors map[string]string `json:"upstreamMirrors"`

	// UpstreamTimeout is the HTTP timeout for contacting upstreams.
	UpstreamTimeout time.Duration `json:"upstreamTimeout"`

	// OfflineMode allows serving cached images when upstream is unreachable.
	OfflineMode bool `json:"offlineMode"`

	// AuthEnabled indicates whether basic authentication is required for clients.
	AuthEnabled bool `json:"authEnabled"`

	// AuthUsers holds username:password credentials for client authentication.
	AuthUsers map[string]string `json:"authUsers,omitempty"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, ".regcraft", "cache")
	if os.Getenv("REGCRAFT_DIR") != "" {
		defaultDir = os.Getenv("REGCRAFT_DIR")
	}

	return &Config{
		ListenAddr:      ":5000",
		CacheDir:        defaultDir,
		QuotaBytes:      20 * 1024 * 1024 * 1024, // 20 GB default quota
		DefaultUpstream: "https://registry-1.docker.io",
		UpstreamMirrors: map[string]string{
			"docker.io": "https://registry-1.docker.io",
			"ghcr.io":   "https://ghcr.io",
			"quay.io":   "https://quay.io",
			"gcr.io":    "https://gcr.io",
		},
		UpstreamTimeout: 30 * time.Second,
		OfflineMode:     true,
		AuthEnabled:     false,
		AuthUsers:       make(map[string]string),
	}
}

// ParseByteSize converts human-readable strings like "500MB", "20GB", "1TB" into bytes.
func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, fmt.Errorf("empty byte size string")
	}

	var multiplier int64 = 1
	var numStr string

	switch {
	case strings.HasSuffix(s, "TB") || strings.HasSuffix(s, "T"):
		multiplier = 1024 * 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(strings.TrimSuffix(s, "TB"), "T")
	case strings.HasSuffix(s, "GB") || strings.HasSuffix(s, "G"):
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(strings.TrimSuffix(s, "GB"), "G")
	case strings.HasSuffix(s, "MB") || strings.HasSuffix(s, "M"):
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(strings.TrimSuffix(s, "MB"), "M")
	case strings.HasSuffix(s, "KB") || strings.HasSuffix(s, "K"):
		multiplier = 1024
		numStr = strings.TrimSuffix(strings.TrimSuffix(s, "KB"), "K")
	case strings.HasSuffix(s, "B"):
		multiplier = 1
		numStr = strings.TrimSuffix(s, "B")
	default:
		multiplier = 1
		numStr = s
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte size %q: %w", s, err)
	}

	return int64(val * float64(multiplier)), nil
}

// FormatByteSize converts a byte count into a human-readable string.
func FormatByteSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
