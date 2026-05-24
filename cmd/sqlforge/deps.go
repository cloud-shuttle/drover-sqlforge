package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/config"
	"github.com/spf13/cobra"
)

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Pull external dependencies defined in packages.yml",
	Run:   runDeps,
}

func init() {
	rootCmd.AddCommand(depsCmd)
}

func runDeps(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadPackagesConfig(".")
	if err != nil {
		fmt.Printf("Error loading packages.yml: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Packages) == 0 {
		fmt.Println("No packages defined in packages.yml")
		return
	}

	packagesDir := "sqlforge_packages"
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		fmt.Printf("Error creating %s directory: %v\n", packagesDir, err)
		os.Exit(1)
	}

	for _, pkg := range cfg.Packages {
		if pkg.Git == "" {
			fmt.Printf("Warning: Skipping package with empty git URL\n")
			continue
		}

		// Extract repo name from URL to use as directory name
		parts := strings.Split(pkg.Git, "/")
		repoName := parts[len(parts)-1]
		repoName = strings.TrimSuffix(repoName, ".git")

		targetDir := filepath.Join(packagesDir, repoName)
		fmt.Printf("Updating %s...\n", repoName)

		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			// Clone
			fmt.Printf("  Cloning %s\n", pkg.Git)
			cloneArgs := []string{"clone", pkg.Git, targetDir}
			out, err := exec.Command("git", cloneArgs...).CombinedOutput()
			if err != nil {
				fmt.Printf("  Error cloning: %v\n%s\n", err, string(out))
				continue
			}
		} else {
			// Fetch
			fmt.Printf("  Fetching latest from origin\n")
			out, err := exec.Command("git", "-C", targetDir, "fetch", "--all").CombinedOutput()
			if err != nil {
				fmt.Printf("  Error fetching: %v\n%s\n", err, string(out))
				continue
			}
		}

		// Checkout revision
		rev := pkg.Revision
		if rev == "" {
			rev = "main" // fallback to main if no revision specified
		}
		
		fmt.Printf("  Checking out %s\n", rev)
		out, err := exec.Command("git", "-C", targetDir, "checkout", rev).CombinedOutput()
		if err != nil {
			fmt.Printf("  Error checking out revision %s: %v\n%s\n", rev, err, string(out))
			continue
		}
		
		// If it's a branch, we should try pulling the latest just in case.
		// If it fails (e.g. detached HEAD), we ignore the error.
		_ = exec.Command("git", "-C", targetDir, "pull").Run()

		fmt.Printf("  Installed %s@%s successfully.\n", repoName, rev)
	}
	
	fmt.Println("Dependencies updated.")
}
