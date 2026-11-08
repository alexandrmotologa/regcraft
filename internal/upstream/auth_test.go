package upstream

import (
	"testing"
)

func TestCredentialStore(t *testing.T) {
	cs := NewCredentialStore()

	cs.SetCredentials("ghcr.io", "myuser", "ghp_secrettoken123")
	u, p, found := cs.GetCredentials("ghcr.io")
	if !found {
		t.Fatalf("expected credentials found for ghcr.io")
	}
	if u != "myuser" || p != "ghp_secrettoken123" {
		t.Errorf("unexpected creds: user=%s, pass=%s", u, p)
	}

	cs.SetCredentials("https://registry-1.docker.io/v2/", "dockeruser", "dockerpass")
	u2, p2, found2 := cs.GetCredentials("docker.io")
	if !found2 {
		t.Fatalf("expected credentials normalized for docker.io")
	}
	if u2 != "dockeruser" || p2 != "dockerpass" {
		t.Errorf("unexpected creds: user=%s, pass=%s", u2, p2)
	}
}
