package test

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/regcraft/internal/config"
	"github.com/alexandrmotologa/regcraft/internal/oci"
	"github.com/alexandrmotologa/regcraft/internal/storage"
	"github.com/alexandrmotologa/regcraft/internal/telemetry"
	"github.com/alexandrmotologa/regcraft/internal/upstream"
)

func TestWarmerAndTagPinning(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regcraft-warm-pin-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	layerData := []byte("layer-to-be-prewarmed-into-cas")
	lh := sha256.Sum256(layerData)
	layerDigest := "sha256:" + hex.EncodeToString(lh[:])

	configData := []byte(`{"architecture":"amd64","os":"linux"}`)
	ch := sha256.Sum256(configData)
	configDigest := "sha256:" + hex.EncodeToString(ch[:])

	manifestPayload := []byte(fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {"mediaType": "application/vnd.docker.container.image.v1+json", "size": %d, "digest": "%s"},
		"layers": [{"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip", "size": %d, "digest": "%s"}]
	}`, len(configData), configDigest, len(layerData), layerDigest))

	upstreamCalls := 0
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if r.URL.Path == "/v2/library/redis/manifests/7" {
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Header().Set("Docker-Content-Digest", "sha256:5555555555555555555555555555555555555555555555555555555555555555")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(manifestPayload)
			return
		}

		if r.URL.Path == "/v2/library/redis/blobs/"+configDigest {
			w.Header().Set("Content-Type", "application/vnd.docker.container.image.v1+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(configData)
			return
		}

		if r.URL.Path == "/v2/library/redis/blobs/"+layerDigest {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(layerData)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(layerData)
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
		t.Fatalf("failed to create store: %v", err)
	}

	metrics := telemetry.NewMetrics()
	client := upstream.NewClient(5 * time.Second)
	warmer := upstream.NewWarmer(cfg, store, client, metrics, 2)

	// 1. Test Warmer
	res, err := warmer.Warm(context.Background(), "library/redis:7")
	if err != nil {
		t.Fatalf("warmer.Warm failed: %v", err)
	}

	if res.LayersDownloaded != 2 {
		t.Errorf("expected 2 blobs downloaded (config + layer), got %d", res.LayersDownloaded)
	}
	if !store.HasBlob(layerDigest) {
		t.Fatalf("blob should be in CAS after warm")
	}

	// 2. Test Tag Pinning
	if err := store.SetTagPin("library/redis", "7", true); err != nil {
		t.Fatalf("SetTagPin failed: %v", err)
	}

	cm, err := store.GetManifest("library/redis", "7")
	if err != nil || !cm.Pinned {
		t.Fatalf("manifest should be flagged as pinned")
	}

	// Test OCI router serves pinned tag without calling upstream
	ociHandler := oci.NewHandler(cfg, store, client, metrics)
	callsBefore := upstreamCalls

	req := httptest.NewRequest(http.MethodGet, "/v2/library/redis/manifests/7", nil)
	w := httptest.NewRecorder()
	ociHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK for pinned tag, got %d", w.Code)
	}
	if upstreamCalls != callsBefore {
		t.Errorf("pinned tag should NOT call upstream, but upstreamCalls increased from %d to %d", callsBefore, upstreamCalls)
	}
}

func TestExportImportTarLifecycle(t *testing.T) {
	tempSrc, _ := os.MkdirTemp("", "regcraft-export-src-*")
	tempDst, _ := os.MkdirTemp("", "regcraft-export-dst-*")
	defer os.RemoveAll(tempSrc)
	defer os.RemoveAll(tempDst)

	storeSrc, _ := storage.NewStore(tempSrc)
	storeDst, _ := storage.NewStore(tempDst)

	// Write blob and manifest into source
	blob := []byte("exportable layer bytes")
	h := sha256.Sum256(blob)
	digest := "sha256:" + hex.EncodeToString(h[:])
	_, _ = storeSrc.WriteBlob(digest, bytes.NewReader(blob))

	manifestJSON := []byte(fmt.Sprintf(`{"schemaVersion":2,"layers":[{"digest":"%s","size":%d}]}`, digest, len(blob)))
	_, _ = storeSrc.SaveManifest("library/myapp", "v1", "application/vnd.oci.image.manifest.v1+json", manifestJSON)

	// Create a tar in memory
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	// Write manifest to tar
	cm, _ := storeSrc.GetManifest("library/myapp", "v1")
	mData, _ := json.Marshal(cm)
	_ = tw.WriteHeader(&tar.Header{
		Name: "manifests/library/myapp/tags/v1.json",
		Size: int64(len(mData)),
		Mode: 0644,
	})
	_, _ = tw.Write(mData)

	// Write blob to tar
	digestHex, _ := storage.CanonicalDigest(digest)
	_ = tw.WriteHeader(&tar.Header{
		Name: filepath.ToSlash(filepath.Join("blobs", "sha256", digestHex[:2], digestHex)),
		Size: int64(len(blob)),
		Mode: 0644,
	})
	_, _ = tw.Write(blob)
	_ = tw.Close()

	// Simulate import
	tr := tar.NewReader(&tarBuf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}

		if filepath.Base(filepath.Dir(hdr.Name)) == digestHex[:2] {
			w, err := storeDst.NewBlobWriter(digest)
			if err != nil {
				t.Fatalf("NewBlobWriter failed: %v", err)
			}
			_, _ = io.Copy(w, tr)
			_ = w.Commit()
		} else if filepath.Ext(hdr.Name) == ".json" {
			data, _ := io.ReadAll(tr)
			var m storage.CachedManifest
			_ = json.Unmarshal(data, &m)
			_, _ = storeDst.SaveManifest(m.Repository, m.Reference, m.MediaType, m.Payload)
		}
	}

	// Verify imported state
	if !storeDst.HasBlob(digest) {
		t.Errorf("imported store missing blob %s", digest)
	}
	importedManifest, err := storeDst.GetManifest("library/myapp", "v1")
	if err != nil || importedManifest == nil {
		t.Errorf("imported store missing manifest")
	}
}
