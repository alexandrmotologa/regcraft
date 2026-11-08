import React, { useEffect, useState, useCallback } from 'react';
import { Header } from './components/Header';
import { BandwidthGauges } from './components/BandwidthGauges';
import { ImageCatalog } from './components/ImageCatalog';
import { LayerVisualizer } from './components/LayerVisualizer';
import { PruneConsole } from './components/PruneConsole';
import { UpstreamStatus } from './components/UpstreamStatus';
import { DaemonStatus, MetricsSnapshot, RepoSummary, UpstreamItem, CachedManifest } from './types';
import { Terminal, AlertTriangle } from 'lucide-react';

export const App: React.FC = () => {
  const [status, setStatus] = useState<DaemonStatus | null>(null);
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [repos, setRepos] = useState<RepoSummary[]>([]);
  const [upstreams, setUpstreams] = useState<UpstreamItem[]>([]);
  const [selectedManifest, setSelectedManifest] = useState<CachedManifest | null>(null);
  const [isGCOpen, setIsGCOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      const [statusRes, metricsRes, catalogRes, upstreamsRes] = await Promise.all([
        fetch('/api/v1/status').then((r) => (r.ok ? r.json() : null)),
        fetch('/api/v1/metrics').then((r) => (r.ok ? r.json() : null)),
        fetch('/api/v1/catalog').then((r) => (r.ok ? r.json() : [])),
        fetch('/api/v1/upstreams').then((r) => (r.ok ? r.json() : [])),
      ]);

      if (statusRes) setStatus(statusRes);
      if (metricsRes) setMetrics(metricsRes);
      if (catalogRes) setRepos(catalogRes);
      if (upstreamsRes) setUpstreams(upstreamsRes);
      setError(null);
    } catch (err: any) {
      console.error('Error fetching regcraft API data:', err);
      setError('Unable to connect to regcraft daemon. Ensure the server is running on :5000.');
    }
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 3000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const handleInspect = async (repo: string, tag: string) => {
    try {
      const res = await fetch(`/api/v1/catalog/${repo}/tags/${tag}`);
      if (res.ok) {
        const data = await res.json();
        setSelectedManifest(data);
      }
    } catch (err) {
      console.error('Failed to load manifest layers:', err);
    }
  };

  const handleDeleteTag = async (repo: string, tag: string) => {
    if (!window.confirm(`Delete tag ${repo}:${tag} from local cache?`)) return;

    try {
      const res = await fetch(`/api/v1/catalog/${repo}/tags/${tag}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        await fetchData();
      }
    } catch (err) {
      console.error('Failed to delete tag:', err);
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
      <Header
        status={status}
        onRefresh={fetchData}
        onOpenGC={() => setIsGCOpen(true)}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
        {error && (
          <div className="p-4 bg-rose-950/60 border border-rose-800 text-rose-300 rounded-xl text-xs flex items-center justify-between">
            <div className="flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 shrink-0 text-rose-400" />
              <span>{error}</span>
            </div>
            <button
              onClick={fetchData}
              className="px-2.5 py-1 bg-rose-900/60 hover:bg-rose-800 rounded font-medium text-white transition"
            >
              Retry
            </button>
          </div>
        )}

        {/* Live Gauges */}
        <BandwidthGauges metrics={metrics} status={status} />

        {/* Docker Mirror Quick Snippet */}
        <div className="bg-gradient-to-r from-sky-950/40 via-slate-900 to-indigo-950/40 border border-slate-800 rounded-xl p-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 text-xs">
          <div className="flex items-center gap-2.5 text-slate-300">
            <Terminal className="h-4 w-4 text-sky-400 shrink-0" />
            <span>
              Configured as registry mirror in <code className="text-sky-300 bg-slate-950 px-1.5 py-0.5 rounded border border-slate-800">/etc/docker/daemon.json</code>:
            </span>
          </div>
          <div className="font-mono text-sky-400 bg-slate-950 px-3 py-1.5 rounded-lg border border-slate-800 select-all w-full sm:w-auto text-center sm:text-left">
            "registry-mirrors": ["http://localhost:5000"]
          </div>
        </div>

        {/* Cached Images Catalog */}
        <ImageCatalog
          repos={repos}
          onInspect={handleInspect}
          onDeleteTag={handleDeleteTag}
          listenHost={status?.cacheDir ? 'localhost:5000' : 'localhost:5000'}
        />

        {/* Upstream Status */}
        <UpstreamStatus upstreams={upstreams} />
      </main>

      {/* Layer Visualizer Modal */}
      <LayerVisualizer
        manifest={selectedManifest}
        onClose={() => setSelectedManifest(null)}
      />

      {/* Prune / GC Console Modal */}
      <PruneConsole
        status={status}
        isOpen={isGCOpen}
        onClose={() => setIsGCOpen(false)}
        onSuccess={fetchData}
      />

      <footer className="border-t border-slate-900 py-6 text-center text-xs text-slate-600">
        <p>regcraft • High-Speed OCI Registry & Multi-Arch Container Image Caching Proxy</p>
      </footer>
    </div>
  );
};
export default App;
