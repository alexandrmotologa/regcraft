package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alexandrmotologa/regcraft/internal/api"
	"github.com/alexandrmotologa/regcraft/internal/config"
	"github.com/alexandrmotologa/regcraft/internal/oci"
	"github.com/alexandrmotologa/regcraft/internal/storage"
	"github.com/alexandrmotologa/regcraft/internal/telemetry"
	"github.com/alexandrmotologa/regcraft/internal/ui"
	"github.com/alexandrmotologa/regcraft/internal/upstream"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"
	commit  = "dev"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "regcraft",
		Short: "High-speed pull-through OCI registry and container image caching proxy",
		Long: `regcraft accelerates container pulls for Docker, Podman, and Kubernetes by caching
layers in content-addressed local storage with SHA-256 deduplication and low memory footprint (< 30MB).`,
	}

	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newWarmCmd())
	rootCmd.AddCommand(newPruneCmd())
	rootCmd.AddCommand(newStatsCmd())
	rootCmd.AddCommand(newPinCmd())
	rootCmd.AddCommand(newUnpinCmd())
	rootCmd.AddCommand(newExportCmd())
	rootCmd.AddCommand(newImportCmd())
	rootCmd.AddCommand(newSetupCmd())
	rootCmd.AddCommand(newVersionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newServeCmd() *cobra.Command {
	cfg := config.DefaultConfig()
	var port string
	var quotaStr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the OCI registry caching proxy and web dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !strings.HasPrefix(port, ":") {
				port = ":" + port
			}
			cfg.ListenAddr = port

			if quotaStr != "" {
				q, err := config.ParseByteSize(quotaStr)
				if err != nil {
					return fmt.Errorf("invalid quota format %q: %w", quotaStr, err)
				}
				cfg.QuotaBytes = q
			}

			// Ensure cache directory exists
			absDir, err := filepath.Abs(cfg.CacheDir)
			if err != nil {
				return err
			}
			cfg.CacheDir = absDir

			store, err := storage.NewStore(cfg.CacheDir)
			if err != nil {
				return fmt.Errorf("failed to initialize storage store: %w", err)
			}

			metrics := telemetry.Default()
			upstreamClient := upstream.NewClient(cfg.UpstreamTimeout)
			warmer := upstream.NewWarmer(cfg, store, upstreamClient, metrics, 4)

			ociHandler := oci.NewHandler(cfg, store, upstreamClient, metrics)
			apiHandler := api.NewHandler(cfg, store, metrics, warmer, version)
			uiHandler := ui.Handler()

			// Root mux router
			mux := http.NewServeMux()

			// 1. OCI API under /v2/
			mux.Handle("/v2/", ociHandler)
			mux.Handle("/v2", ociHandler)

			// 2. Management REST API under /api/v1/
			mux.Handle("/api/v1/", apiHandler)

			// 3. Prometheus metrics endpoint
			mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
				metrics.WritePrometheus(w)
			})

			// 4. Embedded React 19 UI
			mux.Handle("/", uiHandler)

			server := &http.Server{
				Addr:         cfg.ListenAddr,
				Handler:      mux,
				ReadTimeout:  30 * time.Minute, // Long timeout for large multi-gigabyte layers
				WriteTimeout: 30 * time.Minute,
				IdleTimeout:  60 * time.Second,
			}

			// Graceful shutdown channel
			stopChan := make(chan os.Signal, 1)
			signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

			go func() {
				fmt.Printf("\n=== regcraft v%s (%s) ===\n", version, commit)
				fmt.Printf("• OCI Registry Mirror: http://localhost%s\n", cfg.ListenAddr)
				fmt.Printf("• Web Dashboard:       http://localhost%s\n", cfg.ListenAddr)
				fmt.Printf("• Cache Directory:     %s\n", cfg.CacheDir)
				fmt.Printf("• Quota Limit:         %s\n", config.FormatByteSize(cfg.QuotaBytes))
				fmt.Printf("• Default Upstream:    %s\n", cfg.DefaultUpstream)
				fmt.Println("\nReady to accept docker pull requests.")

				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
					os.Exit(1)
				}
			}()

			<-stopChan
			fmt.Println("\nShutting down regcraft gracefully...")

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			return server.Shutdown(ctx)
		},
	}

	cmd.Flags().StringVar(&port, "port", "5000", "Port to listen on (e.g. 5000)")
	cmd.Flags().StringVar(&cfg.CacheDir, "dir", cfg.CacheDir, "Root storage directory for cached layers")
	cmd.Flags().StringVar(&quotaStr, "quota", "20GB", "Storage quota before LRU eviction (e.g. 20GB, 50GB)")
	cmd.Flags().StringVar(&cfg.DefaultUpstream, "upstream-hub", cfg.DefaultUpstream, "Default Docker Hub upstream URL")
	cmd.Flags().BoolVar(&cfg.OfflineMode, "offline", true, "Serve cached images if upstream is unreachable")

	return cmd
}

func newPruneCmd() *cobra.Command {
	var dir string
	var quotaStr string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Run garbage collection to reclaim storage space",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, ".regcraft", "cache")
			}

			store, err := storage.NewStore(dir)
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}

			var quota int64 = 20 * 1024 * 1024 * 1024
			if quotaStr != "" {
				q, err := config.ParseByteSize(quotaStr)
				if err != nil {
					return err
				}
				quota = q
			}

			fmt.Printf("Scanning storage in %s...\n", dir)
			report, err := store.RunGC(quota, dryRun)
			if err != nil {
				return err
			}

			mode := "Executed"
			if dryRun {
				mode = "Simulated (Dry-Run)"
			}

			fmt.Printf("\n=== Garbage Collection Report (%s) ===\n", mode)
			fmt.Printf("Initial storage:   %s (%d blobs)\n", config.FormatByteSize(report.InitialBytes), report.InitialBlobs)
			fmt.Printf("Reclaimed space:   %s\n", config.FormatByteSize(report.ReclaimedBytes))
			fmt.Printf("Deleted blobs:     %d\n", report.DeletedBlobs)
			fmt.Printf("Deleted manifests: %d\n", report.DeletedManifests)
			fmt.Printf("Final storage:     %s (%d blobs)\n", config.FormatByteSize(report.FinalBytes), report.FinalBlobs)

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Cache directory")
	cmd.Flags().StringVar(&quotaStr, "quota", "20GB", "Target storage quota (e.g. 10GB)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview space to be reclaimed without deleting files")

	return cmd
}

func newStatsCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Display storage statistics and cached repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, ".regcraft", "cache")
			}

			store, err := storage.NewStore(dir)
			if err != nil {
				return fmt.Errorf("failed to open storage at %s: %w", dir, err)
			}

			bytesUsed, blobCount, err := store.DiskUsage()
			if err != nil {
				return err
			}

			repos := store.ListRepositories()

			fmt.Printf("\n=== regcraft Storage Statistics ===\n")
			fmt.Printf("Location:        %s\n", dir)
			fmt.Printf("Disk usage:      %s\n", config.FormatByteSize(bytesUsed))
			fmt.Printf("Blobs in CAS:    %d\n", blobCount)
			fmt.Printf("Repositories:    %d\n", len(repos))

			if len(repos) > 0 {
				fmt.Println("\nCached Repositories:")
				for _, repo := range repos {
					tags := store.ListTags(repo)
					fmt.Printf("  • %-30s [tags: %s]\n", repo, strings.Join(tags, ", "))
				}
			} else {
				fmt.Println("\nNo images currently cached.")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Cache directory")

	return cmd
}

func newWarmCmd() *cobra.Command {
	var dir string
	var parallel int
	var upstreamHub string

	cmd := &cobra.Command{
		Use:   "warm <image1> [image2...]",
		Short: "Pre-fetch images and all layers into CAS local cache",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, ".regcraft", "cache")
			}
			cfg := config.DefaultConfig()
			cfg.CacheDir = dir
			if upstreamHub != "" {
				cfg.DefaultUpstream = upstreamHub
			}

			store, err := storage.NewStore(dir)
			if err != nil {
				return err
			}

			metrics := telemetry.Default()
			client := upstream.NewClient(cfg.UpstreamTimeout)
			warmer := upstream.NewWarmer(cfg, store, client, metrics, parallel)

			ctx := context.Background()
			for _, img := range args {
				fmt.Printf("Warming image %s...\n", img)
				res, err := warmer.Warm(ctx, img)
				if err != nil {
					fmt.Printf("  x Failed: %v\n", err)
					continue
				}
				fmt.Printf("  ✓ Cached: %d layers downloaded (%s), %d layers skipped in %v\n",
					res.LayersDownloaded, config.FormatByteSize(res.BytesDownloaded), res.LayersSkipped, res.Duration.Round(time.Millisecond))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Cache directory")
	cmd.Flags().IntVar(&parallel, "concurrency", 4, "Concurrent layer download workers")
	cmd.Flags().StringVar(&upstreamHub, "upstream-hub", "", "Override default Docker Hub URL")

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("regcraft v%s (%s)\n", version, commit)
		},
	}
}

func newPinCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "pin <image:tag>",
		Short: "Freeze a tag to prevent upstream updates",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, ".regcraft", "cache")
			}
			parts := strings.Split(args[0], ":")
			if len(parts) != 2 {
				return fmt.Errorf("expected format <repo>:<tag> (e.g. library/postgres:16)")
			}
			store, err := storage.NewStore(dir)
			if err != nil {
				return err
			}
			if err := store.SetTagPin(parts[0], parts[1], true); err != nil {
				return err
			}
			fmt.Printf("Pinned %s:%s. Manifest is now frozen.\n", parts[0], parts[1])
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Cache directory")
	return cmd
}

func newUnpinCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "unpin <image:tag>",
		Short: "Unfreeze a tag to allow upstream updates",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				home, _ := os.UserHomeDir()
				dir = filepath.Join(home, ".regcraft", "cache")
			}
			parts := strings.Split(args[0], ":")
			if len(parts) != 2 {
				return fmt.Errorf("expected format <repo>:<tag> (e.g. library/postgres:16)")
			}
			store, err := storage.NewStore(dir)
			if err != nil {
				return err
			}
			if err := store.SetTagPin(parts[0], parts[1], false); err != nil {
				return err
			}
			fmt.Printf("Unpinned %s:%s. Upstream update checks re-enabled.\n", parts[0], parts[1])
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Cache directory")
	return cmd
}
