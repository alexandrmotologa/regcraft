package test

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
	"strings"
	"testing"
	"time"

	"github.com/alexandrmotologa/regcraft/internal/api"
	"github.com/alexandrmotologa/regcraft/internal/config"
	"github.com/alexandrmotologa/regcraft/internal/oci"
	"github.com/alexandrmotologa/regcraft/internal/storage"
	"github.com/alexandrmotologa/regcraft/internal/telemetry"
	"github.com/alexandrmotologa/regcraft/internal/ui"
	"github.com/alexandrmotologa/regcraft/internal/upstream"
)

func TestEndToEndFullLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regcraft-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test layers
	baseLayerData := []byte("debian-rootfs-base-layer-10gbps-ssd-speed")
	bh := sha256.Sum256(baseLayerData)
	baseDigest := "sha256:" + hex.EncodeToString(bh[:])

	appLayerData := []byte("app-binary-layer-optimized-go")
	ah := sha256.Sum256(appLayerData)
	appDigest := "sha256:" + hex.EncodeToString(ah[:])

	manifestPayload := []byte(fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"size": 50,
			"digest": "sha256:2222222222222222222222222222222222222222222222222222222222222222"
		},
		"layers": [
			{"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip", "size": %d, "digest": "%s"},
			{"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip", "size": %d, "digest": "%s"}
		]
	}`, len(baseLayerData), baseDigest, len(appLayerData), appDigest))

	// Mock Upstream Registry
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/library/postgres/manifests/16":
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Header().Set("Docker-Content-Digest", "sha256:3333333333333333333333333333333333333333333333333333333333333333")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(manifestPayload)

		case "/v2/library/postgres/blobs/" + baseDigest:
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(baseLayerData)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(baseLayerData)

		case "/v2/library/postgres/blobs/" + appDigest:
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(appLayerData)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(appLayerData)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockUpstream.Close()

	// Initialize regcraft daemon
	cfg := config.DefaultConfig()
	cfg.CacheDir = tempDir
	cfg.DefaultUpstream = mockUpstream.URL

	store, err := storage.NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	metrics := telemetry.NewMetrics()
	upstreamClient := upstream.NewClient(5 * time.Second)

	ociHandler := oci.NewHandler(cfg, store, upstreamClient, metrics)
	apiHandler := api.NewHandler(cfg, store, metrics, "1.0.0")
	uiHandler := ui.Handler()

	mux := http.NewServeMux()
	mux.Handle("/v2/", ociHandler)
	mux.Handle("/v2", ociHandler)
	mux.Handle("/api/v1/", apiHandler)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.WritePrometheus(w)
	})
	mux.Handle("/", uiHandler)

	regcraftServer := httptest.NewServer(mux)
	defer regcraftServer.Close()

	client := regcraftServer.Client()

	// Step 1: Test OCI /v2/ Ping
	respPing, err := client.Get(regcraftServer.URL + "/v2/")
	if err != nil {
		t.Fatalf("GET /v2/ failed: %v", err)
	}
	if respPing.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", respPing.StatusCode)
	}
	if v := respPing.Header.Get("Docker-Distribution-API-Version"); v != "registry/2.0" {
		t.Errorf("missing or incorrect distribution header: %q", v)
	}
	_ = respPing.Body.Close()

	// Step 2: Test Web Dashboard (Single Page App index.html)
	respUI, err := client.Get(regcraftServer.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	if respUI.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for UI, got %d", respUI.StatusCode)
	}
	uiContent, _ := io.ReadAll(respUI.Body)
	_ = respUI.Body.Close()
	if !strings.Contains(string(uiContent), "regcraft") {
		t.Errorf("embedded UI does not mention regcraft")
	}

	// Step 3: First Pull of Manifest (Cache MISS -> Upstream)
	respMan1, err := client.Get(regcraftServer.URL + "/v2/library/postgres/manifests/16")
	if err != nil {
		t.Fatalf("GET manifest failed: %v", err)
	}
	if respMan1.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for manifest pull, got %d", respMan1.StatusCode)
	}
	manBody1, _ := io.ReadAll(respMan1.Body)
	_ = respMan1.Body.Close()
	if !bytes.Equal(manBody1, manifestPayload) {
		t.Errorf("manifest body mismatch")
	}

	// Step 4: First Pull of Base Layer (Cache MISS -> Stream + Disk CAS save)
	respBlob1, err := client.Get(regcraftServer.URL + "/v2/library/postgres/blobs/" + baseDigest)
	if err != nil {
		t.Fatalf("GET blob failed: %v", err)
	}
	if respBlob1.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for blob, got %d", respBlob1.StatusCode)
	}
	blobData1, _ := io.ReadAll(respBlob1.Body)
	_ = respBlob1.Body.Close()
	if !bytes.Equal(blobData1, baseLayerData) {
		t.Errorf("blob body mismatch")
	}

	// Verify CAS saved baseLayer
	if !store.HasBlob(baseDigest) {
		t.Fatalf("base layer should exist in CAS")
	}

	// Step 5: Second Pull of Base Layer (Cache HIT -> Served directly from local CAS)
	respBlob2, err := client.Get(regcraftServer.URL + "/v2/library/postgres/blobs/" + baseDigest)
	if err != nil {
		t.Fatalf("GET blob hit failed: %v", err)
	}
	if respBlob2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for cached blob, got %d", respBlob2.StatusCode)
	}
	blobData2, _ := io.ReadAll(respBlob2.Body)
	_ = respBlob2.Body.Close()
	if !bytes.Equal(blobData2, baseLayerData) {
		t.Errorf("cached blob body mismatch")
	}

	// Step 6: Verify Management REST API Catalog
	respCat, err := client.Get(regcraftServer.URL + "/api/v1/catalog")
	if err != nil {
		t.Fatalf("GET /api/v1/catalog failed: %v", err)
	}
	var catalog []api.RepoSummary
	_ = json.NewDecoder(respCat.Body).Decode(&catalog)
	_ = respCat.Body.Close()

	if len(catalog) != 1 || catalog[0].Repository != "library/postgres" {
		t.Errorf("unexpected catalog: %+v", catalog)
	}

	// Step 7: Verify Prometheus Metrics Endpoint
	respProm, err := client.Get(regcraftServer.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	promBody, _ := io.ReadAll(respProm.Body)
	_ = respProm.Body.Close()
	if !strings.Contains(string(promBody), "regcraft_cache_hits_total") {
		t.Errorf("expected Prometheus metrics, got: %s", string(promBody))
	}

	// Step 8: Trigger GC via API
	gcReq, _ := json.Marshal(map[string]interface{}{"dryRun": false})
	respGC, err := client.Post(regcraftServer.URL+"/api/v1/gc", "application/json", bytes.NewReader(gcReq))
	if err != nil {
		t.Fatalf("POST /api/v1/gc failed: %v", err)
	}
	var gcReport storage.GCReport
	_ = json.NewDecoder(respGC.Body).Decode(&gcReport)
	_ = respGC.Body.Close()

	if gcReport.FinalBytes < 0 {
		t.Errorf("unexpected final bytes: %d", gcReport.FinalBytes)
	}

	// Step 9: Delete Tag via API
	delReq, _ := http.NewRequest(http.MethodDelete, regcraftServer.URL+"/api/v1/catalog/library/postgres/tags/16", nil)
	respDel, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE tag failed: %v", err)
	}
	if respDel.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for tag deletion, got %d", respDel.StatusCode)
	}
	_ = respDel.Body.Close()
}
