package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexandrmotologa/regcraft/internal/config"
	"github.com/alexandrmotologa/regcraft/internal/storage"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var dir string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export cached images and layers into a portable tar archive for air-gapped systems",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputFile == "" {
				return fmt.Errorf("output file flag --output is required (e.g. --output regcraft-export.tar)")
			}

			if dir == "" {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, ".regcraft", "cache")
			}

			store, err := storage.NewStore(dir)
			if err != nil {
				return fmt.Errorf("failed to open storage at %s: %w", dir, err)
			}

			// Determine which images to export
			var targets []string
			if len(args) > 0 {
				targets = args
			} else {
				// Export all cached images
				for _, repo := range store.ListRepositories() {
					for _, tag := range store.ListTags(repo) {
						targets = append(targets, repo+":"+tag)
					}
				}
			}

			if len(targets) == 0 {
				fmt.Println("No images found to export.")
				return nil
			}

			outFile, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("failed to create output file %s: %w", outputFile, err)
			}
			defer outFile.Close()

			tw := tar.NewWriter(outFile)
			defer tw.Close()

			fmt.Printf("Exporting %d images into %s...\n", len(targets), outputFile)

			blobSet := make(map[string]bool)
			var totalBytes int64

			for _, target := range targets {
				parts := strings.Split(target, ":")
				repo := parts[0]
				tag := "latest"
				if len(parts) > 1 {
					tag = parts[1]
				}

				cm, err := store.GetManifest(repo, tag)
				if err != nil {
					fmt.Printf("  x Skipping %s:%s (not in cache)\n", repo, tag)
					continue
				}

				// Write manifest JSON into tar
				manifestPath := filepath.ToSlash(filepath.Join("manifests", repo, "tags", tag+".json"))
				data, _ := json.MarshalIndent(cm, "", "  ")
				hdr := &tar.Header{
					Name: manifestPath,
					Mode: 0644,
					Size: int64(len(data)),
				}
				_ = tw.WriteHeader(hdr)
				_, _ = tw.Write(data)

				// Collect blobs
				if cm.ConfigDigest != "" {
					blobSet[cm.ConfigDigest] = true
				}
				for _, l := range cm.Layers {
					blobSet[l.Digest] = true
				}
				fmt.Printf("  ✓ Bundled manifest: %s:%s (%d layers)\n", repo, tag, len(cm.Layers))
			}

			// Write referenced blobs into tar
			var blobCount int
			for digest := range blobSet {
				f, size, err := store.OpenBlob(digest)
				if err != nil {
					continue
				}

				digestHex, _ := storage.CanonicalDigest(digest)
				tarBlobPath := filepath.ToSlash(filepath.Join("blobs", "sha256", digestHex[:2], digestHex))

				hdr := &tar.Header{
					Name: tarBlobPath,
					Mode: 0644,
					Size: size,
				}
				if err := tw.WriteHeader(hdr); err == nil {
					n, _ := io.Copy(tw, f)
					totalBytes += n
					blobCount++
				}
				f.Close()
			}

			fmt.Printf("\nExport complete: %d unique blobs packaged (%s total).\n", blobCount, config.FormatByteSize(totalBytes))
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Cache directory")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Destination tar file (e.g. images.tar)")

	return cmd
}

func newImportCmd() *cobra.Command {
	var dir string
	var inputFile string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a portable tar archive of cached images and layers into CAS",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputFile == "" {
				return fmt.Errorf("input file flag --input is required (e.g. --input regcraft-export.tar)")
			}

			if dir == "" {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, ".regcraft", "cache")
			}

			store, err := storage.NewStore(dir)
			if err != nil {
				return fmt.Errorf("failed to open storage at %s: %w", dir, err)
			}

			inFile, err := os.Open(inputFile)
			if err != nil {
				return fmt.Errorf("failed to open archive %s: %w", inputFile, err)
			}
			defer inFile.Close()

			tr := tar.NewReader(inFile)
			var importedBlobs int
			var importedManifests int
			var importedBytes int64

			fmt.Printf("Importing from %s into %s...\n", inputFile, dir)

			for {
				hdr, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("error reading tar archive: %w", err)
				}

				cleanPath := filepath.Clean(hdr.Name)

				if strings.HasPrefix(cleanPath, "blobs") {
					digestHex := filepath.Base(cleanPath)
					digest := "sha256:" + digestHex

					// Write through blob writer to verify SHA-256
					writer, err := store.NewBlobWriter(digest)
					if err == nil {
						n, errCopy := io.Copy(writer, tr)
						if errCopy == nil {
							if errCommit := writer.Commit(); errCommit == nil {
								importedBlobs++
								importedBytes += n
							}
						}
					}
				} else if strings.HasPrefix(cleanPath, "manifests") {
					payload, err := io.ReadAll(tr)
					if err == nil {
						var cm storage.CachedManifest
						if err := json.Unmarshal(payload, &cm); err == nil {
							_, _ = store.SaveManifest(cm.Repository, cm.Reference, cm.MediaType, cm.Payload)
							importedManifests++
							fmt.Printf("  ✓ Restored manifest: %s:%s\n", cm.Repository, cm.Reference)
						}
					}
				}
			}

			fmt.Printf("\nImport complete: %d manifests restored, %d blobs imported (%s).\n",
				importedManifests, importedBlobs, config.FormatByteSize(importedBytes))
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Cache directory")
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Source tar file to import")

	return cmd
}
