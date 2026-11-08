package upstream

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AcceptedManifestTypes lists the media types accepted from upstreams.
const AcceptedManifestTypes = "application/vnd.docker.distribution.manifest.v2+json, " +
	"application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.oci.image.manifest.v1+json, " +
	"application/vnd.oci.image.index.v1+json, " +
	"application/vnd.docker.distribution.manifest.v1+prettyjws"

// ResolvedRepo contains the parsed upstream target URL and canonical repo path.
type ResolvedRepo struct {
	RegistryHost string
	BaseURL      string
	RepoPath     string
}

// ResolveRepository parses a requested image path and determines its upstream endpoint.
func ResolveRepository(raw string, defaultUpstream string, mirrors map[string]string) ResolvedRepo {
	clean := strings.Trim(raw, "/")

	// Check known mirrors by prefix
	for domain, upstreamURL := range mirrors {
		prefix := domain + "/"
		if strings.HasPrefix(clean, prefix) {
			return ResolvedRepo{
				RegistryHost: domain,
				BaseURL:      upstreamURL,
				RepoPath:     strings.TrimPrefix(clean, prefix),
			}
		}
	}

	// Clean up docker.io prefix if present
	if strings.HasPrefix(clean, "docker.io/") {
		clean = strings.TrimPrefix(clean, "docker.io/")
	}

	// Docker Hub special rule: single token repositories (e.g. "ubuntu") live in "library/"
	repoPath := clean
	if !strings.Contains(repoPath, "/") {
		repoPath = "library/" + repoPath
	}

	return ResolvedRepo{
		RegistryHost: "docker.io",
		BaseURL:      defaultUpstream,
		RepoPath:     repoPath,
	}
}

// ManifestResult contains the fetched manifest data and metadata.
type ManifestResult struct {
	MediaType string
	Digest    string
	ETag      string
	Payload   []byte
}

// Client communicates with upstream OCI registries.
type Client struct {
	httpClient *http.Client
	tokens     *TokenManager
	creds      *CredentialStore
}

// NewClient creates an upstream registry client.
func NewClient(timeout time.Duration) *Client {
	hc := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not leak Authorization headers when redirected to third-party CDNs (e.g. AWS S3)
			if len(via) > 0 && req.URL.Host != via[0].URL.Host {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}

	return &Client{
		httpClient: hc,
		tokens:     NewTokenManager(hc),
		creds:      NewCredentialStore(),
	}
}

// Credentials returns the credential store for configuring auth.
func (c *Client) Credentials() *CredentialStore {
	return c.creds
}

// executeWithAuth performs an HTTP request, negotiating bearer auth challenges if required.
func (c *Client) executeWithAuth(ctx context.Context, host, method, targetURL, scope string, configure func(req *http.Request)) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
	if err != nil {
		return nil, err
	}
	if configure != nil {
		configure(req)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// If 401 Unauthorized with Www-Authenticate header, authenticate and retry once
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("Www-Authenticate")
		resp.Body.Close()

		if authHeader == "" {
			return nil, fmt.Errorf("upstream returned 401 without Www-Authenticate header")
		}

		challenge, err := ParseAuthChallenge(authHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to parse auth challenge: %w", err)
		}

		if challenge.Scope == "" && scope != "" {
			challenge.Scope = scope
		}

		username, password, _ := c.creds.GetCredentials(host)
		token, err := c.tokens.GetToken(challenge, username, password)
		if err != nil {
			return nil, fmt.Errorf("token negotiation failed: %w", err)
		}

		retryReq, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
		if err != nil {
			return nil, err
		}
		if configure != nil {
			configure(retryReq)
		}
		retryReq.Header.Set("Authorization", "Bearer "+token)

		return c.httpClient.Do(retryReq)
	}

	return resp, nil
}

// FetchManifest retrieves an image manifest from the upstream registry.
func (c *Client) FetchManifest(ctx context.Context, repo ResolvedRepo, reference string) (*ManifestResult, error) {
	targetURL := fmt.Sprintf("%s/v2/%s/manifests/%s", repo.BaseURL, repo.RepoPath, reference)
	scope := fmt.Sprintf("repository:%s:pull", repo.RepoPath)

	resp, err := c.executeWithAuth(ctx, repo.RegistryHost, http.MethodGet, targetURL, scope, func(req *http.Request) {
		req.Header.Set("Accept", AcceptedManifestTypes)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream manifest request failed with HTTP %d", resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest body: %w", err)
	}

	mediaType := resp.Header.Get("Content-Type")
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		h := sha256.Sum256(payload)
		digest = "sha256:" + hex.EncodeToString(h[:])
	}

	return &ManifestResult{
		MediaType: mediaType,
		Digest:    digest,
		ETag:      resp.Header.Get("ETag"),
		Payload:   payload,
	}, nil
}

// CheckManifest sends a HEAD request to verify if upstream manifest has changed.
func (c *Client) CheckManifest(ctx context.Context, repo ResolvedRepo, reference string, cachedETag string) (bool, string, error) {
	targetURL := fmt.Sprintf("%s/v2/%s/manifests/%s", repo.BaseURL, repo.RepoPath, reference)
	scope := fmt.Sprintf("repository:%s:pull", repo.RepoPath)

	resp, err := c.executeWithAuth(ctx, repo.RegistryHost, http.MethodHead, targetURL, scope, func(req *http.Request) {
		req.Header.Set("Accept", AcceptedManifestTypes)
		if cachedETag != "" {
			req.Header.Set("If-None-Match", cachedETag)
		}
	})
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return false, cachedETag, nil
	}

	if resp.StatusCode == http.StatusOK {
		digest := resp.Header.Get("Docker-Content-Digest")
		etag := resp.Header.Get("ETag")
		if etag != "" {
			return true, etag, nil
		}
		return true, digest, nil
	}

	return false, "", fmt.Errorf("unexpected status %d checking manifest", resp.StatusCode)
}

// FetchBlob streams an image layer or config blob from upstream.
func (c *Client) FetchBlob(ctx context.Context, repo ResolvedRepo, digest string) (io.ReadCloser, int64, string, error) {
	targetURL := fmt.Sprintf("%s/v2/%s/blobs/%s", repo.BaseURL, repo.RepoPath, digest)
	scope := fmt.Sprintf("repository:%s:pull", repo.RepoPath)

	resp, err := c.executeWithAuth(ctx, repo.RegistryHost, http.MethodGet, targetURL, scope, nil)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to fetch blob: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, "", fmt.Errorf("upstream blob request returned HTTP %d", resp.StatusCode)
	}

	return resp.Body, resp.ContentLength, resp.Header.Get("Content-Type"), nil
}
