import React from 'react';
import { Zap, Percent, ShieldAlert, Database, ArrowDownCircle } from 'lucide-react';
import { MetricsSnapshot, DaemonStatus } from '../types';
import { formatBytes } from '../utils';

interface BandwidthGaugesProps {
  metrics: MetricsSnapshot | null;
  status: DaemonStatus | null;
}

export const BandwidthGauges: React.FC<BandwidthGaugesProps> = ({ metrics, status }) => {
  const savedBytes = metrics?.bandwidthSavedBytes || 0;
  const upstreamBytes = metrics?.bytesFetchedUpstream || 0;
  const hitRatio = metrics?.hitRatio ? metrics.hitRatio.toFixed(1) : '0.0';
  const rateLimitsAvoided = metrics?.rateLimitsAvoided || 0;

  const usedBytes = status?.storageUsedBytes || 0;
  const quotaBytes = status?.quotaBytes || 21474836480; // 20GB default
  const quotaPercentage = Math.min(100, Math.round((usedBytes / quotaBytes) * 100));

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      {/* 1. Bandwidth Saved */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm relative overflow-hidden">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-slate-400">Bandwidth Saved (SSD)</span>
          <div className="h-8 w-8 rounded-lg bg-sky-950 text-sky-400 flex items-center justify-center">
            <Zap className="h-4 w-4" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="text-2xl font-bold font-mono text-white tracking-tight">
            {formatBytes(savedBytes)}
          </span>
          <span className="text-xs text-sky-400 font-mono">10 Gbps</span>
        </div>
        <p className="mt-2 text-xs text-slate-500 flex items-center gap-1.5">
          <ArrowDownCircle className="h-3 w-3 text-emerald-400" />
          <span>Upstream fetched: {formatBytes(upstreamBytes)}</span>
        </p>
      </div>

      {/* 2. Cache Hit Ratio */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm relative overflow-hidden">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-slate-400">Cache Hit Ratio</span>
          <div className="h-8 w-8 rounded-lg bg-emerald-950 text-emerald-400 flex items-center justify-center">
            <Percent className="h-4 w-4" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="text-2xl font-bold font-mono text-white tracking-tight">
            {hitRatio}%
          </span>
          <span className="text-xs text-slate-400">
            ({metrics?.cacheHits || 0} / {metrics?.totalPulls || 0} pulls)
          </span>
        </div>
        <div className="mt-3 w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
          <div
            className="bg-emerald-500 h-1.5 rounded-full transition-all duration-500"
            style={{ width: `${hitRatio}%` }}
          ></div>
        </div>
      </div>

      {/* 3. Docker Hub Rate Limits Avoided */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm relative overflow-hidden">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-slate-400">429 Rate Limits Avoided</span>
          <div className="h-8 w-8 rounded-lg bg-amber-950 text-amber-400 flex items-center justify-center">
            <ShieldAlert className="h-4 w-4" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="text-2xl font-bold font-mono text-white tracking-tight">
            {rateLimitsAvoided}
          </span>
          <span className="text-xs text-amber-400/90 font-mono">requests saved</span>
        </div>
        <p className="mt-2 text-xs text-slate-500">
          Zero Docker Hub 100-pulls/6h throttling
        </p>
      </div>

      {/* 4. Storage Quota */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm relative overflow-hidden">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-slate-400">Local CAS Storage</span>
          <div className="h-8 w-8 rounded-lg bg-indigo-950 text-indigo-400 flex items-center justify-center">
            <Database className="h-4 w-4" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="text-2xl font-bold font-mono text-white tracking-tight">
            {formatBytes(usedBytes)}
          </span>
          <span className="text-xs text-slate-400">/ {formatBytes(quotaBytes)}</span>
        </div>
        <div className="mt-3 w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
          <div
            className={`h-1.5 rounded-full transition-all duration-500 ${
              quotaPercentage > 85 ? 'bg-rose-500' : quotaPercentage > 60 ? 'bg-amber-500' : 'bg-indigo-500'
            }`}
            style={{ width: `${quotaPercentage}%` }}
          ></div>
        </div>
      </div>
    </div>
  );
};
