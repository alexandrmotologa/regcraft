import React, { useState } from 'react';
import { HardDrive, Trash2, CheckCircle2, AlertCircle, RefreshCw, X } from 'lucide-react';
import { GCReport, DaemonStatus } from '../types';
import { formatBytes } from '../utils';

interface PruneConsoleProps {
  status: DaemonStatus | null;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export const PruneConsole: React.FC<PruneConsoleProps> = ({ status, isOpen, onClose, onSuccess }) => {
  const [dryRun, setDryRun] = useState(false);
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState<GCReport | null>(null);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleRunGC = async () => {
    setLoading(true);
    setError(null);
    setReport(null);

    try {
      const resp = await fetch('/api/v1/gc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dryRun, quotaBytes: status?.quotaBytes }),
      });

      if (!resp.ok) {
        throw new Error(`GC failed with HTTP ${resp.status}`);
      }

      const data: GCReport = await resp.json();
      setReport(data);
      if (!dryRun) {
        onSuccess();
      }
    } catch (err: any) {
      setError(err.message || 'Failed to execute garbage collection');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-fade-in">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-xl shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="p-5 border-b border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-indigo-950 border border-indigo-800/60 text-indigo-400 flex items-center justify-center">
              <HardDrive className="h-5 w-5" />
            </div>
            <div>
              <h3 className="font-semibold text-white text-base">Storage & LRU Prune Console</h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Reclaim storage by evicting unreferenced layers and stale images
              </p>
            </div>
          </div>

          <button
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-5">
          {/* Current Storage Meter */}
          <div className="bg-slate-950/70 border border-slate-800 rounded-xl p-4">
            <div className="flex items-center justify-between text-xs mb-2">
              <span className="text-slate-400">Current Storage Used</span>
              <span className="font-mono text-slate-200">
                {formatBytes(status?.storageUsedBytes || 0)} / {formatBytes(status?.quotaBytes || 0)}
              </span>
            </div>
            <div className="w-full bg-slate-900 rounded-full h-2 overflow-hidden">
              <div
                className="bg-indigo-500 h-2 rounded-full transition-all duration-300"
                style={{
                  width: `${Math.min(
                    100,
                    Math.round(((status?.storageUsedBytes || 0) / (status?.quotaBytes || 1)) * 100)
                  )}%`,
                }}
              ></div>
            </div>
            <div className="mt-3 flex items-center justify-between text-[11px] text-slate-500">
              <span>{status?.blobCount || 0} Blobs in CAS</span>
              <span>{status?.repoCount || 0} Repositories Cached</span>
            </div>
          </div>

          {/* Options */}
          <div className="space-y-3">
            <label className="flex items-center gap-3 cursor-pointer p-3 rounded-lg bg-slate-950/50 border border-slate-800/80 hover:border-slate-700 transition">
              <input
                type="checkbox"
                checked={dryRun}
                onChange={(e) => setDryRun(e.target.checked)}
                className="rounded border-slate-700 text-sky-500 focus:ring-sky-500 bg-slate-900"
              />
              <div className="text-xs">
                <span className="font-medium text-slate-200 block">Dry Run Mode</span>
                <span className="text-slate-500 block">
                  Simulate garbage collection without deleting any files from disk
                </span>
              </div>
            </label>
          </div>

          {/* Error Message */}
          {error && (
            <div className="p-3 bg-rose-950/50 border border-rose-800 text-rose-300 rounded-lg text-xs flex items-center gap-2">
              <AlertCircle className="h-4 w-4 shrink-0 text-rose-400" />
              <span>{error}</span>
            </div>
          )}

          {/* Success / Report Display */}
          {report && (
            <div className="p-4 bg-emerald-950/40 border border-emerald-800/60 rounded-xl space-y-2">
              <div className="flex items-center gap-2 text-xs font-semibold text-emerald-400">
                <CheckCircle2 className="h-4 w-4" />
                <span>
                  {report.dryRun ? 'Dry Run Simulation Complete' : 'Garbage Collection Finished'}
                </span>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs text-slate-300 mt-2">
                <div>
                  <span className="text-slate-500 block">Reclaimed Space:</span>
                  <span className="font-mono font-bold text-emerald-300">
                    {formatBytes(report.reclaimedBytes)}
                  </span>
                </div>
                <div>
                  <span className="text-slate-500 block">Deleted Blobs:</span>
                  <span className="font-mono font-bold text-white">{report.deletedBlobs}</span>
                </div>
                <div>
                  <span className="text-slate-500 block">Deleted Manifests:</span>
                  <span className="font-mono text-white">{report.deletedManifests}</span>
                </div>
                <div>
                  <span className="text-slate-500 block">Final Storage:</span>
                  <span className="font-mono text-white">{formatBytes(report.finalBytes)}</span>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-3">
          <button
            onClick={onClose}
            className="px-4 py-2 text-xs font-medium text-slate-400 hover:text-white transition"
          >
            Cancel
          </button>
          <button
            onClick={handleRunGC}
            disabled={loading}
            className="flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white rounded-lg text-xs font-medium transition shadow-lg shadow-indigo-600/20"
          >
            {loading ? (
              <RefreshCw className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Trash2 className="h-3.5 w-3.5" />
            )}
            <span>{loading ? 'Sweeping Storage...' : dryRun ? 'Simulate Prune' : 'Run Prune Now'}</span>
          </button>
        </div>
      </div>
    </div>
  );
};
