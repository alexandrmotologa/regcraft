package storage

import (
	"strings"
	"testing"
)

func TestParseImageConfig(t *testing.T) {
	rawJSON := `{
		"architecture": "amd64",
		"os": "linux",
		"config": {
			"Env": ["PATH=/usr/local/bin", "PORT=8080"],
			"Cmd": ["postgres"]
		},
		"history": [
			{"created_by": "/bin/sh -c #(nop) ADD file:abc in /", "empty_layer": false},
			{"created_by": "/bin/sh -c set -eux; apt-get update; apt-get install -y postgresql", "empty_layer": false},
			{"created_by": "/bin/sh -c #(nop) ENV PORT=8080", "empty_layer": true}
		]
	}`

	info, err := ParseImageConfigBlob(strings.NewReader(rawJSON))
	if err != nil {
		t.Fatalf("ParseImageConfigBlob failed: %v", err)
	}

	if info.Architecture != "amd64" || info.OS != "linux" {
		t.Errorf("unexpected arch/os: %s/%s", info.Architecture, info.OS)
	}

	if len(info.History) != 3 {
		t.Fatalf("expected 3 history items, got %d", len(info.History))
	}

	if info.History[0] != "ADD file:abc in /" {
		t.Errorf("history[0] not cleaned: %q", info.History[0])
	}
	if !strings.HasPrefix(info.History[1], "set -eux") {
		t.Errorf("history[1] not cleaned: %q", info.History[1])
	}
}
