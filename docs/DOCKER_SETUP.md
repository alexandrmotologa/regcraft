# Docker Daemon Configuration Guide

regcraft runs as an OCI pull-through mirror on port 5000. Configuring your Docker client or daemon redirects image pulls through regcraft automatically.

## 1. Docker Desktop (macOS and Windows)

1. Open Docker Desktop Settings.
2. Select **Docker Engine** in the left sidebar.
3. Add `registry-mirrors` to your JSON configuration:

```json
{
  "registry-mirrors": [
    "http://localhost:5000"
  ]
}
```

4. Click **Apply & restart**.
5. Pull any image:
```bash
docker pull alpine:latest
docker pull postgres:16
```
The first pull streams through regcraft and caches the layers. Subsequent pulls from any terminal on the machine read directly from local storage.

## 2. Linux Docker Daemon (systemd)

Edit `/etc/docker/daemon.json` (create if it does not exist):

```json
{
  "registry-mirrors": [
    "http://127.0.0.1:5000"
  ]
}
```

Reload the systemd daemon and restart Docker:

```bash
sudo systemctl daemon-reload
sudo systemctl restart docker
```

Verify mirror activation:
```bash
docker info | grep -A 3 "Registry Mirrors"
```

Output should show:
```
 Registry Mirrors:
  http://127.0.0.1:5000/
```

## 3. Remote or LAN Server Setup

If regcraft runs on a central server in your office or home lab (for example, at `192.168.1.50:5000`):

1. Because plain HTTP is used, mark the host as an insecure registry in `/etc/docker/daemon.json` on client machines:

```json
{
  "registry-mirrors": [
    "http://192.168.1.50:5000"
  ],
  "insecure-registries": [
    "192.168.1.50:5000"
  ]
}
```

2. Restart Docker on the client. All machines on the network will now share the central layer cache, avoiding duplicate downloads across developer laptops and CI machines.

## 4. Pulling Directly from Non-Hub Registries

regcraft supports pulling from GitHub Container Registry (ghcr.io) and Quay.io using namespace prefixing:

```bash
# Pull through regcraft explicitly:
docker pull localhost:5000/ghcr.io/astral-sh/uv:latest
docker pull localhost:5000/quay.io/prometheus/prometheus:latest
```
regcraft transparently routes the request to the upstream host, acquires necessary tokens, and caches layers locally.
