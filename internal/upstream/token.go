package upstream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AuthChallenge represents the parsed parameters of a Www-Authenticate header.
type AuthChallenge struct {
	Scheme  string
	Realm   string
	Service string
	Scope   string
}

// ParseAuthChallenge extracts realm, service, and scope from a Www-Authenticate header.
func ParseAuthChallenge(header string) (*AuthChallenge, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil, fmt.Errorf("empty auth header")
	}

	parts := strings.SplitN(header, " ", 2)
	challenge := &AuthChallenge{
		Scheme: strings.ToLower(parts[0]),
	}

	if len(parts) < 2 {
		return challenge, nil
	}

	params := parts[1]
	// Parse key="value" pairs
	for _, item := range strings.Split(params, ",") {
		item = strings.TrimSpace(item)
		kv := strings.SplitN(item, "=", 2)
		if len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			v := strings.Trim(strings.TrimSpace(kv[1]), "\"")
			switch strings.ToLower(k) {
			case "realm":
				challenge.Realm = v
			case "service":
				challenge.Service = v
			case "scope":
				challenge.Scope = v
			}
		}
	}

	return challenge, nil
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

// TokenManager negotiates and caches bearer tokens for upstream registries.
type TokenManager struct {
	client *http.Client
	cache  map[string]cachedToken
	mu     sync.RWMutex
}

// NewTokenManager initializes a token manager.
func NewTokenManager(client *http.Client) *TokenManager {
	return &TokenManager{
		client: client,
		cache:  make(map[string]cachedToken),
	}
}

// GetToken acquires a bearer token for the given challenge parameters.
func (tm *TokenManager) GetToken(challenge *AuthChallenge) (string, error) {
	cacheKey := fmt.Sprintf("%s|%s|%s", challenge.Realm, challenge.Service, challenge.Scope)

	tm.mu.RLock()
	if entry, ok := tm.cache[cacheKey]; ok {
		if time.Now().Before(entry.expiresAt) {
			tm.mu.RUnlock()
			return entry.token, nil
		}
	}
	tm.mu.RUnlock()

	reqURL, err := url.Parse(challenge.Realm)
	if err != nil {
		return "", fmt.Errorf("invalid auth realm URL %q: %w", challenge.Realm, err)
	}

	q := reqURL.Query()
	if challenge.Service != "" {
		q.Set("service", challenge.Service)
	}
	if challenge.Scope != "" {
		q.Set("scope", challenge.Scope)
	}
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	resp, err := tm.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch auth token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth endpoint returned HTTP %d", resp.StatusCode)
	}

	var tokenResp struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse auth response: %w", err)
	}

	tok := tokenResp.Token
	if tok == "" {
		tok = tokenResp.AccessToken
	}
	if tok == "" {
		return "", fmt.Errorf("empty token received from %s", challenge.Realm)
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 300 // 5 minutes default
	}

	tm.mu.Lock()
	tm.cache[cacheKey] = cachedToken{
		token:     tok,
		expiresAt: time.Now().Add(time.Duration(expiresIn-30) * time.Second),
	}
	tm.mu.Unlock()

	return tok, nil
}
