package telemetry

import (
	"fmt"
	"io"
	"runtime"
	"sync/atomic"
	"time"
)

// Metrics records operational statistics for regcraft.
type Metrics struct {
	startTime              time.Time
	cacheHits              atomic.Uint64
	cacheMisses            atomic.Uint64
	bytesServedCached      atomic.Uint64
	bytesFetchedUpstream   atomic.Uint64
	activePulls            atomic.Int64
	manifestsServedCached  atomic.Uint64
	manifestsFetchedRemote atomic.Uint64
}

// Global metrics instance.
var defaultMetrics = NewMetrics()

// Default returns the default global metrics registry.
func Default() *Metrics {
	return defaultMetrics
}

// NewMetrics initializes a new telemetry tracker.
func NewMetrics() *Metrics {
	return &Metrics{
		startTime: time.Now(),
	}
}

// RecordHit increments cache hit counters and records delivered bytes.
func (m *Metrics) RecordHit(bytes int64) {
	m.cacheHits.Add(1)
	if bytes > 0 {
		m.bytesServedCached.Add(uint64(bytes))
	}
}

// RecordMiss increments cache miss counters and records upstream downloaded bytes.
func (m *Metrics) RecordMiss(bytes int64) {
	m.cacheMisses.Add(1)
	if bytes > 0 {
		m.bytesFetchedUpstream.Add(uint64(bytes))
	}
}

// RecordManifestHit increments the manifest cache hit counter.
func (m *Metrics) RecordManifestHit() {
	m.manifestsServedCached.Add(1)
}

// RecordManifestMiss increments the remote manifest query counter.
func (m *Metrics) RecordManifestMiss() {
	m.manifestsFetchedRemote.Add(1)
}

// IncActivePulls increments the gauge of currently active blob streams.
func (m *Metrics) IncActivePulls() {
	m.activePulls.Add(1)
}

// DecActivePulls decrements the gauge of currently active blob streams.
func (m *Metrics) DecActivePulls() {
	m.activePulls.Add(-1)
}

// Snapshot provides a point-in-time copy of metrics.
type Snapshot struct {
	UptimeSeconds          int64   `json:"uptimeSeconds"`
	CacheHits              uint64  `json:"cacheHits"`
	CacheMisses            uint64  `json:"cacheMisses"`
	TotalPulls             uint64  `json:"totalPulls"`
	HitRatio               float64 `json:"hitRatio"`
	BytesServedCached      uint64  `json:"bytesServedCached"`
	BytesFetchedUpstream   uint64  `json:"bytesFetchedUpstream"`
	BandwidthSavedBytes    uint64  `json:"bandwidthSavedBytes"`
	ActivePulls            int64   `json:"activePulls"`
	ManifestsServedCached  uint64  `json:"manifestsServedCached"`
	ManifestsFetchedRemote uint64  `json:"manifestsFetchedRemote"`
	RateLimitsAvoided      uint64  `json:"rateLimitsAvoided"`
	MemoryAllocBytes       uint64  `json:"memoryAllocBytes"`
	MemorySysBytes         uint64  `json:"memorySysBytes"`
	NumGoroutine           int     `json:"numGoroutine"`
}

// GetSnapshot returns current metrics as a Snapshot struct.
func (m *Metrics) GetSnapshot() Snapshot {
	hits := m.cacheHits.Load()
	misses := m.cacheMisses.Load()
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total) * 100.0
	}

	cachedBytes := m.bytesServedCached.Load()
	upstreamBytes := m.bytesFetchedUpstream.Load()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return Snapshot{
		UptimeSeconds:          int64(time.Since(m.startTime).Seconds()),
		CacheHits:              hits,
		CacheMisses:            misses,
		TotalPulls:             total,
		HitRatio:               hitRatio,
		BytesServedCached:      cachedBytes,
		BytesFetchedUpstream:   upstreamBytes,
		BandwidthSavedBytes:    cachedBytes,
		ActivePulls:            m.activePulls.Load(),
		ManifestsServedCached:  m.manifestsServedCached.Load(),
		ManifestsFetchedRemote: m.manifestsFetchedRemote.Load(),
		RateLimitsAvoided:      hits + m.manifestsServedCached.Load(),
		MemoryAllocBytes:       mem.Alloc,
		MemorySysBytes:         mem.Sys,
		NumGoroutine:           runtime.NumGoroutine(),
	}
}

// WritePrometheus formats metrics into standard Prometheus exposition syntax.
func (m *Metrics) WritePrometheus(w io.Writer) {
	snap := m.GetSnapshot()

	fmt.Fprintf(w, "# HELP regcraft_uptime_seconds Total running time in seconds\n")
	fmt.Fprintf(w, "# TYPE regcraft_uptime_seconds gauge\n")
	fmt.Fprintf(w, "regcraft_uptime_seconds %d\n\n", snap.UptimeSeconds)

	fmt.Fprintf(w, "# HELP regcraft_cache_hits_total Number of requests served from cache\n")
	fmt.Fprintf(w, "# TYPE regcraft_cache_hits_total counter\n")
	fmt.Fprintf(w, "regcraft_cache_hits_total %d\n\n", snap.CacheHits)

	fmt.Fprintf(w, "# HELP regcraft_cache_misses_total Number of requests fetched from upstream\n")
	fmt.Fprintf(w, "# TYPE regcraft_cache_misses_total counter\n")
	fmt.Fprintf(w, "regcraft_cache_misses_total %d\n\n", snap.CacheMisses)

	fmt.Fprintf(w, "# HELP regcraft_bytes_served_cached_total Total bytes delivered from local cache\n")
	fmt.Fprintf(w, "# TYPE regcraft_bytes_served_cached_total counter\n")
	fmt.Fprintf(w, "regcraft_bytes_served_cached_total %d\n\n", snap.BytesServedCached)

	fmt.Fprintf(w, "# HELP regcraft_bytes_fetched_upstream_total Total bytes fetched from upstream registries\n")
	fmt.Fprintf(w, "# TYPE regcraft_bytes_fetched_upstream_total counter\n")
	fmt.Fprintf(w, "regcraft_bytes_fetched_upstream_total %d\n\n", snap.BytesFetchedUpstream)

	fmt.Fprintf(w, "# HELP regcraft_active_pulls Current active concurrent layer transfers\n")
	fmt.Fprintf(w, "# TYPE regcraft_active_pulls gauge\n")
	fmt.Fprintf(w, "regcraft_active_pulls %d\n\n", snap.ActivePulls)

	fmt.Fprintf(w, "# HELP regcraft_memory_alloc_bytes Bytes of allocated heap objects\n")
	fmt.Fprintf(w, "# TYPE regcraft_memory_alloc_bytes gauge\n")
	fmt.Fprintf(w, "regcraft_memory_alloc_bytes %d\n\n", snap.MemoryAllocBytes)
}
