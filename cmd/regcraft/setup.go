package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Automate container runtime mirror configuration",
	}

	setupCmd.AddCommand(newSetupDockerCmd())
	setupCmd.AddCommand(newSetupContainerdCmd())

	return setupCmd
}

func newSetupDockerCmd() *cobra.Command {
	var mirrorURL string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Configure Docker daemon to use regcraft as a pull-through mirror",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := detectDockerDaemonPath()

			fmt.Printf("Target Docker daemon configuration: %s\n", path)

			var daemonConfig map[string]interface{}

			data, err := os.ReadFile(path)
			if err == nil {
				if err := json.Unmarshal(data, &daemonConfig); err != nil {
					return fmt.Errorf("failed to parse existing daemon.json: %w", err)
				}
			} else {
				daemonConfig = make(map[string]interface{})
			}

			// Retrieve or create registry-mirrors array
			var mirrors []string
			if rawMirrors, ok := daemonConfig["registry-mirrors"]; ok {
				if list, ok := rawMirrors.([]interface{}); ok {
					for _, item := range list {
						if s, ok := item.(string); ok {
							mirrors = append(mirrors, s)
						}
					}
				}
			}

			// Check if already present
			alreadyConfigured := false
			for _, m := range mirrors {
				if m == mirrorURL {
					alreadyConfigured = true
					break
				}
			}

			if alreadyConfigured {
				fmt.Printf("✓ Mirror %s is already configured in %s\n", mirrorURL, path)
				return nil
			}

			mirrors = append(mirrors, mirrorURL)
			daemonConfig["registry-mirrors"] = mirrors

			outputJSON, err := json.MarshalIndent(daemonConfig, "", "  ")
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Println("\n[Dry-Run Preview of daemon.json]")
				fmt.Println(string(outputJSON))
				return nil
			}

			// Ensure parent directory exists
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory %s: %w", dir, err)
			}

			// Backup if existing
			if len(data) > 0 {
				backupPath := fmt.Sprintf("%s.bak.%d", path, time.Now().Unix())
				if err := os.WriteFile(backupPath, data, 0644); err == nil {
					fmt.Printf("Created backup at %s\n", backupPath)
				}
			}

			if err := os.WriteFile(path, outputJSON, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w (ensure you have write permissions)", path, err)
			}

			fmt.Printf("✓ Successfully added %s to %s\n", mirrorURL, path)
			fmt.Println("\nRestart Docker to apply changes:")
			if runtime.GOOS == "linux" {
				fmt.Println("  sudo systemctl restart docker")
			} else {
				fmt.Println("  Restart Docker Desktop via system tray icon")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&mirrorURL, "mirror-url", "http://localhost:5000", "regcraft mirror URL to register")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without modifying daemon.json")

	return cmd
}

func newSetupContainerdCmd() *cobra.Command {
	var mirrorURL string

	cmd := &cobra.Command{
		Use:   "containerd",
		Short: "Display containerd /etc/containerd/certs.d mirror configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("# /etc/containerd/certs.d/docker.io/hosts.toml\n")
			fmt.Printf("server = \"https://registry-1.docker.io\"\n\n")
			fmt.Printf("[host.%q]\n", mirrorURL)
			fmt.Printf("  capabilities = [\"pull\", \"resolve\"]\n")
			fmt.Printf("  skip_verify = true\n")
		},
	}

	cmd.Flags().StringVar(&mirrorURL, "mirror-url", "http://localhost:5000", "regcraft mirror URL")

	return cmd
}

func detectDockerDaemonPath() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		userPath := filepath.Join(home, ".docker", "daemon.json")
		return userPath
	case "darwin":
		return filepath.Join(home, ".docker", "daemon.json")
	default:
		// Linux
		return "/etc/docker/daemon.json"
	}
}
