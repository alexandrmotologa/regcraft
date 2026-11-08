# API Specification

regcraft serves two distinct HTTP API suites on a single port (default `5000`):
1. **OCI Distribution Specification v1.1 & Docker Registry HTTP API v2** under `/v2/`
2. **Management & Monitoring REST API** under `/api/v1/`

---

## 1. OCI Distribution Endpoints (`/v2/`)

### Version Check
```http
GET /v2/
```
- **Responses:**
  - `200 OK`: When unauthenticated or valid credentials supplied.
  - Header: `Docker-Distribution-API-Version: registry/2.0`

### Manifest Operations
```http
GET /v2/<name>/manifests/<reference>
HEAD /v2/<name>/manifests/<reference>
```
- `<reference>` can be a mutable tag (e.g., `latest`, `16`) or an immutable digest (`sha256:717b09...`).
- Supported Media Types:
  - `application/vnd.docker.distribution.manifest.v2+json`
  - `application/vnd.docker.distribution.manifest.list.v2+json`
  - `application/vnd.oci.image.manifest.v1+json`
  - `application/vnd.oci.image.index.v1+json`
- Headers returned:
  - `Content-Type`: Matching manifest media type
  - `Docker-Content-Digest`: Manifest SHA-256 digest
  - `Etag`: Digest string

### Blob Operations
```http
GET /v2/<name>/blobs/<digest>
HEAD /v2/<name>/blobs/<digest>
```
- `<digest>`: Blob SHA-256 string (`sha256:...`).
- Supports byte range requests (`Range: bytes=0-1048575`) with `206 Partial Content`.
- Streams layer at line rate from SSD if cached, or from upstream while concurrently writing to disk if uncached.

### Tag Listing
```http
GET /v2/<name>/tags/list?n=<count>&last=<tag>
```
- Returns JSON object with repository name and tags array.

---

## 2. Management REST API (`/api/v1/`)

### Daemon Status
```http
GET /api/v1/status
```
Returns system uptime, memory usage, version, and cache location.

### Cached Image Catalog
```http
GET /api/v1/catalog
```
Returns list of all cached repositories, known tags, total sizes, and last pulled timestamps.

### Image Tag Inspector
```http
GET /api/v1/catalog/<repository>/tags/<tag>
```
Returns detailed layer breakdown for a given tag:
- Architecture, OS, Docker version
- Individual layer digests, compressed sizes, and uncompressed sizes
- Command history (Dockerfile instructions)

### Real-time Telemetry & Metrics
```http
GET /api/v1/metrics
```
Returns hit counts, miss counts, bytes served from cache, bytes fetched from upstream, hit ratio percentage, and active download streams.

### Prometheus Metrics
```http
GET /metrics
```
Standard Prometheus exposition format for Grafana dashboards.

### Trigger Garbage Collection
```http
POST /api/v1/gc
```
Body (optional):
```json
{
  "dryRun": false,
  "quotaBytes": 21474836480
}
```
Executes LRU cache eviction and returns reclaimed storage bytes and deleted blob count.

### Delete Cached Tag
```http
DELETE /api/v1/catalog/<repository>/tags/<tag>
```
Removes a specific tag from local manifest cache and marks unreferenced layers for reclamation.
