export interface DaemonStatus {
  status: string;
  version: string;
  uptimeSeconds: number;
  cacheDir: string;
  quotaBytes: number;
  storageUsedBytes: number;
  blobCount: number;
  repoCount: number;
  memoryAllocBytes: number;
  memorySysBytes: number;
  numGoroutine: number;
  offlineMode: boolean;
}

export interface MetricsSnapshot {
  uptimeSeconds: number;
  cacheHits: number;
  cacheMisses: number;
  totalPulls: number;
  hitRatio: number;
  bytesServedCached: number;
  bytesFetchedUpstream: number;
  bandwidthSavedBytes: number;
  activePulls: number;
  manifestsServedCached: number;
  manifestsFetchedRemote: number;
  rateLimitsAvoided: number;
  memoryAllocBytes: number;
  memorySysBytes: number;
  numGoroutine: number;
}

export interface UpstreamItem {
  name: string;
  endpoint: string;
  isDefault: boolean;
}

export interface TagSummary {
  name: string;
  digest: string;
  mediaType: string;
  totalSize: number;
  layerCount: number;
  lastAccessed: string;
}

export interface RepoSummary {
  repository: string;
  tags: TagSummary[];
  totalSize: number;
  lastAccessed: string;
}

export interface LayerRef {
  digest: string;
  size: number;
  mediaType: string;
}

export interface CachedManifest {
  repository: string;
  reference: string;
  digest: string;
  mediaType: string;
  size: number;
  layers: LayerRef[];
  configDigest?: string;
  createdAt: string;
  lastAccessed: string;
}

export interface GCReport {
  initialBytes: number;
  finalBytes: number;
  reclaimedBytes: number;
  initialBlobs: number;
  finalBlobs: number;
  deletedBlobs: number;
  deletedManifests: number;
  dryRun: boolean;
  timestamp: string;
}
