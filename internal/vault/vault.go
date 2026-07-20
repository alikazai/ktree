package vault

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// VaultDir returns the path to the vault directory for the given repo.
// It lives alongside the repo: <parent>/.vault-<repo>.
func VaultDir(repoDir string) string {
	return filepath.Join(filepath.Dir(repoDir), ".vault-"+filepath.Base(repoDir))
}

// VaultExists reports whether the vault directory has been initialized.
func VaultExists(repoDir string) bool {
	_, err := os.Stat(VaultDir(repoDir))
	return err == nil
}

// DiscoverEnvFiles walks repoDir and returns relative paths of all .env* files,
// skipping the .git directory.
func DiscoverEnvFiles(repoDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".env") {
			rel, err := filepath.Rel(repoDir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

// InitVault copies the given relative file paths from repoDir into the vault,
// preserving directory structure.
func InitVault(repoDir string, files []string) error {
	vaultDir := VaultDir(repoDir)
	for _, rel := range files {
		src := filepath.Join(repoDir, rel)
		dst := filepath.Join(vaultDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}

// CopyVaultToWorktree copies all files from the vault into worktreePath,
// preserving directory structure and overwriting existing files.
func CopyVaultToWorktree(repoDir, worktreePath string) error {
	vaultDir := VaultDir(repoDir)
	return filepath.Walk(vaultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(vaultDir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(worktreePath, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		return copyFile(path, dst)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
