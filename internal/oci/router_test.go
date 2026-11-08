package oci

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alexandrmotologa/regcraft/internal/config"
	"github.com/alexandrmotologa/regcraft/internal/storage"
	"github.com/alexandrmotologa/regcraft/internal/telemetry"
	"github.com/alexandrmotologa/regcraft/internal/upstream"
)

func TestOCIPingAndCatalog(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regcraft-oci-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := config.DefaultConfig()
	cfg.CacheDir = tempDir

	store, err := storage.NewStore(tempDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	metrics := telemetry.NewMetrics()
	upstreamClient := upstream.NewClient(2 * time.Second)
	handler := NewHandler(cfg, store, upstreamClient, metrics)

	// 1. Test GET /v2/
	req := httptest.NewRequest(http.MethodGet, "/v2/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /v2/, got %d", resp.StatusCode)
	}
	if v := resp.Header.Get("Docker-Distribution-API-Version"); v != "registry/2.0" {
		t.Errorf("expected Docker-Distribution-API-Version header, got %q", v)
	}

	// 2. Test GET /v2/_catalog
	reqCat := httptest.NewRequest(http.MethodGet, "/v2/_catalog", nil)
	wCat := httptest.NewRecorder()
	handler.ServeHTTP(wCat, reqCat)

	respCat := wCat.Result()
	if respCat.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /v2/_catalog, got %d", respCat.StatusCode)
	}

	var catResp struct {
		Repositories []string `json:"repositories"`
	}
	_ = json.NewDecoder(respCat.Body).Decode(&catResp)
	if len(catResp.Repositories) != 0 {
		t.Errorf("expected empty catalog, got %v", catResp.Repositories)
	}
}

func TestOCIManifestAndBlobStreaming(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regcraft-stream-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	blobData := []byte("high performance container layer raw bytes")
	h := sha256.Sum256(blobData)
	blobDigest := "sha256:" + hex.EncodeToString(h[:])

	manifestPayload := []byte(fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {"mediaType": "application/vnd.docker.container.image.v1+json", "size": 100, "digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		"layers": [{"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip", "size": %d, "digest": "%s"}]
	}`, len(blobData), blobDigest))

	// Mock Upstream Server
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/library/ubuntu/manifests/24.04" {
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Header().Set("Docker-Content-Digest", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(manifestPayload)
			return
		}

		if r.URL.Path == "/v2/library/ubuntu/blobs/"+blobDigest {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(blobData)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(blobData)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockUpstream.Close()

	cfg := config.DefaultConfig()
	cfg.CacheDir = tempDir
	cfg.DefaultUpstream = mockUpstream.URL

	store, err := storage.NewStore(tempDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	metrics := telemetry.NewMetrics()
	upstreamClient := upstream.NewClient(5 * time.Second)
	handler := NewHandler(cfg, store, upstreamClient, metrics)

	// 1. Fetch Manifest (Cache MISS -> Upstream Fetch -> Local Save)
	reqMan := httptest.NewRequest(http.MethodGet, "/v2/library/ubuntu/manifests/24.04", nil)
	wMan := httptest.NewRecorder()
	handler.ServeHTTP(wMan, reqMan)

	respMan := wMan.Result()
	if respMan.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for manifest fetch, got %d", respMan.StatusCode)
	}
	bodyMan, _ := io.ReadAll(respMan.Body)
	if !bytes.Equal(bodyMan, manifestPayload) {
		t.Errorf("manifest payload mismatch")
	}

	// 2. Fetch Blob First Time (Cache MISS -> Stream to client + Tee to disk)
	reqBlob1 := httptest.NewRequest(http.MethodGet, "/v2/library/ubuntu/blobs/"+blobDigest, nil)
	wBlob1 := httptest.NewRecorder()
	handler.ServeHTTP(wBlob1, reqBlob1)

	respBlob1 := wBlob1.Result()
	if respBlob1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for first blob pull, got %d", respBlob1.StatusCode)
	}
	bodyBlob1, _ := io.ReadAll(respBlob1.Body)
	if !bytes.Equal(bodyBlob1, blobData) {
		t.Errorf("blob content mismatch on first pull")
	}

	// Verify that blob was saved into local CAS
	if !store.HasBlob(blobDigest) {
		t.Errorf("blob should have been saved to local CAS after first pull")
	}

	// 3. Fetch Blob Second Time (Cache HIT -> Served directly from SSD/CAS)
	reqBlob2 := httptest.NewRequest(http.MethodGet, "/v2/library/ubuntu/blobs/"+blobDigest, nil)
	wBlob2 := httptest.NewRecorder()
	handler.ServeHTTP(wBlob2, reqBlob2)

	respBlob2 := wBlob2.Result()
	if respBlob2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for second blob pull (cache hit), got %d", respBlob2.StatusCode)
	}
	bodyBlob2, _ := io.ReadAll(respBlob2.Body)
	if !bytes.Equal(bodyBlob2, blobData) {
		t.Errorf("blob content mismatch on second pull")
	}

	// Check metrics
	snap := metrics.GetSnapshot()
	if snap.CacheHits < 1 {
		t.Errorf("expected at least 1 cache hit, got %d", snap.CacheHits)
	}
	if snap.CacheMisses < 1 {
		t.Errorf("expected at least 1 cache miss, got %d", snap.CacheMisses)
	}
}
