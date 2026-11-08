package oci

import (
	"encoding/json"
	"net/http"
)

// ErrorCode represents an OCI Distribution error code.
type ErrorCode string

const (
	ErrCodeBlobUnknown     ErrorCode = "BLOB_UNKNOWN"
	ErrCodeManifestUnknown ErrorCode = "MANIFEST_UNKNOWN"
	ErrCodeNameUnknown     ErrorCode = "NAME_UNKNOWN"
	ErrCodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrCodeDenied          ErrorCode = "DENIED"
	ErrCodeUnsupported     ErrorCode = "UNSUPPORTED"
)

// ErrorDescriptor describes an individual error in the response.
type ErrorDescriptor struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Detail  interface{} `json:"detail,omitempty"`
}

// ErrorResponse represents the top-level OCI Distribution error schema.
type ErrorResponse struct {
	Errors []ErrorDescriptor `json:"errors"`
}

// WriteError writes a compliant OCI error payload.
func WriteError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(status)

	resp := ErrorResponse{
		Errors: []ErrorDescriptor{
			{
				Code:    code,
				Message: message,
			},
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}
