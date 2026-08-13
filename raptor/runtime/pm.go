package raptor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PackageManifest represents a raptor.json package descriptor.
type PackageManifest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Dependencies map[string]string `json:"dependencies"`
}

// InitPackage creates a new raptor.json manifest in the current directory.
func InitPackage(name string) error {
	if name == "" {
		wd, err := os.Getwd()
		if err == nil {
			name = filepath.Base(wd)
		} else {
			name = "raptor-app"
		}
	}

	manifestFile := "raptor.json"
	if _, err := os.Stat(manifestFile); err == nil {
		return fmt.Errorf("raptor.json already exists in current directory")
	}

	manifest := PackageManifest{
		Name:         name,
		Version:      "0.1.0",
		Description:  "Raptor application",
		Dependencies: make(map[string]string),
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(manifestFile, data, 0644); err != nil {
		return err
	}

	_ = os.MkdirAll("raptor_modules", 0755)
	_ = os.MkdirAll("lib", 0755)

	fmt.Printf("Initialized new Raptor package %q (created raptor.json, lib/, raptor_modules/)\n", name)
	return nil
}

// GetPackage clones a Git repository into ./raptor_modules/ and updates raptor.json.
func GetPackage(pkgSpec string) error {
	if pkgSpec == "" {
		return fmt.Errorf("package repository specification required (e.g. github.com/user/lib or https://github.com/user/lib.git)")
	}

	parts := strings.Split(pkgSpec, "@")
	repoPath := parts[0]
	tag := ""
	if len(parts) > 1 {
		tag = parts[1]
	}

	gitURL := repoPath
	if !strings.HasPrefix(gitURL, "http://") && !strings.HasPrefix(gitURL, "https://") && !strings.HasPrefix(gitURL, "git@") {
		gitURL = "https://" + repoPath
	}
	if !strings.HasSuffix(gitURL, ".git") && !strings.HasPrefix(gitURL, "git@") {
		gitURL = gitURL + ".git"
	}

	// Calculate target directory inside raptor_modules/
	cleanRepo := strings.TrimPrefix(strings.TrimPrefix(repoPath, "https://"), "http://")
	cleanRepo = strings.TrimSuffix(cleanRepo, ".git")
	targetDir := filepath.Join("raptor_modules", filepath.Clean(cleanRepo))

	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Printf("Fetching package %q into %s ...\n", repoPath, targetDir)

	if _, err := os.Stat(targetDir); err == nil {
		// Already cloned, update with git pull
		fmt.Printf("Package already exists at %s, pulling latest changes...\n", targetDir)
		cmd := exec.Command("git", "pull")
		cmd.Dir = targetDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	} else {
		// Clone repository
		cloneArgs := []string{"clone"}
		if tag != "" {
			cloneArgs = append(cloneArgs, "--branch", tag, "--depth", "1")
		} else {
			cloneArgs = append(cloneArgs, "--depth", "1")
		}
		cloneArgs = append(cloneArgs, gitURL, targetDir)

		cmd := exec.Command("git", cloneArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed for %s: %w", gitURL, err)
		}
	}

	// Update raptor.json if present
	manifestFile := "raptor.json"
	var manifest PackageManifest
	if data, err := os.ReadFile(manifestFile); err == nil {
		if err := json.Unmarshal(data, &manifest); err == nil {
			if manifest.Dependencies == nil {
				manifest.Dependencies = make(map[string]string)
			}
			depVal := tag
			if depVal == "" {
				depVal = "latest"
			}
			manifest.Dependencies[repoPath] = depVal

			updatedData, _ := json.MarshalIndent(manifest, "", "  ")
			_ = os.WriteFile(manifestFile, updatedData, 0644)
			fmt.Printf("Updated %s with dependency %s@%s\n", manifestFile, repoPath, depVal)
		}
	}

	fmt.Printf("Successfully installed %s into %s\n", repoPath, targetDir)
	return nil
}

// InstallPackages reads raptor.json and clones all missing dependencies.
func InstallPackages() error {
	manifestFile := "raptor.json"
	data, err := os.ReadFile(manifestFile)
	if err != nil {
		return fmt.Errorf("raptor.json not found in current directory: run 'raptor init' first")
	}

	var manifest PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("error parsing raptor.json: %w", err)
	}

	if len(manifest.Dependencies) == 0 {
		fmt.Println("No dependencies found in raptor.json")
		return nil
	}

	fmt.Printf("Installing %d dependencies from raptor.json into raptor_modules/ ...\n", len(manifest.Dependencies))
	for repo, tag := range manifest.Dependencies {
		spec := repo
		if tag != "" && tag != "latest" {
			spec = fmt.Sprintf("%s@%s", repo, tag)
		}
		if err := GetPackage(spec); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get %s: %v\n", spec, err)
		}
	}

	fmt.Println("All dependencies processed.")
	return nil
}

// FindModuleInRaptorModules searches for a module name in raptor_modules/
func FindModuleInRaptorModules(moduleName string) string {
	modulesDir := "raptor_modules"
	if _, err := os.Stat(modulesDir); err != nil {
		return ""
	}

	// Try standard candidates:
	// 1. raptor_modules/<module>/lib/<module>.rp
	// 2. raptor_modules/<module>/<module>.rp
	// 3. raptor_modules/**/<module>.rp
	var foundPath string
	_ = filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			if base == moduleName && (strings.HasSuffix(path, ".rp") || strings.HasSuffix(path, ".raptor")) {
				foundPath = path
				return filepath.SkipAll
			}
		}
		return nil
	})

	return foundPath
}
