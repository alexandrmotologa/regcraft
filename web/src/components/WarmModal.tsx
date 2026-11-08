import React, { useState } from 'react';
import { Flame, CheckCircle2, AlertCircle, RefreshCw, X } from 'lucide-react';
import { WarmResult } from '../types';
import { formatBytes } from '../utils';

interface WarmModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const POPULAR_IMAGES = [
  'alpine:latest',
  'postgres:16',
  'redis:7',
  'node:22-alpine',
  'golang:1.24',
  'python:3.12-slim',
];

export const WarmModal: React.FC<WarmModalProps> = ({ isOpen, onClose, onSuccess }) => {
  const [imageInput, setImageInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<WarmResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleWarm = async (imageToWarm?: string) => {
    const target = imageToWarm || imageInput.trim();
    if (!target) return;

    setLoading(true);
    setError(null);
    setResult(null);

    try {
      const resp = await fetch('/api/v1/warm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ image: target }),
      });

      const data: WarmResult = await resp.json();
      if (!resp.ok) {
        throw new Error(data.error || `HTTP ${resp.status}`);
      }

      setResult(data);
      onSuccess();
    } catch (err: any) {
      setError(err.message || 'Failed to pre-warm image');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-fade-in">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="p-5 border-b border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-amber-950 border border-amber-800/60 text-amber-400 flex items-center justify-center">
              <Flame className="h-5 w-5" />
            </div>
            <div>
              <h3 className="font-semibold text-white text-base">Pre-Warm Image Cache</h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Download and verify all layers into local SSD before developers or CI pull
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

        {/* Form Body */}
        <div className="p-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">
              Container Image Reference
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                placeholder="e.g. library/postgres:16 or ghcr.io/astral-sh/uv:latest"
                value={imageInput}
                onChange={(e) => setImageInput(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleWarm()}
                disabled={loading}
                className="flex-1 bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs font-mono text-slate-100 placeholder-slate-500 focus:outline-none focus:border-amber-500 transition"
              />
              <button
                onClick={() => handleWarm()}
                disabled={loading || !imageInput.trim()}
                className="px-4 py-2 bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-white rounded-lg text-xs font-medium transition flex items-center gap-2"
              >
                {loading ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : <Flame className="h-3.5 w-3.5" />}
                <span>{loading ? 'Warming...' : 'Pre-Warm'}</span>
              </button>
            </div>
          </div>

          {/* Quick suggestions */}
          <div>
            <span className="text-[11px] text-slate-500 block mb-2">Quick popular base images:</span>
            <div className="flex flex-wrap gap-1.5">
              {POPULAR_IMAGES.map((img) => (
                <button
                  key={img}
                  onClick={() => {
                    setImageInput(img);
                    handleWarm(img);
                  }}
                  disabled={loading}
                  className="px-2 py-1 rounded bg-slate-950 border border-slate-800 hover:border-amber-700/60 text-slate-400 hover:text-amber-300 text-[11px] font-mono transition"
                >
                  +{img}
                </button>
              ))}
            </div>
          </div>

          {/* Error Banner */}
          {error && (
            <div className="p-3 bg-rose-950/50 border border-rose-800 text-rose-300 rounded-lg text-xs flex items-center gap-2">
              <AlertCircle className="h-4 w-4 shrink-0 text-rose-400" />
              <span>{error}</span>
            </div>
          )}

          {/* Result Banner */}
          {result && (
            <div className="p-4 bg-emerald-950/40 border border-emerald-800/60 rounded-xl space-y-2">
              <div className="flex items-center gap-2 text-xs font-semibold text-emerald-400">
                <CheckCircle2 className="h-4 w-4" />
                <span>Image {result.image} pre-warmed successfully</span>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs text-slate-300 mt-2">
                <div>
                  <span className="text-slate-500 block">Downloaded Layers:</span>
                  <span className="font-mono font-bold text-white">{result.layersDownloaded}</span>
                </div>
                <div>
                  <span className="text-slate-500 block">Deduplicated (Skipped):</span>
                  <span className="font-mono font-bold text-emerald-400">{result.layersSkipped}</span>
                </div>
                <div>
                  <span className="text-slate-500 block">Network Data:</span>
                  <span className="font-mono font-bold text-sky-400">{formatBytes(result.bytesDownloaded)}</span>
                </div>
                <div>
                  <span className="text-slate-500 block">Time Elapsed:</span>
                  <span className="font-mono text-slate-300">
                    {(result.duration / 1000000).toFixed(0)} ms
                  </span>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-slate-800 bg-slate-950/80 flex items-center justify-between text-xs text-slate-500">
          <span>Subsequent pulls on this machine will deliver at 10 Gbps SSD speed</span>
          <button
            onClick={onClose}
            className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
