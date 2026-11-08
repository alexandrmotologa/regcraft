package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OCI descriptor representation.
type descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

// standardManifest models Docker and OCI v1 manifests.
type standardManifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        descriptor   `json:"config"`
	Layers        []descriptor `json:"layers"`
	Manifests     []descriptor `json:"manifests"` // For Image Indexes / Manifest Lists
}

// ParseManifestLayers extracts layer and config descriptors from raw JSON payload.
func ParseManifestLayers(payload []byte) (configDigest string, layers []LayerRef, err error) {
	var m standardManifest
	if err := json.Unmarshal(payload, &m); err != nil {
		return "", nil, err
	}

	configDigest = m.Config.Digest

	// Single image manifest layers
	for _, l := range m.Layers {
		layers = append(layers, LayerRef{
			Digest:    l.Digest,
			Size:      l.Size,
			MediaType: l.MediaType,
		})
	}

	// Multi-arch image index / manifest list items
	for _, sub := range m.Manifests {
		layers = append(layers, LayerRef{
			Digest:    sub.Digest,
			Size:      sub.Size,
			MediaType: sub.MediaType,
		})
	}

	return configDigest, layers, nil
}

// sanitizeRepo converts repository names into safe directory paths.
func sanitizeRepo(repo string) string {
	clean := strings.Trim(repo, "/")
	clean = strings.ReplaceAll(clean, "..", "")
	return filepath.Clean(clean)
}

// SaveManifest persists an image manifest and records its metadata.
func (s *Store) SaveManifest(repo, reference, mediaType string, payload []byte) (*CachedManifest, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("cannot save empty manifest payload")
	}

	h := sha256.Sum256(payload)
	digestHex := hex.EncodeToString(h[:])
	fullDigest := "sha256:" + digestHex

	configDigest, layers, _ := ParseManifestLayers(payload)

	now := time.Now().UTC()
	cm := &CachedManifest{
		Repository:   repo,
		Reference:    reference,
		Digest:       fullDigest,
		MediaType:    mediaType,
		Size:         int64(len(payload)),
		Payload:      payload,
		Layers:       layers,
		ConfigDigest: configDigest,
		CreatedAt:    now,
		LastAccessed: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	repoDir := filepath.Join(s.manifestsDir, sanitizeRepo(repo))
	digestsDir := filepath.Join(repoDir, "digests")
	tagsDir := filepath.Join(repoDir, "tags")

	if err := os.MkdirAll(digestsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repo digests directory: %w", err)
	}
	if err := os.MkdirAll(tagsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repo tags directory: %w", err)
	}

	data, err := json.MarshalIndent(cm, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize manifest metadata: %w", err)
	}

	// Always write by digest
	digestFile := filepath.Join(digestsDir, digestHex+".json")
	if err := os.WriteFile(digestFile, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write digest manifest: %w", err)
	}

	// If reference is a tag (not a sha256 digest), save tag pointer
	if !strings.HasPrefix(reference, "sha256:") {
		safeTag := strings.ReplaceAll(reference, "/", "_")
		tagFile := filepath.Join(tagsDir, safeTag+".json")
		if err := os.WriteFile(tagFile, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write tag manifest: %w", err)
		}
	}

	return cm, nil
}

// GetManifest retrieves a manifest by tag or digest.
func (s *Store) GetManifest(repo, reference string) (*CachedManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	repoDir := filepath.Join(s.manifestsDir, sanitizeRepo(repo))
	var targetFile string

	if strings.HasPrefix(reference, "sha256:") {
		digestHex, err := CanonicalDigest(reference)
		if err != nil {
			return nil, err
		}
		targetFile = filepath.Join(repoDir, "digests", digestHex+".json")
	} else {
		safeTag := strings.ReplaceAll(reference, "/", "_")
		targetFile = filepath.Join(repoDir, "tags", safeTag+".json")
	}

	data, err := os.ReadFile(targetFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	var cm CachedManifest
	if err := json.Unmarshal(data, &cm); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached manifest: %w", err)
	}

	// Touch last accessed time
	cm.LastAccessed = time.Now().UTC()
	updatedData, _ := json.MarshalIndent(&cm, "", "  ")
	_ = os.WriteFile(targetFile, updatedData, 0644)

	// Load and attach config & Dockerfile history if config blob exists in CAS
	if cm.ConfigDigest != "" {
		if digestHex, err := CanonicalDigest(cm.ConfigDigest); err == nil {
			configPath := s.blobPath(digestHex)
			if f, err := os.Open(configPath); err == nil {
				if cfgInfo, err := ParseImageConfigBlob(f); err == nil {
					cm.ImageConfig = cfgInfo
				}
				_ = f.Close()
			}
		}
	}

	return &cm, nil
}

// SetTagPin toggles immutability lock on a cached tag.
func (s *Store) SetTagPin(repo, tag string, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	safeTag := strings.ReplaceAll(tag, "/", "_")
	tagFile := filepath.Join(s.manifestsDir, sanitizeRepo(repo), "tags", safeTag+".json")

	data, err := os.ReadFile(tagFile)
	if err != nil {
		return fmt.Errorf("tag %s:%s not found in cache: %w", repo, tag, err)
	}

	var cm CachedManifest
	if err := json.Unmarshal(data, &cm); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	cm.Pinned = pinned
	updated, err := json.MarshalIndent(&cm, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tagFile, updated, 0644)
}

// DeleteTag removes a specific tag file for a repository.
func (s *Store) DeleteTag(repo, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	safeTag := strings.ReplaceAll(tag, "/", "_")
	tagFile := filepath.Join(s.manifestsDir, sanitizeRepo(repo), "tags", safeTag+".json")

	err := os.Remove(tagFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListRepositories scans the manifests folder and returns all cached repository names.
func (s *Store) ListRepositories() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var repos []string
	_ = filepath.Walk(s.manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		tagsDir := filepath.Join(path, "tags")
		digestsDir := filepath.Join(path, "digests")
		if _, err1 := os.Stat(tagsDir); err1 == nil {
			if _, err2 := os.Stat(digestsDir); err2 == nil {
				rel, err := filepath.Rel(s.manifestsDir, path)
				if err == nil && rel != "." {
					repos = append(repos, filepath.ToSlash(rel))
				}
			}
		}
		return nil
	})

	return repos
}

// ListTags returns all cached tag names for a given repository.
func (s *Store) ListTags(repo string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tagsDir := filepath.Join(s.manifestsDir, sanitizeRepo(repo), "tags")
	entries, err := os.ReadDir(tagsDir)
	if err != nil {
		return nil
	}

	var tags []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			tag := strings.TrimSuffix(entry.Name(), ".json")
			tags = append(tags, tag)
		}
	}

	return tags
}
