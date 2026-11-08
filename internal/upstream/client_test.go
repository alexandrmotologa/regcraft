package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolveRepository(t *testing.T) {
	defaultUpstream := "https://registry-1.docker.io"
	mirrors := map[string]string{
		"ghcr.io":   "https://ghcr.io",
		"quay.io":   "https://quay.io",
		"custom.io": "https://custom.internal",
	}

	tests := []struct {
		input        string
		expectedHost string
		expectedURL  string
		expectedRepo string
	}{
		{"alpine", "docker.io", defaultUpstream, "library/alpine"},
		{"library/postgres", "docker.io", defaultUpstream, "library/postgres"},
		{"docker.io/redis", "docker.io", defaultUpstream, "library/redis"},
		{"docker.io/bitnami/nginx", "docker.io", defaultUpstream, "bitnami/nginx"},
		{"ghcr.io/astral-sh/uv", "ghcr.io", "https://ghcr.io", "astral-sh/uv"},
		{"quay.io/prometheus/prometheus", "quay.io", "https://quay.io", "prometheus/prometheus"},
		{"custom.io/org/project", "custom.io", "https://custom.internal", "org/project"},
	}

	for _, tt := range tests {
		res := ResolveRepository(tt.input, defaultUpstream, mirrors)
		if res.RegistryHost != tt.expectedHost {
			t.Errorf("ResolveRepository(%q) host = %s, expected %s", tt.input, res.RegistryHost, tt.expectedHost)
		}
		if res.BaseURL != tt.expectedURL {
			t.Errorf("ResolveRepository(%q) URL = %s, expected %s", tt.input, res.BaseURL, tt.expectedURL)
		}
		if res.RepoPath != tt.expectedRepo {
			t.Errorf("ResolveRepository(%q) repo = %s, expected %s", tt.input, res.RepoPath, tt.expectedRepo)
		}
	}
}

func TestParseAuthChallenge(t *testing.T) {
	header := `Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/postgres:pull"`
	ch, err := ParseAuthChallenge(header)
	if err != nil {
		t.Fatalf("ParseAuthChallenge failed: %v", err)
	}

	if ch.Scheme != "bearer" {
		t.Errorf("expected scheme 'bearer', got %q", ch.Scheme)
	}
	if ch.Realm != "https://auth.docker.io/token" {
		t.Errorf("expected realm 'https://auth.docker.io/token', got %q", ch.Realm)
	}
	if ch.Service != "registry.docker.io" {
		t.Errorf("expected service 'registry.docker.io', got %q", ch.Service)
	}
	if ch.Scope != "repository:library/postgres:pull" {
		t.Errorf("expected scope 'repository:library/postgres:pull', got %q", ch.Scope)
	}
}

func TestUpstreamAuthAndFetch(t *testing.T) {
	// Mock Auth Server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"token":      "mock-bearer-token-12345",
			"expires_in": 3600,
		})
	}))
	defer authServer.Close()

	// Mock Registry Server
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer mock-bearer-token-12345" {
			w.Header().Set("Www-Authenticate", fmt.Sprintf(`Bearer realm="%s",service="mock-registry"`, authServer.URL))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/v2/library/alpine/manifests/latest" {
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Header().Set("Docker-Content-Digest", "sha256:fedcba9876543210")
			w.Header().Set("ETag", `"etag-123"`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json"}`))
			return
		}

		if r.URL.Path == "/v2/library/alpine/blobs/sha256:fedcba9876543210" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("mock-layer-binary-content"))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer registryServer.Close()

	client := NewClient(5 * time.Second)
	repo := ResolvedRepo{
		RegistryHost: "docker.io",
		BaseURL:      registryServer.URL,
		RepoPath:     "library/alpine",
	}

	// 1. Test FetchManifest with challenge handling
	ctx := context.Background()
	manifest, err := client.FetchManifest(ctx, repo, "latest")
	if err != nil {
		t.Fatalf("FetchManifest failed: %v", err)
	}

	if manifest.Digest != "sha256:fedcba9876543210" {
		t.Errorf("expected digest sha256:fedcba9876543210, got %s", manifest.Digest)
	}
	if manifest.MediaType != "application/vnd.docker.distribution.manifest.v2+json" {
		t.Errorf("unexpected media type: %s", manifest.MediaType)
	}

	// 2. Test FetchBlob
	body, length, _, err := client.FetchBlob(ctx, repo, "sha256:fedcba9876543210")
	if err != nil {
		t.Fatalf("FetchBlob failed: %v", err)
	}
	defer body.Close()

	content, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll blob failed: %v", err)
	}
	if string(content) != "mock-layer-binary-content" {
		t.Errorf("unexpected blob content: %q", string(content))
	}
	if length != int64(len("mock-layer-binary-content")) {
		t.Errorf("expected length %d, got %d", len("mock-layer-binary-content"), length)
	}
}
