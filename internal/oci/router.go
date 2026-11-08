package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/regcraft/internal/config"
	"github.com/alexandrmotologa/regcraft/internal/storage"
	"github.com/alexandrmotologa/regcraft/internal/telemetry"
	"github.com/alexandrmotologa/regcraft/internal/upstream"
)

// Handler serves OCI Distribution Specification v1.1 and Docker v2 endpoints.
type Handler struct {
	cfg      *config.Config
	store    *storage.Store
	upstream *upstream.Client
	metrics  *telemetry.Metrics
}

// NewHandler creates a new OCI distribution router.
func NewHandler(cfg *config.Config, store *storage.Store, upstreamClient *upstream.Client, metrics *telemetry.Metrics) *Handler {
	return &Handler{
		cfg:      cfg,
		store:    store,
		upstream: upstreamClient,
		metrics:  metrics,
	}
}

// ServeHTTP routes incoming OCI requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Always set the OCI/Docker distribution version header
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")

	path := strings.TrimPrefix(r.URL.Path, "/v2")
	path = strings.TrimPrefix(path, "/")

	// Ping endpoint: /v2 or /v2/
	if path == "" {
		h.handlePing(w, r)
		return
	}

	// Catalog endpoint: /v2/_catalog
	if path == "_catalog" {
		h.handleCatalog(w, r)
		return
	}

	// Tags list endpoint: /v2/<name>/tags/list
	if strings.HasSuffix(path, "/tags/list") {
		repo := strings.TrimSuffix(path, "/tags/list")
		h.handleTagsList(w, r, repo)
		return
	}

	// Manifest endpoints: /v2/<name>/manifests/<reference>
	if idx := strings.Index(path, "/manifests/"); idx != -1 {
		repo := path[:idx]
		reference := path[idx+len("/manifests/"):]
		h.handleManifest(w, r, repo, reference)
		return
	}

	// Blob endpoints: /v2/<name>/blobs/<digest>
	if idx := strings.Index(path, "/blobs/"); idx != -1 {
		repo := path[:idx]
		digest := path[idx+len("/blobs/"):]
		h.handleBlob(w, r, repo, digest)
		return
	}

	WriteError(w, http.StatusNotFound, ErrCodeUnsupported, "unsupported OCI registry endpoint")
}

// handlePing responds to GET /v2/ with 200 OK.
func (h *Handler) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{}\n"))
}

// handleCatalog returns the list of cached repositories.
func (h *Handler) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	repos := h.store.ListRepositories()
	if repos == nil {
		repos = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"repositories": repos,
	})
}

// handleTagsList returns the tags for a given repository.
func (h *Handler) handleTagsList(w http.ResponseWriter, r *http.Request, repo string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	tags := h.store.ListTags(repo)
	if tags == nil {
		tags = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"name": repo,
		"tags": tags,
	})
}

// handleManifest serves manifests (HEAD or GET).
func (h *Handler) handleManifest(w http.ResponseWriter, r *http.Request, repo, reference string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	resolved := upstream.ResolveRepository(repo, h.cfg.DefaultUpstream, h.cfg.UpstreamMirrors)
	isDigest := strings.HasPrefix(reference, "sha256:")

	// 1. If requesting by immutable digest, check local CAS first
	if isDigest {
		cached, err := h.store.GetManifest(repo, reference)
		if err == nil {
			h.metrics.RecordManifestHit()
			h.writeManifestResponse(w, r, cached)
			return
		}
	} else {
		// 2. Requesting by mutable tag: check local cache
		cached, err := h.store.GetManifest(repo, reference)
		if err == nil {
			// Check if upstream has changed
			ctx, cancel := context.WithTimeout(r.Context(), h.cfg.UpstreamTimeout)
			defer cancel()

			changed, _, checkErr := h.upstream.CheckManifest(ctx, resolved, reference, cached.Digest)
			if checkErr == nil && !changed {
				// Cache HIT: manifest unchanged upstream!
				h.metrics.RecordManifestHit()
				h.writeManifestResponse(w, r, cached)
				return
			}

			// If upstream check failed but offline mode is active, serve cached
			if checkErr != nil && h.cfg.OfflineMode {
				h.metrics.RecordManifestHit()
				h.writeManifestResponse(w, r, cached)
				return
			}
		}
	}

	// 3. Cache MISS: Fetch manifest from upstream
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.UpstreamTimeout)
	defer cancel()

	result, err := h.upstream.FetchManifest(ctx, resolved, reference)
	if err != nil {
		// If fetch failed and we have a cached version, fallback to cached
		if h.cfg.OfflineMode {
			if cached, errCache := h.store.GetManifest(repo, reference); errCache == nil {
				h.metrics.RecordManifestHit()
				h.writeManifestResponse(w, r, cached)
				return
			}
		}

		WriteError(w, http.StatusNotFound, ErrCodeManifestUnknown, fmt.Sprintf("failed to fetch manifest from upstream: %v", err))
		return
	}

	h.metrics.RecordManifestMiss()

	// Persist in local storage
	cached, err := h.store.SaveManifest(repo, reference, result.MediaType, result.Payload)
	if err != nil {
		// If save fails, still deliver upstream payload to client
		w.Header().Set("Content-Type", result.MediaType)
		w.Header().Set("Docker-Content-Digest", result.Digest)
		w.Header().Set("Content-Length", strconv.Itoa(len(result.Payload)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(result.Payload)
		}
		return
	}

	h.writeManifestResponse(w, r, cached)
}

// writeManifestResponse sets standard headers and outputs the manifest body.
func (h *Handler) writeManifestResponse(w http.ResponseWriter, r *http.Request, cm *storage.CachedManifest) {
	w.Header().Set("Content-Type", cm.MediaType)
	w.Header().Set("Docker-Content-Digest", cm.Digest)
	w.Header().Set("Etag", fmt.Sprintf("%q", cm.Digest))
	w.Header().Set("Content-Length", strconv.FormatInt(cm.Size, 10))

	if r.Method == http.MethodGet {
		_, _ = w.Write(cm.Payload)
	}
}

// handleBlob serves image layers and configs (HEAD or GET).
func (h *Handler) handleBlob(w http.ResponseWriter, r *http.Request, repo, digest string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 1. Check if blob exists locally in CAS
	if h.store.HasBlob(digest) {
		file, size, err := h.store.OpenBlob(digest)
		if err == nil {
			defer file.Close()

			h.metrics.RecordHit(size)

			w.Header().Set("Docker-Content-Digest", digest)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Accept-Ranges", "bytes")

			http.ServeContent(w, r, digest, time.Time{}, file)
			return
		}
	}

	// 2. Cache MISS: Stream from upstream while writing to CAS
	resolved := upstream.ResolveRepository(repo, h.cfg.DefaultUpstream, h.cfg.UpstreamMirrors)

	h.metrics.IncActivePulls()
	defer h.metrics.DecActivePulls()

	ctx := r.Context()
	upstreamBody, contentLength, contentType, err := h.upstream.FetchBlob(ctx, resolved, digest)
	if err != nil {
		WriteError(w, http.StatusNotFound, ErrCodeBlobUnknown, fmt.Sprintf("blob not found upstream: %v", err))
		return
	}
	defer upstreamBody.Close()

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Type", contentType)
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}

	// If HEAD request, do not stream body or write to disk
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Create streaming CAS writer with SHA-256 validation
	blobWriter, err := h.store.NewBlobWriter(digest)
	if err != nil {
		// Fallback: direct pipe to client without saving if local disk error
		_, _ = io.Copy(w, upstreamBody)
		return
	}
	defer blobWriter.Abort()

	// TeeReader streams to response writer while simultaneously hashing and saving to disk
	tee := io.TeeReader(upstreamBody, w)
	copied, err := io.Copy(blobWriter, tee)
	if err != nil {
		// Client aborted or upstream disconnected mid-stream
		return
	}

	// Verify SHA-256 and commit atomically
	if commitErr := blobWriter.Commit(); commitErr == nil {
		h.metrics.RecordMiss(copied)
	}
}
