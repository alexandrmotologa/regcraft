import React from 'react';
import { Zap, CheckCircle2, ArrowDownCircle, Flame } from 'lucide-react';
import { LiveEvent } from '../types';

interface LiveActivityBarProps {
  lastEvent: LiveEvent | null;
}

export const LiveActivityBar: React.FC<LiveActivityBarProps> = ({ lastEvent }) => {
  if (!lastEvent) {
    return (
      <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl px-4 py-2.5 flex items-center justify-between text-xs text-slate-500">
        <div className="flex items-center gap-2">
          <span className="h-2 w-2 rounded-full bg-slate-600"></span>
          <span>Live Activity: Listening for container pull requests...</span>
        </div>
        <span className="font-mono text-[11px] text-slate-600">SSE Stream Active</span>
      </div>
    );
  }

  const isHit = lastEvent.type === 'cache_hit';
  const isComplete = lastEvent.type === 'pull_complete';
  const isWarm = lastEvent.type === 'warm_progress';

  return (
    <div
      className={`border rounded-xl px-4 py-2.5 flex items-center justify-between text-xs transition-all duration-300 ${
        isHit
          ? 'bg-emerald-950/30 border-emerald-800/60 text-emerald-300'
          : isWarm
          ? 'bg-amber-950/30 border-amber-800/60 text-amber-300'
          : isComplete
          ? 'bg-sky-950/30 border-sky-800/60 text-sky-300'
          : 'bg-slate-900 border-slate-800 text-slate-300'
      }`}
    >
      <div className="flex items-center gap-2.5 truncate mr-3">
        {isHit ? (
          <Zap className="h-4 w-4 text-emerald-400 shrink-0" />
        ) : isWarm ? (
          <Flame className="h-4 w-4 text-amber-400 shrink-0 animate-pulse" />
        ) : isComplete ? (
          <CheckCircle2 className="h-4 w-4 text-sky-400 shrink-0" />
        ) : (
          <ArrowDownCircle className="h-4 w-4 text-slate-400 shrink-0" />
        )}
        <span className="font-medium truncate">{lastEvent.message}</span>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        <span className="h-2 w-2 rounded-full bg-emerald-400 animate-ping"></span>
        <span className="font-mono text-[11px] text-slate-400">
          {new Date(lastEvent.timestamp).toLocaleTimeString()}
        </span>
      </div>
    </div>
  );
};
