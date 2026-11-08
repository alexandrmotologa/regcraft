package telemetry

import (
	"bytes"
	"strings"
	"testing"
)

func TestMetricsTracking(t *testing.T) {
	m := NewMetrics()

	m.RecordHit(1024)
	m.RecordHit(2048)
	m.RecordMiss(4096)
	m.RecordManifestHit()
	m.IncActivePulls()

	snap := m.GetSnapshot()

	if snap.CacheHits != 2 {
		t.Errorf("expected 2 hits, got %d", snap.CacheHits)
	}
	if snap.CacheMisses != 1 {
		t.Errorf("expected 1 miss, got %d", snap.CacheMisses)
	}
	if snap.BytesServedCached != 3072 {
		t.Errorf("expected 3072 cached bytes, got %d", snap.BytesServedCached)
	}
	if snap.BytesFetchedUpstream != 4096 {
		t.Errorf("expected 4096 upstream bytes, got %d", snap.BytesFetchedUpstream)
	}
	if snap.ActivePulls != 1 {
		t.Errorf("expected 1 active pull, got %d", snap.ActivePulls)
	}
	if snap.TotalPulls != 3 {
		t.Errorf("expected 3 total pulls, got %d", snap.TotalPulls)
	}
	expectedRatio := (2.0 / 3.0) * 100.0
	if snap.HitRatio < expectedRatio-0.1 || snap.HitRatio > expectedRatio+0.1 {
		t.Errorf("expected hit ratio ~%.1f, got %.1f", expectedRatio, snap.HitRatio)
	}

	var buf bytes.Buffer
	m.WritePrometheus(&buf)
	promText := buf.String()

	if !strings.Contains(promText, "regcraft_cache_hits_total 2") {
		t.Errorf("Prometheus output missing hits: %s", promText)
	}
	if !strings.Contains(promText, "regcraft_bytes_served_cached_total 3072") {
		t.Errorf("Prometheus output missing cached bytes: %s", promText)
	}
}
