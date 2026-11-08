package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GCReport describes the outcome of a garbage collection sweep.
type GCReport struct {
	InitialBytes     int64     `json:"initialBytes"`
	FinalBytes       int64     `json:"finalBytes"`
	ReclaimedBytes   int64     `json:"reclaimedBytes"`
	InitialBlobs     int       `json:"initialBlobs"`
	FinalBlobs       int       `json:"finalBlobs"`
	DeletedBlobs     int       `json:"deletedBlobs"`
	DeletedManifests int       `json:"deletedManifests"`
	DryRun           bool      `json:"dryRun"`
	Timestamp        time.Time `json:"timestamp"`
}

// DiskUsage computes the total bytes occupied by cached blobs.
func (s *Store) DiskUsage() (int64, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalBytes int64
	var count int

	err := filepath.Walk(s.blobsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalBytes += info.Size()
			count++
		}
		return nil
	})

	return totalBytes, count, err
}

// tagEntry tracks tag metadata for LRU sorting.
type tagEntry struct {
	repo         string
	tag          string
	path         string
	lastAccessed time.Time
	digest       string
	layers       []string
}

// RunGC executes automated LRU garbage collection.
func (s *Store) RunGC(quotaBytes int64, dryRun bool) (*GCReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	initialBytes, initialBlobs, err := s.diskUsageLocked()
	if err != nil {
		return nil, fmt.Errorf("failed to compute disk usage: %w", err)
	}

	report := &GCReport{
		InitialBytes: initialBytes,
		InitialBlobs: initialBlobs,
		DryRun:       dryRun,
		Timestamp:    time.Now().UTC(),
	}

	// 1. Scan all manifest files and collect referenced blob digests
	referencedBlobs := make(map[string]int) // digestHex -> refCount
	var tagEntries []tagEntry

	err = filepath.Walk(s.manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var cm CachedManifest
		if err := json.Unmarshal(data, &cm); err != nil {
			return nil
		}

		// Track config
		if cm.ConfigDigest != "" {
			if h, err := CanonicalDigest(cm.ConfigDigest); err == nil {
				referencedBlobs[h]++
			}
		}

		// Track layers
		var layerDigests []string
		for _, l := range cm.Layers {
			if h, err := CanonicalDigest(l.Digest); err == nil {
				referencedBlobs[h]++
				layerDigests = append(layerDigests, h)
			}
		}

		// If this is a tag file, add to LRU candidate pool
		if strings.Contains(path, filepath.Join("tags", "")) {
			rel, _ := filepath.Rel(s.manifestsDir, path)
			parts := strings.Split(filepath.ToSlash(rel), "/")
			if len(parts) >= 3 {
				repo := strings.Join(parts[:len(parts)-2], "/")
				tag := strings.TrimSuffix(parts[len(parts)-1], ".json")
				tagEntries = append(tagEntries, tagEntry{
					repo:         repo,
					tag:          tag,
					path:         path,
					lastAccessed: cm.LastAccessed,
					digest:       cm.Digest,
					layers:       layerDigests,
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking manifests: %w", err)
	}

	// 2. Identify orphan blobs (blobs on disk not referenced by any manifest)
	var orphanBlobs []string
	_ = filepath.Walk(s.blobsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		digestHex := filepath.Base(path)
		if _, ok := referencedBlobs[digestHex]; !ok {
			orphanBlobs = append(orphanBlobs, path)
		}
		return nil
	})

	// Delete orphan blobs
	for _, blobPath := range orphanBlobs {
		info, err := os.Stat(blobPath)
		if err == nil {
			report.ReclaimedBytes += info.Size()
			report.DeletedBlobs++
			if !dryRun {
				_ = os.Remove(blobPath)
			}
		}
	}

	currentUsage := initialBytes - report.ReclaimedBytes

	// 3. If still over quota, evict oldest tags (LRU)
	if currentUsage > quotaBytes {
		// Sort tags oldest first
		sort.Slice(tagEntries, func(i, j int) bool {
			return tagEntries[i].lastAccessed.Before(tagEntries[j].lastAccessed)
		})

		targetUsage := int64(float64(quotaBytes) * 0.85)

		for _, te := range tagEntries {
			if currentUsage <= targetUsage {
				break
			}

			// Decrement refcounts for this tag's layers
			for _, lDigest := range te.layers {
				referencedBlobs[lDigest]--
				if referencedBlobs[lDigest] <= 0 {
					blobPath := s.blobPath(lDigest)
					if info, err := os.Stat(blobPath); err == nil {
						report.ReclaimedBytes += info.Size()
						report.DeletedBlobs++
						currentUsage -= info.Size()
						if !dryRun {
							_ = os.Remove(blobPath)
						}
					}
				}
			}

			report.DeletedManifests++
			if !dryRun {
				_ = os.Remove(te.path)
			}
		}
	}

	report.FinalBytes = initialBytes - report.ReclaimedBytes
	report.FinalBlobs = initialBlobs - report.DeletedBlobs

	return report, nil
}

// diskUsageLocked calculates blob usage while mutex is held.
func (s *Store) diskUsageLocked() (int64, int, error) {
	var totalBytes int64
	var count int

	err := filepath.Walk(s.blobsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalBytes += info.Size()
			count++
		}
		return nil
	})

	return totalBytes, count, err
}
