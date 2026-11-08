package upstream

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/regcraft/internal/config"
	"github.com/alexandrmotologa/regcraft/internal/storage"
	"github.com/alexandrmotologa/regcraft/internal/telemetry"
)

// WarmResult reports the layers cached during a warm operation.
type WarmResult struct {
	Image            string        `json:"image"`
	LayersDownloaded int           `json:"layersDownloaded"`
	LayersSkipped    int           `json:"layersSkipped"`
	BytesDownloaded  int64         `json:"bytesDownloaded"`
	Duration         time.Duration `json:"duration"`
	Error            string        `json:"error,omitempty"`
}

// Warmer orchestrates concurrent pre-fetching of container images into CAS.
type Warmer struct {
	cfg      *config.Config
	store    *storage.Store
	client   *Client
	metrics  *telemetry.Metrics
	parallel int
}

// NewWarmer creates an image pre-warming coordinator.
func NewWarmer(cfg *config.Config, store *storage.Store, client *Client, metrics *telemetry.Metrics, parallel int) *Warmer {
	if parallel <= 0 {
		parallel = 4
	}
	return &Warmer{
		cfg:      cfg,
		store:    store,
		client:   client,
		metrics:  metrics,
		parallel: parallel,
	}
}

// Warm fetches an image manifest and all missing layers into local CAS.
func (w *Warmer) Warm(ctx context.Context, imageRef string) (*WarmResult, error) {
	startTime := time.Now()

	// Parse repo and tag
	parts := strings.Split(imageRef, ":")
	repo := parts[0]
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}

	resolved := ResolveRepository(repo, w.cfg.DefaultUpstream, w.cfg.UpstreamMirrors)

	telemetry.Events().Broadcast(telemetry.LiveEvent{
		Type:       telemetry.EventWarmProgress,
		Repository: repo,
		Reference:  tag,
		Message:    fmt.Sprintf("Pre-warming %s:%s...", repo, tag),
	})

	// 1. Fetch manifest
	manifestResult, err := w.client.FetchManifest(ctx, resolved, tag)
	if err != nil {
		return &WarmResult{Image: imageRef, Error: err.Error()}, fmt.Errorf("failed to fetch manifest: %w", err)
	}

	// 2. Save manifest
	cm, err := w.store.SaveManifest(repo, tag, manifestResult.MediaType, manifestResult.Payload)
	if err != nil {
		return &WarmResult{Image: imageRef, Error: err.Error()}, fmt.Errorf("failed to save manifest: %w", err)
	}

	// Collect blobs to fetch: config + layers
	var blobsToFetch []string
	if cm.ConfigDigest != "" && !w.store.HasBlob(cm.ConfigDigest) {
		blobsToFetch = append(blobsToFetch, cm.ConfigDigest)
	}

	var skippedCount int
	for _, l := range cm.Layers {
		if w.store.HasBlob(l.Digest) {
			skippedCount++
		} else {
			blobsToFetch = append(blobsToFetch, l.Digest)
		}
	}

	// Concurrently fetch missing blobs
	sem := make(chan struct{}, w.parallel)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var downloadedCount int
	var totalBytes int64
	var fetchErrors []string

	for _, digest := range blobsToFetch {
		wg.Add(1)
		sem <- struct{}{}

		go func(d string) {
			defer wg.Done()
			defer func() { <-sem }()

			body, _, _, err := w.client.FetchBlob(ctx, resolved, d)
			if err != nil {
				mu.Lock()
				fetchErrors = append(fetchErrors, fmt.Sprintf("blob %s: %v", d[:16], err))
				mu.Unlock()
				return
			}
			defer body.Close()

			n, err := w.store.WriteBlob(d, body)
			if err != nil {
				mu.Lock()
				fetchErrors = append(fetchErrors, fmt.Sprintf("save %s: %v", d[:16], err))
				mu.Unlock()
				return
			}

			mu.Lock()
			downloadedCount++
			totalBytes += n
			w.metrics.RecordMiss(n)
			mu.Unlock()

			telemetry.Events().Broadcast(telemetry.LiveEvent{
				Type:       telemetry.EventWarmProgress,
				Repository: repo,
				Digest:     d,
				Bytes:      n,
				Message:    fmt.Sprintf("Warmed %s layer %s (%s)", repo, d[:16], config.FormatByteSize(n)),
			})
		}(digest)
	}

	wg.Wait()

	result := &WarmResult{
		Image:            imageRef,
		LayersDownloaded: downloadedCount,
		LayersSkipped:    skippedCount,
		BytesDownloaded:  totalBytes,
		Duration:         time.Since(startTime),
	}

	if len(fetchErrors) > 0 {
		result.Error = strings.Join(fetchErrors, "; ")
		return result, fmt.Errorf("warm encountered errors: %s", result.Error)
	}

	telemetry.Events().Broadcast(telemetry.LiveEvent{
		Type:       telemetry.EventPullComplete,
		Repository: repo,
		Reference:  tag,
		Bytes:      totalBytes,
		Message:    fmt.Sprintf("Pre-warmed %s:%s successfully (%d layers, %s)", repo, tag, downloadedCount, config.FormatByteSize(totalBytes)),
	})

	return result, nil
}
