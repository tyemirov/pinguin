package tests

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerRuntimeEnvironmentAccessIsCentralized(t *testing.T) {
	t.Helper()

	allowedOccurrences := map[string][]string{
		"internal/config/config.go": {
			"os.Expand(",
			"os.LookupEnv(",
		},
		"internal/doctor/doctor.go": {
			"os.ExpandEnv(",
		},
	}

	patterns := []string{
		"os.Getenv(",
		"os.LookupEnv(",
		"os.Expand(",
		"os.ExpandEnv(",
		"os.Environ(",
		"syscall.Getenv(",
		"godotenv.",
		".BindEnv(",
		".AutomaticEnv(",
		"process.env",
		"import.meta.env",
	}

	var violations []string
	for _, root := range []string{"cmd", "internal", "pkg", "web/js"} {
		walkErr := filepath.WalkDir(repoPath(root), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if shouldSkipEnvironmentBoundaryFile(path) {
				return nil
			}

			contents, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			relativePath := filepath.ToSlash(strings.TrimPrefix(path, repoPath()+string(filepath.Separator)))
			for _, line := range strings.Split(string(contents), "\n") {
				for _, pattern := range patterns {
					if !strings.Contains(line, pattern) {
						continue
					}
					if isAllowedEnvironmentBoundaryOccurrence(allowedOccurrences, relativePath, line) {
						continue
					}
					violations = append(violations, relativePath+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s: %v", root, walkErr)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("configuration environment access must stay centralized in config.yml parsing or explicit doctor validation:\n%s", strings.Join(violations, "\n"))
	}
}

func isAllowedEnvironmentBoundaryOccurrence(allowedOccurrences map[string][]string, relativePath, line string) bool {
	for _, allowedSubstring := range allowedOccurrences[relativePath] {
		if strings.Contains(line, allowedSubstring) {
			return true
		}
	}
	return false
}

func shouldSkipEnvironmentBoundaryFile(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	switch filepath.Ext(path) {
	case ".go", ".js", ".ts":
		return false
	default:
		return true
	}
}
