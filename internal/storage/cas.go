package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LayerRef represents a layer or config descriptor referenced by a manifest.
type LayerRef struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType"`
}

// CachedManifest encapsulates an image manifest and its metadata.
type CachedManifest struct {
	Repository   string     `json:"repository"`
	Reference    string     `json:"reference"`
	Digest       string     `json:"digest"`
	MediaType    string     `json:"mediaType"`
	Size         int64      `json:"size"`
	Payload      []byte     `json:"payload"`
	Layers       []LayerRef `json:"layers"`
	ConfigDigest string           `json:"configDigest,omitempty"`
	Pinned       bool             `json:"pinned"`
	ImageConfig  *ImageConfigInfo `json:"imageConfig,omitempty"`
	CreatedAt    time.Time        `json:"createdAt"`
	LastAccessed time.Time  `json:"lastAccessed"`
}

// Store manages content-addressed blobs and manifests on local disk.
type Store struct {
	root         string
	blobsDir     string
	manifestsDir string
	tmpDir       string
	mu           sync.RWMutex
}

// NewStore initializes the storage directory layout.
func NewStore(root string) (*Store, error) {
	blobsDir := filepath.Join(root, "blobs", "sha256")
	manifestsDir := filepath.Join(root, "manifests")
	tmpDir := filepath.Join(root, "tmp")

	dirs := []string{blobsDir, manifestsDir, tmpDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("failed to create storage dir %q: %w", d, err)
		}
	}

	return &Store{
		root:         root,
		blobsDir:     blobsDir,
		manifestsDir: manifestsDir,
		tmpDir:       tmpDir,
	}, nil
}

// Root returns the root storage directory.
func (s *Store) Root() string {
	return s.root
}

// CanonicalDigest strips any algorithm prefix and normalizes to lowercase sha256 hex.
func CanonicalDigest(digest string) (string, error) {
	d := strings.TrimSpace(digest)
	if strings.HasPrefix(d, "sha256:") {
		d = strings.TrimPrefix(d, "sha256:")
	}
	if len(d) != 64 {
		return "", fmt.Errorf("invalid sha256 digest length %d: %q", len(d), digest)
	}
	return strings.ToLower(d), nil
}

// blobPath computes the partitioned file path for a digest.
func (s *Store) blobPath(digestHex string) string {
	prefix := digestHex[:2]
	return filepath.Join(s.blobsDir, prefix, digestHex)
}

// HasBlob checks whether a blob exists in CAS.
func (s *Store) HasBlob(digest string) bool {
	digestHex, err := CanonicalDigest(digest)
	if err != nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.blobPath(digestHex)
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

// GetBlobPath returns the filesystem path of a cached blob if it exists.
func (s *Store) GetBlobPath(digest string) (string, error) {
	digestHex, err := CanonicalDigest(digest)
	if err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.blobPath(digestHex)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", os.ErrNotExist
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("expected file but found directory at %q", path)
	}
	return path, nil
}

// OpenBlob opens a cached blob file for reading and returns its size.
func (s *Store) OpenBlob(digest string) (*os.File, int64, error) {
	path, err := s.GetBlobPath(digest)
	if err != nil {
		return nil, 0, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}

	return f, info.Size(), nil
}

// BlobWriter handles streaming writes into CAS with SHA-256 verification.
type BlobWriter struct {
	store          *Store
	expectedDigest string
	tempFile       *os.File
	tempPath       string
	hasher         io.Writer
	hash           [32]byte
	bytesWritten   int64
	hasherImpl     interface{ Sum([]byte) []byte }
	closed         bool
}

// NewBlobWriter creates an atomic streaming writer for a blob.
func (s *Store) NewBlobWriter(expectedDigest string) (*BlobWriter, error) {
	digestHex, err := CanonicalDigest(expectedDigest)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	h := sha256.New()
	tempFile, err := os.CreateTemp(s.tmpDir, fmt.Sprintf("blob-%s-*.tmp", digestHex[:8]))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp blob file: %w", err)
	}

	return &BlobWriter{
		store:          s,
		expectedDigest: digestHex,
		tempFile:       tempFile,
		tempPath:       tempFile.Name(),
		hasher:         io.MultiWriter(tempFile, h),
		hasherImpl:     h,
	}, nil
}

// Write writes data to the temp file and computes the SHA-256 hash.
func (w *BlobWriter) Write(p []byte) (n int, err error) {
	n, err = w.hasher.Write(p)
	w.bytesWritten += int64(n)
	return n, err
}

// Commit verifies the SHA-256 checksum, flushes to disk, and moves to CAS.
func (w *BlobWriter) Commit() error {
	if w.closed {
		return errors.New("blob writer already closed")
	}
	w.closed = true

	if err := w.tempFile.Sync(); err != nil {
		_ = w.tempFile.Close()
		_ = os.Remove(w.tempPath)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := w.tempFile.Close(); err != nil {
		_ = os.Remove(w.tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	actualDigest := hex.EncodeToString(w.hasherImpl.Sum(nil))
	if actualDigest != w.expectedDigest {
		_ = os.Remove(w.tempPath)
		return fmt.Errorf("digest mismatch: expected sha256:%s, got sha256:%s", w.expectedDigest, actualDigest)
	}

	targetPath := w.store.blobPath(w.expectedDigest)
	targetDir := filepath.Dir(targetPath)

	w.store.mu.Lock()
	defer w.store.mu.Unlock()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		_ = os.Remove(w.tempPath)
		return fmt.Errorf("failed to create blob target directory: %w", err)
	}

	// Rename temp file to destination. If target exists, overwrite atomically.
	if err := os.Rename(w.tempPath, targetPath); err != nil {
		_ = os.Remove(w.tempPath)
		return fmt.Errorf("failed to commit blob to CAS: %w", err)
	}

	return nil
}

// Abort closes and deletes the temporary file if streaming was interrupted.
func (w *BlobWriter) Abort() {
	if !w.closed {
		w.closed = true
		_ = w.tempFile.Close()
		_ = os.Remove(w.tempPath)
	}
}

// BytesWritten returns total bytes streamed so far.
func (w *BlobWriter) BytesWritten() int64 {
	return w.bytesWritten
}

// WriteBlob stores a complete blob from an io.Reader.
func (s *Store) WriteBlob(digest string, r io.Reader) (int64, error) {
	writer, err := s.NewBlobWriter(digest)
	if err != nil {
		return 0, err
	}
	defer writer.Abort()

	n, err := io.Copy(writer, r)
	if err != nil {
		return n, fmt.Errorf("error writing blob data: %w", err)
	}

	if err := writer.Commit(); err != nil {
		return n, err
	}

	return n, nil
}
