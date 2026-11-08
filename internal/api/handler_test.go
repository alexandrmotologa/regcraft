package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/alexandrmotologa/regcraft/internal/config"
	"github.com/alexandrmotologa/regcraft/internal/storage"
	"github.com/alexandrmotologa/regcraft/internal/telemetry"
)

func TestAPIEndpoints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regcraft-api-*")
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
	metrics.RecordHit(5000)

	apiHandler := NewHandler(cfg, store, metrics, "1.0.0")

	// 1. Status test
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	wStatus := httptest.NewRecorder()
	apiHandler.ServeHTTP(wStatus, reqStatus)

	if wStatus.Code != http.StatusOK {
		t.Errorf("status expected 200, got %d", wStatus.Code)
	}

	var statusResp map[string]interface{}
	_ = json.NewDecoder(wStatus.Body).Decode(&statusResp)
	if statusResp["version"] != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %v", statusResp["version"])
	}

	// 2. Metrics test
	reqMetrics := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	wMetrics := httptest.NewRecorder()
	apiHandler.ServeHTTP(wMetrics, reqMetrics)

	if wMetrics.Code != http.StatusOK {
		t.Errorf("metrics expected 200, got %d", wMetrics.Code)
	}

	var metricsResp telemetry.Snapshot
	_ = json.NewDecoder(wMetrics.Body).Decode(&metricsResp)
	if metricsResp.CacheHits != 1 {
		t.Errorf("expected 1 hit, got %d", metricsResp.CacheHits)
	}

	// 3. Upstreams test
	reqUp := httptest.NewRequest(http.MethodGet, "/api/v1/upstreams", nil)
	wUp := httptest.NewRecorder()
	apiHandler.ServeHTTP(wUp, reqUp)

	if wUp.Code != http.StatusOK {
		t.Errorf("upstreams expected 200, got %d", wUp.Code)
	}

	// 4. Catalog test
	reqCat := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	wCat := httptest.NewRecorder()
	apiHandler.ServeHTTP(wCat, reqCat)

	if wCat.Code != http.StatusOK {
		t.Errorf("catalog expected 200, got %d", wCat.Code)
	}
}
