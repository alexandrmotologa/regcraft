import React from 'react';
import { Layers, Activity, Cpu, HardDrive, RefreshCw } from 'lucide-react';
import { DaemonStatus } from '../types';
import { formatUptime } from '../utils';

interface HeaderProps {
  status: DaemonStatus | null;
  onRefresh: () => void;
  onOpenGC: () => void;
}

export const Header: React.FC<HeaderProps> = ({ status, onRefresh, onOpenGC }) => {
  const ramMB = status ? (status.memoryAllocBytes / (1024 * 1024)).toFixed(1) : '0';

  return (
    <header className="border-b border-slate-800 bg-slate-900/80 backdrop-blur sticky top-0 z-40">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-gradient-to-tr from-sky-600 to-cyan-400 flex items-center justify-center shadow-lg shadow-sky-500/20">
            <Layers className="h-6 w-6 text-white" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-bold text-lg tracking-tight text-white font-mono">regcraft</span>
              <span className="text-xs px-2 py-0.5 rounded-full bg-sky-950 text-sky-400 border border-sky-800/60 font-mono">
                v{status?.version || '1.0.0'}
              </span>
            </div>
            <p className="text-xs text-slate-400 hidden sm:block">
              OCI v1.1 Pull-Through Registry Cache & Multi-Arch Inspector
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 sm:gap-4">
          {/* Status pill */}
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-emerald-950/60 text-emerald-400 border border-emerald-800/40 text-xs font-medium">
            <span className="h-2 w-2 rounded-full bg-emerald-400 animate-pulse"></span>
            <span className="hidden md:inline">Status:</span>
            <span>{status?.status || 'Online'}</span>
          </div>

          {/* Memory pill */}
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-800 text-slate-300 border border-slate-700 text-xs font-mono">
            <Cpu className="h-3.5 w-3.5 text-sky-400" />
            <span>{ramMB} MB RAM</span>
            <span className="text-[10px] text-slate-500">(&lt; 30MB)</span>
          </div>

          {/* Uptime pill */}
          <div className="hidden lg:flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-800 text-slate-300 border border-slate-700 text-xs font-mono">
            <Activity className="h-3.5 w-3.5 text-amber-400" />
            <span>Up {status ? formatUptime(status.uptimeSeconds) : '0s'}</span>
          </div>

          {/* Refresh button */}
          <button
            onClick={onRefresh}
            title="Refresh Metrics"
            className="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition"
          >
            <RefreshCw className="h-4 w-4" />
          </button>

          {/* GC Button */}
          <button
            onClick={onOpenGC}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 hover:border-slate-600 text-xs font-medium transition"
          >
            <HardDrive className="h-3.5 w-3.5 text-indigo-400" />
            <span>Prune</span>
          </button>

          <a
            href="https://github.com/alexandrmotologa/regcraft"
            target="_blank"
            rel="noreferrer"
            className="p-2 text-slate-400 hover:text-white transition rounded-lg hover:bg-slate-800"
            title="GitHub Repository"
          >
            <svg className="h-4 w-4 fill-current" viewBox="0 0 24 24">
              <path fillRule="evenodd" clipRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
            </svg>
          </a>
        </div>
      </div>
    </header>
  );
};
