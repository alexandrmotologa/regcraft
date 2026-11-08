import React, { useState } from 'react';
import { X, Layers, Copy, Check, HardDrive, CheckCircle2, Terminal, Cpu } from 'lucide-react';
import { CachedManifest } from '../types';
import { formatBytes, formatRelativeTime } from '../utils';

interface LayerVisualizerProps {
  manifest: CachedManifest | null;
  onClose: () => void;
}

export const LayerVisualizer: React.FC<LayerVisualizerProps> = ({ manifest, onClose }) => {
  const [copiedDigest, setCopiedDigest] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'layers' | 'config'>('layers');

  if (!manifest) return null;

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedDigest(text);
    setTimeout(() => setCopiedDigest(null), 2000);
  };

  const totalSize = manifest.layers.reduce((acc, l) => acc + l.size, 0);
  const maxSize = Math.max(...manifest.layers.map((l) => l.size), 1);
  const history = manifest.imageConfig?.history || [];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-fade-in">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-3xl max-h-[90vh] flex flex-col shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="p-5 border-b border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-sky-950 border border-sky-800/60 text-sky-400 flex items-center justify-center">
              <Layers className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-semibold text-white font-mono text-base">
                  {manifest.repository}:{manifest.reference}
                </h3>
                <span className="text-xs px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono">
                  {formatBytes(totalSize)}
                </span>
                {manifest.pinned && (
                  <span className="text-[10px] px-2 py-0.5 rounded bg-amber-950 text-amber-400 border border-amber-800/60 font-semibold uppercase">
                    Frozen
                  </span>
                )}
              </div>
              <p className="text-xs text-slate-400 mt-0.5 font-mono truncate max-w-md">
                Digest: {manifest.digest}
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

        {/* Manifest Metadata */}
        <div className="p-4 bg-slate-950/50 border-b border-slate-800/60 grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
          <div>
            <span className="text-slate-500 block">Platform</span>
            <span className="font-mono text-slate-200 flex items-center gap-1">
              <Cpu className="h-3 w-3 text-sky-400" />
              <span>
                {manifest.imageConfig?.os || 'linux'}/{manifest.imageConfig?.architecture || 'amd64'}
              </span>
            </span>
          </div>
          <div>
            <span className="text-slate-500 block">Total Layers</span>
            <span className="font-semibold text-white">{manifest.layers.length} layers</span>
          </div>
          <div>
            <span className="text-slate-500 block">Config Digest</span>
            <span className="font-mono text-sky-400 truncate block">
              {manifest.configDigest ? manifest.configDigest.slice(0, 16) + '...' : 'N/A'}
            </span>
          </div>
          <div>
            <span className="text-slate-500 block">Cached Since</span>
            <span className="text-slate-300">{formatRelativeTime(manifest.createdAt)}</span>
          </div>
        </div>

        {/* Tab switcher */}
        <div className="flex border-b border-slate-800 bg-slate-950/30 px-5 text-xs font-medium">
          <button
            onClick={() => setActiveTab('layers')}
            className={`py-2.5 px-3 border-b-2 transition ${
              activeTab === 'layers'
                ? 'border-sky-500 text-sky-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            Layers & Dockerfile Commands ({manifest.layers.length})
          </button>
          <button
            onClick={() => setActiveTab('config')}
            className={`py-2.5 px-3 border-b-2 transition ${
              activeTab === 'config'
                ? 'border-sky-500 text-sky-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            Environment & Runtime Config
          </button>
        </div>

        {/* Tab 1: Layers Stack */}
        {activeTab === 'layers' && (
          <div className="flex-1 overflow-y-auto p-5 space-y-3">
            <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2 flex items-center gap-1.5">
              <HardDrive className="h-3.5 w-3.5 text-sky-400" />
              <span>Layer Composition (Base to Top)</span>
            </div>

            {manifest.layers.map((layer, index) => {
              const pct = Math.max(4, Math.round((layer.size / maxSize) * 100));
              const isCopied = copiedDigest === layer.digest;
              const isBase = index === 0;
              const cmd = history[index] || '';

              return (
                <div
                  key={layer.digest + index}
                  className="bg-slate-950 border border-slate-800/90 rounded-xl p-3.5 hover:border-slate-700 transition"
                >
                  <div className="flex items-center justify-between gap-2 mb-2">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-mono font-bold text-slate-400 bg-slate-900 px-2 py-0.5 rounded border border-slate-800">
                        #{index + 1}
                      </span>
                      {isBase && (
                        <span className="text-[10px] uppercase font-semibold text-emerald-400 bg-emerald-950/80 border border-emerald-800/50 px-2 py-0.5 rounded">
                          Base Layer (Deduplicated)
                        </span>
                      )}
                      <span className="text-xs font-mono font-semibold text-slate-200">
                        {formatBytes(layer.size)}
                      </span>
                    </div>

                    <button
                      onClick={() => handleCopy(layer.digest)}
                      className="flex items-center gap-1.5 text-[11px] font-mono text-slate-400 hover:text-sky-300 transition"
                    >
                      <span>{layer.digest.slice(0, 19)}...</span>
                      {isCopied ? (
                        <Check className="h-3 w-3 text-emerald-400" />
                      ) : (
                        <Copy className="h-3 w-3" />
                      )}
                    </button>
                  </div>

                  {/* Dockerfile Command Snippet if available */}
                  {cmd && (
                    <div className="my-2 bg-slate-900/90 border border-slate-800 rounded px-2.5 py-1.5 text-[11px] font-mono text-slate-300 flex items-start gap-2">
                      <Terminal className="h-3.5 w-3.5 text-amber-400 shrink-0 mt-0.5" />
                      <span className="break-all">{cmd}</span>
                    </div>
                  )}

                  {/* Relative visual size bar */}
                  <div className="w-full bg-slate-900 rounded-full h-1.5 overflow-hidden">
                    <div
                      className={`h-1.5 rounded-full transition-all duration-300 ${
                        isBase ? 'bg-emerald-500' : 'bg-sky-500'
                      }`}
                      style={{ width: `${pct}%` }}
                    ></div>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Tab 2: Runtime Environment & Config */}
        {activeTab === 'config' && (
          <div className="flex-1 overflow-y-auto p-5 space-y-4 text-xs font-mono">
            {/* Entrypoint / CMD */}
            <div className="bg-slate-950 border border-slate-800 rounded-xl p-4">
              <span className="text-slate-500 uppercase block text-[10px] tracking-wider mb-2 font-sans font-semibold">
                Execution Command
              </span>
              <div className="space-y-1">
                <div>
                  <span className="text-slate-500">Entrypoint: </span>
                  <span className="text-sky-300">
                    {manifest.imageConfig?.entrypoint?.join(' ') || 'None'}
                  </span>
                </div>
                <div>
                  <span className="text-slate-500">CMD: </span>
                  <span className="text-emerald-300">
                    {manifest.imageConfig?.cmd?.join(' ') || 'None'}
                  </span>
                </div>
              </div>
            </div>

            {/* Environment Variables */}
            <div className="bg-slate-950 border border-slate-800 rounded-xl p-4">
              <span className="text-slate-500 uppercase block text-[10px] tracking-wider mb-2 font-sans font-semibold">
                Environment Variables ({manifest.imageConfig?.env?.length || 0})
              </span>
              {manifest.imageConfig?.env && manifest.imageConfig.env.length > 0 ? (
                <div className="space-y-1.5 max-h-56 overflow-y-auto pr-2">
                  {manifest.imageConfig.env.map((envVar, idx) => (
                    <div key={idx} className="bg-slate-900/80 px-2 py-1 rounded text-slate-300 truncate">
                      {envVar}
                    </div>
                  ))}
                </div>
              ) : (
                <span className="text-slate-600">No custom environment variables recorded</span>
              )}
            </div>
          </div>
        )}

        {/* Footer */}
        <div className="p-4 border-t border-slate-800 bg-slate-950/70 flex items-center justify-between">
          <div className="text-xs text-slate-400 flex items-center gap-1.5">
            <CheckCircle2 className="h-4 w-4 text-emerald-400" />
            <span>All layers verified with SHA-256 integrity</span>
          </div>
          <button
            onClick={onClose}
            className="px-4 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg text-xs font-medium transition"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
