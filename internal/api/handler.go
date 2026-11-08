package api

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/alexandrmotologa/regcraft/internal/config"
	"github.com/alexandrmotologa/regcraft/internal/storage"
	"github.com/alexandrmotologa/regcraft/internal/telemetry"
)

// Handler serves the Management REST API under /api/v1/.
type Handler struct {
	cfg       *config.Config
	store     *storage.Store
	metrics   *telemetry.Metrics
	version   string
	startTime time.Time
}

// NewHandler creates a management API router.
func NewHandler(cfg *config.Config, store *storage.Store, metrics *telemetry.Metrics, version string) *Handler {
	return &Handler{
		cfg:       cfg,
		store:     store,
		metrics:   metrics,
		version:   version,
		startTime: time.Now(),
	}
}

// ServeHTTP routes /api/v1/ endpoints.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "status":
		h.handleStatus(w, r)
	case path == "metrics":
		h.handleMetrics(w, r)
	case path == "upstreams":
		h.handleUpstreams(w, r)
	case path == "gc" && r.Method == http.MethodPost:
		h.handleGC(w, r)
	case path == "catalog" && r.Method == http.MethodGet:
		h.handleCatalogList(w, r)
	case strings.HasPrefix(path, "catalog/"):
		h.handleCatalogItem(w, r, strings.TrimPrefix(path, "catalog/"))
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "endpoint not found"})
	}
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	diskBytes, blobCount, _ := h.store.DiskUsage()
	repos := h.store.ListRepositories()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "healthy",
		"version":          h.version,
		"uptimeSeconds":    int64(time.Since(h.startTime).Seconds()),
		"cacheDir":         h.cfg.CacheDir,
		"quotaBytes":       h.cfg.QuotaBytes,
		"storageUsedBytes": diskBytes,
		"blobCount":        blobCount,
		"repoCount":        len(repos),
		"memoryAllocBytes": mem.Alloc,
		"memorySysBytes":   mem.Sys,
		"numGoroutine":     runtime.NumGoroutine(),
		"offlineMode":      h.cfg.OfflineMode,
	})
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_ = json.NewEncoder(w).Encode(h.metrics.GetSnapshot())
}

func (h *Handler) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	type UpstreamItem struct {
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
		IsDefault bool  `json:"isDefault"`
	}

	items := []UpstreamItem{
		{Name: "docker.io", Endpoint: h.cfg.DefaultUpstream, IsDefault: true},
	}
	for domain, endpoint := range h.cfg.UpstreamMirrors {
		if domain != "docker.io" {
			items = append(items, UpstreamItem{
				Name:     domain,
				Endpoint: endpoint,
				IsDefault: false,
			})
		}
	}

	_ = json.NewEncoder(w).Encode(items)
}

type TagSummary struct {
	Name         string    `json:"name"`
	Digest       string    `json:"digest"`
	MediaType    string    `json:"mediaType"`
	TotalSize    int64     `json:"totalSize"`
	LayerCount   int       `json:"layerCount"`
	LastAccessed time.Time `json:"lastAccessed"`
}

type RepoSummary struct {
	Repository   string       `json:"repository"`
	Tags         []TagSummary `json:"tags"`
	TotalSize    int64        `json:"totalSize"`
	LastAccessed time.Time    `json:"lastAccessed"`
}

func (h *Handler) handleCatalogList(w http.ResponseWriter, r *http.Request) {
	repos := h.store.ListRepositories()
	var result []RepoSummary

	for _, repo := range repos {
		tags := h.store.ListTags(repo)
		rs := RepoSummary{
			Repository: repo,
			Tags:       []TagSummary{},
		}

		for _, tag := range tags {
			cm, err := h.store.GetManifest(repo, tag)
			if err == nil {
				var size int64
				for _, l := range cm.Layers {
					size += l.Size
				}
				ts := TagSummary{
					Name:         tag,
					Digest:       cm.Digest,
					MediaType:    cm.MediaType,
					TotalSize:    size,
					LayerCount:   len(cm.Layers),
					LastAccessed: cm.LastAccessed,
				}
				rs.Tags = append(rs.Tags, ts)
				rs.TotalSize += size
				if cm.LastAccessed.After(rs.LastAccessed) {
					rs.LastAccessed = cm.LastAccessed
				}
			}
		}
		result = append(result, rs)
	}

	if result == nil {
		result = []RepoSummary{}
	}

	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleCatalogItem(w http.ResponseWriter, r *http.Request, subPath string) {
	// subPath: <repo>/tags/<tag>
	parts := strings.Split(subPath, "/tags/")
	if len(parts) != 2 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "expected format /api/v1/catalog/<repo>/tags/<tag>"})
		return
	}

	repo := parts[0]
	tag := parts[1]

	switch r.Method {
	case http.MethodGet:
		cm, err := h.store.GetManifest(repo, tag)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "tag not found"})
			return
		}
		_ = json.NewEncoder(w).Encode(cm)

	case http.MethodDelete:
		err := h.store.DeleteTag(repo, tag)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"deleted": tag,
			"repo":    repo,
		})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type GCRequest struct {
	QuotaBytes int64 `json:"quotaBytes"`
	DryRun     bool  `json:"dryRun"`
}

func (h *Handler) handleGC(w http.ResponseWriter, r *http.Request) {
	var req GCRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	quota := req.QuotaBytes
	if quota <= 0 {
		quota = h.cfg.QuotaBytes
	}

	report, err := h.store.RunGC(quota, req.DryRun)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(report)
}
