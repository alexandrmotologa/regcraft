package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestCASBlobLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regcraft-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	payload := []byte("hello regcraft high performance oci layer")
	h := sha256.Sum256(payload)
	digest := "sha256:" + hex.EncodeToString(h[:])

	// Initial check: blob should not exist
	if store.HasBlob(digest) {
		t.Errorf("expected HasBlob to return false before write")
	}

	// Write blob
	n, err := store.WriteBlob(digest, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}
	if n != int64(len(payload)) {
		t.Errorf("expected %d bytes written, got %d", len(payload), n)
	}

	// Verify existence
	if !store.HasBlob(digest) {
		t.Errorf("expected HasBlob to return true after write")
	}

	// Read blob back
	f, size, err := store.OpenBlob(digest)
	if err != nil {
		t.Fatalf("OpenBlob failed: %v", err)
	}
	defer f.Close()

	if size != int64(len(payload)) {
		t.Errorf("expected size %d, got %d", len(payload), size)
	}

	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, payload) {
		t.Errorf("content mismatch: got %q, expected %q", string(content), string(payload))
	}
}

func TestCASDigestMismatch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regcraft-mismatch-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	payload := []byte("correct content")
	corruptDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	_, err = store.WriteBlob(corruptDigest, bytes.NewReader(payload))
	if err == nil {
		t.Errorf("expected WriteBlob to fail on digest mismatch, but got nil")
	}

	if store.HasBlob(corruptDigest) {
		t.Errorf("corrupt blob should not have been stored in CAS")
	}
}

func TestManifestSaveAndGet(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regcraft-manifest-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	layerData := []byte("dummy layer")
	lh := sha256.Sum256(layerData)
	layerDigest := "sha256:" + hex.EncodeToString(lh[:])

	manifestJSON := []byte(fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"size": 1234,
			"digest": "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		},
		"layers": [
			{
				"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
				"size": %d,
				"digest": "%s"
			}
		]
	}`, len(layerData), layerDigest))

	repo := "library/redis"
	tag := "7.0"

	cm, err := store.SaveManifest(repo, tag, "application/vnd.docker.distribution.manifest.v2+json", manifestJSON)
	if err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	if len(cm.Layers) != 1 {
		t.Fatalf("expected 1 layer parsed, got %d", len(cm.Layers))
	}
	if cm.Layers[0].Digest != layerDigest {
		t.Errorf("expected layer digest %s, got %s", layerDigest, cm.Layers[0].Digest)
	}

	// Fetch by tag
	fetchedTag, err := store.GetManifest(repo, tag)
	if err != nil {
		t.Fatalf("GetManifest by tag failed: %v", err)
	}
	if fetchedTag.Digest != cm.Digest {
		t.Errorf("expected digest %s, got %s", cm.Digest, fetchedTag.Digest)
	}

	// Fetch by digest
	fetchedDigest, err := store.GetManifest(repo, cm.Digest)
	if err != nil {
		t.Fatalf("GetManifest by digest failed: %v", err)
	}
	if fetchedDigest.Reference != tag {
		t.Errorf("expected reference %s, got %s", tag, fetchedDigest.Reference)
	}

	// List repos and tags
	repos := store.ListRepositories()
	if len(repos) != 1 || repos[0] != repo {
		t.Errorf("expected repos [%s], got %v", repo, repos)
	}

	tags := store.ListTags(repo)
	if len(tags) != 1 || tags[0] != tag {
		t.Errorf("expected tags [%s], got %v", tag, tags)
	}
}

func TestGarbageCollection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regcraft-gc-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Write active layer
	activeBlob := []byte("active blob data")
	ah := sha256.Sum256(activeBlob)
	activeDigest := "sha256:" + hex.EncodeToString(ah[:])
	_, _ = store.WriteBlob(activeDigest, bytes.NewReader(activeBlob))

	// Write orphan blob (unreferenced)
	orphanBlob := []byte("orphan blob that should be deleted")
	oh := sha256.Sum256(orphanBlob)
	orphanDigest := "sha256:" + hex.EncodeToString(oh[:])
	_, _ = store.WriteBlob(orphanDigest, bytes.NewReader(orphanBlob))

	// Create manifest referencing only activeBlob
	manifestJSON := []byte(fmt.Sprintf(`{
		"schemaVersion": 2,
		"layers": [{"digest": "%s", "size": %d}]
	}`, activeDigest, len(activeBlob)))

	_, err = store.SaveManifest("test/app", "latest", "application/vnd.oci.image.manifest.v1+json", manifestJSON)
	if err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	// Run GC (quota high, should just sweep orphan)
	report, err := store.RunGC(1024*1024, false)
	if err != nil {
		t.Fatalf("RunGC failed: %v", err)
	}

	if report.DeletedBlobs != 1 {
		t.Errorf("expected 1 deleted blob, got %d", report.DeletedBlobs)
	}
	if report.ReclaimedBytes != int64(len(orphanBlob)) {
		t.Errorf("expected %d reclaimed bytes, got %d", len(orphanBlob), report.ReclaimedBytes)
	}

	// Verify active blob remains, orphan is gone
	if !store.HasBlob(activeDigest) {
		t.Errorf("active blob was erroneously deleted")
	}
	if store.HasBlob(orphanDigest) {
		t.Errorf("orphan blob was not deleted")
	}
}
