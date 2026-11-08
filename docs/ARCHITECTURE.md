# Architecture Specification

regcraft is a lightweight pull-through cache proxy for container images. It implements the OCI Distribution Specification v1.1 and the Docker Registry HTTP API v2. It provides local caching, layer deduplication, and an inspection interface in a single Go binary under 30MB of RAM.

## Core Concepts

### Content Addressed Storage (CAS)
Every container layer and configuration object is identified by its SHA-256 cryptographic digest (for example, `sha256:717b09...`). 

Storage layout on disk:
```
<cache-dir>/
├── blobs/
│   └── sha256/
│       ├── 71/
│       │   └── 717b09...
│       └── a4/
│           └── a4d801...
├── manifests/
│   └── <repository>/
│       ├── tags/
│       │   └── <tag>.json
│       └── digests/
│           └── sha256_<hash>.json
├── tmp/
└── index.json
```

Layers are stored under `blobs/sha256/xx/xxhash...` using two-character directory partitioning to avoid filesystem inode congestion when thousands of blobs exist.

### Single Flight Streaming (Zero Intermediate Buffers)
When an image layer is requested:
1. If the blob exists in CAS, regcraft serves the file directly from disk using `http.ServeContent`, supporting range requests (`206 Partial Content`) and kernel level sendfile optimizations.
2. If the blob is missing:
   - regcraft opens a streaming HTTP connection to the upstream registry.
   - An `io.TeeReader` splits the incoming network stream into two paths:
     - Directly into the HTTP response writer to the Docker client.
     - Into a temporary file on disk, wrapped by a SHA-256 hasher.
   - When the stream ends, the computed digest is compared against the requested digest.
   - If the hash matches, the temporary file is moved atomically into its final content-addressed location.
   - If the client aborts or the upstream fails, the partial temporary file is deleted, ensuring no corrupted data enters the cache.

### Layer Deduplication
Many container images share identical base layers. For example, `golang:1.24`, `python:3.12`, and `node:22` based on Debian bookworm share the exact same Debian rootfs layers.

Because regcraft stores blobs by digest in a single unified CAS directory, shared layers are downloaded only once. A 1GB node image sharing 800MB of base layers with an already cached python image only downloads the unique 200MB diff.

### Manifest and Tag Handling
Manifests describe the layers that make up an image. Unlike blobs, tags are mutable:
- When a client requests a manifest by tag (e.g., `postgres:16`):
  - regcraft checks the upstream registry with a `HEAD` request, passing `If-None-Match: <etag>`.
  - If upstream returns `304 Not Modified`, regcraft serves the cached manifest without downloading it again.
  - If upstream returns a new manifest, regcraft updates its manifest cache and schedules missing layers for download.
  - If the upstream is unreachable (offline network, disconnected branch office), regcraft serves the cached manifest directly, allowing offline image pulls.
- When a client requests a manifest by digest (`sha256:...`):
  - Digest references are immutable. If the digest manifest is in the local store, regcraft returns it immediately with zero upstream roundtrips.

### Multi-Architecture Fat Manifests
Modern images are published as multi-architecture manifest lists (`application/vnd.docker.distribution.manifest.list.v2+json`) or OCI image indexes (`application/vnd.oci.image.index.v1+json`).

When a client queries a multi-arch tag:
1. regcraft fetches and caches the root image index.
2. The client inspects the index, matches its architecture (such as `linux/amd64` or `linux/arm64`), and requests the child manifest by digest.
3. regcraft resolves the platform-specific child manifest and streams the respective architecture layers.

### Automated Garbage Collection (LRU Sweeper)
regcraft tracks the last access time of every manifest and blob:
1. Disk usage is monitored continuously.
2. When storage exceeds the configured quota (default 20GB):
   - The garbage collector identifies all active manifests.
   - It builds a reference graph of all blobs referenced by active manifests.
   - It identifies orphan blobs not referenced by any tag.
   - It evicts the least recently accessed images until disk usage drops below the target watermark (typically 85% of quota).
3. The GC can also be triggered manually through the CLI (`regcraft prune`) or the Web UI.
