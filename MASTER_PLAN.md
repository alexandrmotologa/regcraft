# regcraft: Architecture & Engineering Master Plan

> High-Speed Pull-Through OCI Registry & Multi-Arch Container Image Caching Proxy (< 30MB RAM)
> Harbor and Nexus Alternative in a Single Binary (Go + React 19)

---

## 1. Executive Summary and Problem Space

### The Core Problem
Container images (postgres:16, node:20-alpine, golang:1.24, python:3.12) are standard building blocks. Fetching them repeatedly causes three recurring operational problems:
1. **Docker Hub Rate Limits (HTTP 429)**: Unauthenticated developer machines and shared office or VPN IP addresses hit Docker Hub's 100 pulls per 6 hours limit, blocking local builds and CI/CD pipelines.
2. **Bandwidth and Latency Overhead**: Downloading identical base layers across many developer machines and CI runners saturates internet uplinks and slows build iterations.
3. **Heavyweight Self-Hosted Registries**: Traditional solutions like Harbor or Nexus require PostgreSQL, Redis, Java runtime environments, and multiple background containers consuming gigabytes of idle RAM.

### The Solution: regcraft
regcraft is a lightweight, single-binary OCI-compliant pull-through caching proxy written in Go with an embedded React 19 inspection cockpit. It listens on port 5000, implementing the OCI Distribution Specification v1.1 and Docker Registry HTTP API v2.

On the first pull, regcraft streams image layers from Docker Hub, GitHub Container Registry (ghcr.io), Quay, or private registries while simultaneously saving layers to local content-addressed storage. Subsequent pulls on any local machine or CI runner resolve directly from local NVMe or SSD storage at line speed with zero external network roundtrips and zero Docker Hub rate limits, operating under 30MB of RAM.

---

## 2. System Architecture and Caching Pipeline

```
[ Docker CLI / Podman / Kubernetes containerd (docker pull localhost:5000/library/postgres:16) ]
                               │
                               │  GET /v2/library/postgres/manifests/16
                               ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        regcraft Proxy Daemon                           │
│                                                                        │
│  ┌───────────────────────┐         ┌────────────────────────────────┐  │
│  │ OCI Spec v1.1 Router  │ ──────> │ Manifest Cache & Upstream ETag │  │
│  │ (Auth & Path Parser)  │         │ Validator (Checks for updates) │  │
│  └───────────────────────┘         └────────────────┬───────────────┘  │
│                                                     │                  │
│                                                     ▼                  │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Layer Resolution Engine (GET /v2/.../blobs/sha256:<digest>)      │  │
│  └────────────────────────┬─────────────────────────────────────────┘  │
│               Cache HIT   │                     │ Cache MISS           │
│       ┌───────────────────┘                     ▼                      │
│       │                    ┌────────────────────────────────────────┐  │
│       │                    │ Upstream Fetcher (Docker Hub / GHCR)   │  │
│       │                    └────────────────────┬───────────────────┘  │
│       │                                         │ Streaming TeeReader  │
│       │                         ┌───────────────┴───────────────┐      │
│       │                         ▼                               ▼      │
│       │              ┌──────────────────────┐      ┌─────────────────┐ │
│       │              │ Content-Addressed    │      │ Stream directly │ │
│       │              │ Local Storage (SSD)  │      │ to Docker client│ │
│       │              └──────────────────────┘      └────────┬────────┘ │
│       ▼                                                     ▼          │
│  [ High-Speed Local SSD Stream: 10 Gbps, sub-millisecond layer delivery ]
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Automated Garbage Collection & Deduplication Engine              │  │
│  │ - Deduplicates identical base layers across multiple images      │  │
│  │ - LRU Eviction sweeps unreferenced layers when quota is reached  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Embedded React 19 Layer Inspector                                │  │
│  │ - Visual Layer Treemap, Image Ancestry, 1-Click Prune Console    │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Technology Stack

| Layer | Component | Specification |
| :--- | :--- | :--- |
| **Language & Runtime** | Go 1.24+ | Zero-copy streaming I/O (`io.Copy`), minimal memory footprint (< 30MB RAM) |
| **Protocol Compliance**| OCI Distribution Spec v1.1 | Native support for Docker Image Manifest v2, OCI Image Index, multi-arch manifests |
| **Storage Engine** | Content-Addressed Storage | Stores layers under `blobs/sha256/<hash>` with cross-repository deduplication |
| **Upstream Providers** | Multi-Registry Proxy | Docker Hub, GitHub Container Registry (ghcr.io), Quay.io, custom registries |
| **Embedded Web UI** | React 19 + TypeScript + Vite | Built as a single bundle and served via Go `//go:embed` |
| **UI Styling** | Tailwind CSS + Lucide Icons | Clean modern dark mode DevOps cockpit |
| **Metrics & Health** | Prometheus & REST API | `/metrics` and `/api/v1/*` endpoints for monitoring and telemetry |

---

## 4. Repository and Directory Structure

```
B:\workgit\regcraft\
├── cmd\
│   └── regcraft\
│       └── main.go                 # Proxy daemon entrypoint, CLI commands (serve, prune, stats)
├── internal\
│   ├── config\
│   │   └── config.go               # Configuration (cache directory, upstream mirrors, quota limit)
│   ├── oci\
│   │   ├── router.go               # OCI v2 API route definitions (/v2/)
│   │   ├── auth.go                 # Bearer token and basic authentication handler
│   │   ├── manifests.go            # Manifest fetch, caching, and multi-arch digest resolution
│   │   ├── blobs.go                # Blob/Layer streaming with concurrent disk caching
│   │   └── tags.go                 # Tag listing and version enumeration
│   ├── upstream\
│   │   ├── client.go               # Upstream registry client with token refresh
│   │   ├── hub.go                  # Docker Hub auth token broker
│   │   └── token.go                # Token cache and challenge parser
│   ├── storage\
│   │   ├── cas.go                  # Content-addressed filesystem store (blobs/sha256/...)
│   │   ├── deduplication.go        # Shared layer hardlinks across repositories
│   │   ├── gc.go                   # Automated background LRU quota sweeper
│   │   └── index.go                # In-memory and on-disk manifest and blob catalog index
│   ├── api\
│   │   └── handler.go              # Management REST API (/api/v1/catalog, /api/v1/metrics, /api/v1/gc)
│   ├── telemetry\
│   │   └── metrics.go              # Bandwidth saved, cache hit ratios, active pulls
│   └── ui\
│       └── embed.go                # Embedded web assets (embed.FS)
├── web\
│   ├── src\
│   │   ├── components\
│   │   │   ├── Header.tsx          # System status, memory, uptime
│   │   │   ├── ImageCatalog.tsx    # Cached image repositories with tags and sizes
│   │   │   ├── LayerVisualizer.tsx # Step-by-step Dockerfile layer breakdown with size bars
│   │   │   ├── BandwidthGauges.tsx # Visual metrics of local vs upstream bandwidth saved
│   │   │   ├── PruneConsole.tsx    # 1-Click garbage collection and quota adjuster
│   │   │   └── UpstreamStatus.tsx  # Upstream connectivity and latency monitor
│   │   ├── types.ts                # TypeScript interfaces for API contracts
│   │   ├── App.tsx                 # Root DevOps dashboard
│   │   ├── main.tsx                # React 19 entrypoint
│   │   └── index.css               # Tailwind CSS declarations
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── docs\
│   ├── ARCHITECTURE.md             # Detailed engineering specifications
│   ├── DOCKER_SETUP.md             # Docker daemon configuration guide
│   ├── KUBERNETES_CONTAINERD.md    # Kubernetes and containerd configuration
│   └── API.md                      # REST API and OCI endpoints documentation
├── Makefile                        # Multi-platform build and release targets
├── Dockerfile                      # Scratch container image (< 20MB)
├── LICENSE                         # MIT License
├── README.md                       # Humanized project guide
└── MASTER_PLAN.md                  # Master specification
```

---

## 5. Configuration and Docker Daemon Setup

### Quick Daemon Configuration (`/etc/docker/daemon.json` or Docker Desktop)

```json
{
  "registry-mirrors": ["http://localhost:5000"]
}
```

Once configured, any standard docker command automatically routes through regcraft:
```bash
docker pull postgres:16
# First pull: streams from upstream and caches locally
# Second pull: instant delivery from local NVMe/SSD cache
```

---

## 6. Implementation Roadmap

### Phase 1: OCI Distribution Spec v2 Core
- [x] Initialize Go module `github.com/alexandrmotologa/regcraft`.
- [ ] Implement `GET /v2/` ping endpoint with `Docker-Distribution-API-Version` header.
- [ ] Implement manifest fetching, caching, and multi-arch fat manifest index handling.
- [ ] Implement streaming blob proxy using `io.TeeReader` to pipe data to client while writing to disk.

### Phase 2: Content-Addressed Storage & Layer Deduplication
- [ ] Implement content-addressed storage engine (`internal/storage/cas.go`).
- [ ] Verify SHA-256 layer checksums during write to prevent corrupted cache blocks.
- [ ] Implement shared layer deduplication across different image repositories.
- [ ] Implement background LRU garbage collector honoring storage quotas (`internal/storage/gc.go`).

### Phase 3: Upstream Multi-Registry Hub
- [ ] Support Docker Hub anonymous token negotiation.
- [ ] Support GitHub Container Registry (`ghcr.io`) and Quay.io.
- [ ] Handle offline mode: if upstream is unreachable, serve cached manifests directly.

### Phase 4: Embedded React 19 Layer Inspector
- [ ] Scaffold Vite + React 19 frontend in `web/`.
- [ ] Build visual image catalog with tag search and pull commands.
- [ ] Build layer visualizer showing compressed and uncompressed sizes.
- [ ] Embed static assets directly into the Go executable via `//go:embed`.

### Phase 5: Hardening, Benchmarking & Documentation
- [ ] Write unit tests and integration tests for CAS, Manifests, OCI router, Deduplication, and GC.
- [ ] Provide Docker Desktop, Linux systemd, and Kubernetes containerd setup guides.
- [ ] Verify single binary compilation (< 30MB RAM footprint).
