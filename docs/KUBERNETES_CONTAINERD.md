# Kubernetes and containerd Configuration Guide

containerd v1.5+ and v2.0+ use registry host configuration directories in `/etc/containerd/certs.d/` or `/etc/containerd/config.toml` to configure registry mirrors.

## 1. containerd hosts.toml Mirror Setup

For each upstream registry you want to mirror, create a directory under `/etc/containerd/certs.d/`:

### Docker Hub Mirror
Create `/etc/containerd/certs.d/docker.io/hosts.toml`:

```toml
server = "https://registry-1.docker.io"

[host."http://<regcraft-ip>:5000"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
```

### GitHub Container Registry (ghcr.io) Mirror
Create `/etc/containerd/certs.d/ghcr.io/hosts.toml`:

```toml
server = "https://ghcr.io"

[host."http://<regcraft-ip>:5000/ghcr.io"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
```

Ensure containerd config (`/etc/containerd/config.toml`) enables the config path:

```toml
[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/etc/containerd/certs.d"
```

Restart containerd:
```bash
sudo systemctl restart containerd
```

## 2. K3s Lightweight Kubernetes Setup

K3s allows defining registry mirrors in `/etc/rancher/k3s/registries.yaml`:

```yaml
mirrors:
  docker.io:
    endpoint:
      - "http://127.0.0.1:5000"
  "*.docker.io":
    endpoint:
      - "http://127.0.0.1:5000"
```

Restart K3s:
```bash
sudo systemctl restart k3s
```

All pod image pulls in your Kubernetes cluster will now pull through regcraft. When multiple worker nodes pull the same image, layers are delivered locally with zero external network traffic.
