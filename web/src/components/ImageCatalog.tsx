import React, { useState } from 'react';
import { Search, Copy, Check, Trash2, Eye, Box, Clock } from 'lucide-react';
import { RepoSummary } from '../types';
import { formatBytes, formatRelativeTime, shortDigest } from '../utils';

interface ImageCatalogProps {
  repos: RepoSummary[];
  onInspect: (repo: string, tag: string) => void;
  onDeleteTag: (repo: string, tag: string) => void;
  listenHost?: string;
}

export const ImageCatalog: React.FC<ImageCatalogProps> = ({
  repos,
  onInspect,
  onDeleteTag,
  listenHost = 'localhost:5000',
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [copiedPull, setCopiedPull] = useState<string | null>(null);

  const filteredRepos = repos.filter(
    (r) =>
      r.repository.toLowerCase().includes(searchTerm.toLowerCase()) ||
      r.tags.some((t) => t.name.toLowerCase().includes(searchTerm.toLowerCase()))
  );

  const handleCopyPull = (repo: string, tag: string) => {
    const cmd = `docker pull ${listenHost}/${repo}:${tag}`;
    navigator.clipboard.writeText(cmd);
    setCopiedPull(`${repo}:${tag}`);
    setTimeout(() => setCopiedPull(null), 2000);
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
      {/* Header & Search */}
      <div className="p-4 sm:p-5 border-b border-slate-800 flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
        <div className="flex items-center gap-2">
          <Box className="h-5 w-5 text-sky-400" />
          <h2 className="font-semibold text-white">Cached Container Images</h2>
          <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 font-mono">
            {repos.length} repositories
          </span>
        </div>

        <div className="relative w-full sm:w-72">
          <Search className="h-4 w-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
          <input
            type="text"
            placeholder="Filter images or tags..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg pl-9 pr-3 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-sky-500 transition"
          />
        </div>
      </div>

      {/* Repositories list */}
      {filteredRepos.length === 0 ? (
        <div className="p-12 text-center text-slate-500">
          <Box className="h-10 w-10 mx-auto text-slate-600 mb-3" />
          <p className="text-sm font-medium text-slate-400">No cached images found</p>
          <p className="text-xs mt-1 text-slate-500 max-w-sm mx-auto">
            Pull any image via <code className="text-sky-400 font-mono">docker pull {listenHost}/alpine:latest</code> to populate the local SSD cache.
          </p>
        </div>
      ) : (
        <div className="divide-y divide-slate-800/80">
          {filteredRepos.map((repo) => (
            <div key={repo.repository} className="p-4 sm:p-5 hover:bg-slate-800/30 transition">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-3">
                <div className="flex items-center gap-2">
                  <span className="font-mono text-sm font-semibold text-white tracking-wide">
                    {repo.repository}
                  </span>
                  <span className="text-xs text-slate-500 font-mono">
                    Total: {formatBytes(repo.totalSize)}
                  </span>
                </div>
                <div className="flex items-center gap-2 text-xs text-slate-500">
                  <Clock className="h-3.5 w-3.5" />
                  <span>Last active: {formatRelativeTime(repo.lastAccessed)}</span>
                </div>
              </div>

              {/* Tags grid */}
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2.5">
                {repo.tags.map((tag) => {
                  const pullKey = `${repo.repository}:${tag.name}`;
                  const isCopied = copiedPull === pullKey;

                  return (
                    <div
                      key={tag.name}
                      className="bg-slate-950/70 border border-slate-800 rounded-lg p-3 flex flex-col justify-between hover:border-slate-700 transition"
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div>
                          <div className="flex items-center gap-1.5">
                            <span className="text-xs font-bold text-sky-300 font-mono px-2 py-0.5 rounded bg-sky-950/80 border border-sky-800/50">
                              :{tag.name}
                            </span>
                            <span className="text-[11px] text-slate-400 font-mono">
                              {formatBytes(tag.totalSize)}
                            </span>
                          </div>
                          <div className="mt-1.5 text-[11px] text-slate-500 flex items-center gap-2">
                            <span>{tag.layerCount} layers</span>
                            <span>•</span>
                            <span className="font-mono">{shortDigest(tag.digest)}</span>
                          </div>
                        </div>

                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => onInspect(repo.repository, tag.name)}
                            title="Inspect Layers"
                            className="p-1.5 text-slate-400 hover:text-sky-300 hover:bg-slate-800 rounded transition"
                          >
                            <Eye className="h-3.5 w-3.5" />
                          </button>
                          <button
                            onClick={() => onDeleteTag(repo.repository, tag.name)}
                            title="Delete Cached Tag"
                            className="p-1.5 text-slate-400 hover:text-rose-400 hover:bg-slate-800 rounded transition"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      </div>

                      {/* Docker pull snippet */}
                      <button
                        onClick={() => handleCopyPull(repo.repository, tag.name)}
                        className="mt-3 w-full flex items-center justify-between px-2.5 py-1.5 rounded bg-slate-900 border border-slate-800/80 hover:border-sky-700/60 text-slate-300 hover:text-white text-[11px] font-mono group transition"
                      >
                        <span className="truncate mr-2 text-slate-400 group-hover:text-slate-200">
                          docker pull {listenHost}/{repo.repository}:{tag.name}
                        </span>
                        {isCopied ? (
                          <Check className="h-3.5 w-3.5 text-emerald-400 shrink-0" />
                        ) : (
                          <Copy className="h-3.5 w-3.5 text-slate-500 group-hover:text-sky-400 shrink-0" />
                        )}
                      </button>
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
