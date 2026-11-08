package storage

import (
	"encoding/json"
	"io"
	"strings"
	"time"
)

// ImageConfigInfo contains container runtime configurations and Dockerfile instruction history.
type ImageConfigInfo struct {
	Architecture string    `json:"architecture,omitempty"`
	OS           string    `json:"os,omitempty"`
	Created      time.Time `json:"created,omitempty"`
	Env          []string  `json:"env,omitempty"`
	Cmd          []string  `json:"cmd,omitempty"`
	Entrypoint   []string  `json:"entrypoint,omitempty"`
	History      []string  `json:"history,omitempty"`
}

type rawImageConfig struct {
	Architecture string    `json:"architecture"`
	OS           string    `json:"os"`
	Created      time.Time `json:"created"`
	Config       struct {
		Env        []string `json:"Env"`
		Cmd        []string `json:"Cmd"`
		Entrypoint []string `json:"Entrypoint"`
	} `json:"config"`
	History []struct {
		CreatedBy  string `json:"created_by"`
		EmptyLayer bool   `json:"empty_layer"`
		Comment    string `json:"comment"`
	} `json:"history"`
}

// CleanDockerfileInstruction strips `/bin/sh -c #(nop)` boilerplate from history instructions.
func CleanDockerfileInstruction(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "/bin/sh -c #(nop) ")
	s = strings.TrimPrefix(s, "/bin/sh -c ")
	s = strings.TrimSpace(s)
	return s
}

// ParseImageConfigBlob reads a cached config JSON blob and extracts its runtime and history information.
func ParseImageConfigBlob(r io.Reader) (*ImageConfigInfo, error) {
	var raw rawImageConfig
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}

	info := &ImageConfigInfo{
		Architecture: raw.Architecture,
		OS:           raw.OS,
		Created:      raw.Created,
		Env:          raw.Config.Env,
		Cmd:          raw.Config.Cmd,
		Entrypoint:   raw.Config.Entrypoint,
	}

	for _, h := range raw.History {
		cleaned := CleanDockerfileInstruction(h.CreatedBy)
		if cleaned != "" {
			info.History = append(info.History, cleaned)
		}
	}

	return info, nil
}
