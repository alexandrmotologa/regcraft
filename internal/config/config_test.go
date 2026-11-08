package config

import (
	"testing"
)

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasErr   bool
	}{
		{"500B", 500, false},
		{"10KB", 10 * 1024, false},
		{"50MB", 50 * 1024 * 1024, false},
		{"20GB", 20 * 1024 * 1024 * 1024, false},
		{"1TB", 1024 * 1024 * 1024 * 1024, false},
		{"1.5GB", int64(1.5 * 1024 * 1024 * 1024), false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		res, err := ParseByteSize(tt.input)
		if (err != nil) != tt.hasErr {
			t.Errorf("ParseByteSize(%q) unexpected error status: %v", tt.input, err)
		}
		if !tt.hasErr && res != tt.expected {
			t.Errorf("ParseByteSize(%q) = %d, expected %d", tt.input, res, tt.expected)
		}
	}
}

func TestFormatByteSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{50 * 1024 * 1024, "50.0 MB"},
		{20 * 1024 * 1024 * 1024, "20.0 GB"},
	}

	for _, tt := range tests {
		res := FormatByteSize(tt.bytes)
		if res != tt.expected {
			t.Errorf("FormatByteSize(%d) = %q, expected %q", tt.bytes, res, tt.expected)
		}
	}
}
