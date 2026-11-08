package upstream

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DockerConfigFile represents ~/.docker/config.json structure.
type DockerConfigFile struct {
	Auths map[string]DockerAuth `json:"auths"`
}

// DockerAuth holds base64 encoded auth or username/password.
type DockerAuth struct {
	Auth     string `json:"auth,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// CredentialStore discovers and provides credentials for registries.
type CredentialStore struct {
	credentials map[string]DockerAuth
}

// NewCredentialStore loads credentials from ~/.docker/config.json if available.
func NewCredentialStore() *CredentialStore {
	cs := &CredentialStore{
		credentials: make(map[string]DockerAuth),
	}

	configPaths := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		configPaths = append(configPaths, filepath.Join(home, ".docker", "config.json"))
	}
	if dockerConfig := os.Getenv("DOCKER_CONFIG"); dockerConfig != "" {
		configPaths = append([]string{filepath.Join(dockerConfig, "config.json")}, configPaths...)
	}

	for _, path := range configPaths {
		if data, err := os.ReadFile(path); err == nil {
			var cfg DockerConfigFile
			if err := json.Unmarshal(data, &cfg); err == nil && cfg.Auths != nil {
				for host, auth := range cfg.Auths {
					cleanHost := normalizeHost(host)
					cs.credentials[cleanHost] = auth
				}
				break
			}
		}
	}

	return cs
}

// normalizeHost strips schemes, paths, and standardizes registry hostnames.
func normalizeHost(host string) string {
	h := strings.TrimSpace(host)
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	if idx := strings.Index(h, "/"); idx != -1 {
		h = h[:idx]
	}
	if h == "index.docker.io" || h == "registry-1.docker.io" || h == "index.docker.io:443" {
		return "docker.io"
	}
	return h
}

// GetCredentials retrieves username and password for a target registry.
func (cs *CredentialStore) GetCredentials(registryHost string) (username, password string, found bool) {
	cleanHost := normalizeHost(registryHost)
	auth, ok := cs.credentials[cleanHost]
	if !ok {
		// Try matching without port or subdomains if any
		return "", "", false
	}

	if auth.Username != "" && auth.Password != "" {
		return auth.Username, auth.Password, true
	}

	if auth.Auth != "" {
		decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				return parts[0], parts[1], true
			}
		}
	}

	return "", "", false
}

// SetCredentials allows setting credentials manually or programmatically.
func (cs *CredentialStore) SetCredentials(host, username, password string) {
	cleanHost := normalizeHost(host)
	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	cs.credentials[cleanHost] = DockerAuth{
		Auth:     encoded,
		Username: username,
		Password: password,
	}
}
