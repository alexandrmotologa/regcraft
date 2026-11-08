import React from 'react';
import { Globe, CheckCircle2 } from 'lucide-react';
import { UpstreamItem } from '../types';

interface UpstreamStatusProps {
  upstreams: UpstreamItem[];
}

export const UpstreamStatus: React.FC<UpstreamStatusProps> = ({ upstreams }) => {
  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Globe className="h-5 w-5 text-sky-400" />
          <h2 className="font-semibold text-white">Upstream Registry Mirrors</h2>
        </div>
        <span className="text-xs text-slate-500">Auto token negotiation</span>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {upstreams.map((item) => (
          <div
            key={item.name}
            className="bg-slate-950/70 border border-slate-800 rounded-lg p-3 flex flex-col justify-between"
          >
            <div>
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs font-bold text-white font-mono">{item.name}</span>
                {item.isDefault && (
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-sky-950 text-sky-400 border border-sky-800/50 font-medium">
                    Default Hub
                  </span>
                )}
              </div>
              <p className="text-[11px] text-slate-500 font-mono truncate" title={item.endpoint}>
                {item.endpoint}
              </p>
            </div>

            <div className="mt-3 flex items-center gap-1.5 text-[11px] text-emerald-400">
              <CheckCircle2 className="h-3.5 w-3.5" />
              <span>Ready for pull-through</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
