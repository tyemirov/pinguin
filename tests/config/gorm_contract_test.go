package tests

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type prohibitedGORMPattern struct {
	name    string
	pattern *regexp.Regexp
}

func TestGORMQueriesUseStructuredClauses(t *testing.T) {
	t.Helper()

	patterns := []prohibitedGORMPattern{
		{name: "raw or exec query", pattern: regexp.MustCompile(`\.(Raw|Exec)\s*\(`)},
		{name: "string-fragment query builder", pattern: regexp.MustCompile(`\.(Where|Order|Joins|Select|Table)\s*\(\s*"`)},
		{name: "raw expression clause", pattern: regexp.MustCompile(`(gorm|clause)\.Expr\b`)},
	}

	var violations []string
	walkErr := filepath.WalkDir(repoPath("."), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipGORMContractDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, ".pb.go") {
			return nil
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, pattern := range patterns {
			matches := pattern.pattern.FindAllIndex(source, -1)
			for _, match := range matches {
				violations = append(violations, formatGORMContractViolation(path, source, match[0], pattern.name))
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk source tree: %v", walkErr)
	}
	if len(violations) > 0 {
		t.Fatalf("GORM contract violations:\n%s", strings.Join(violations, "\n"))
	}
}

func shouldSkipGORMContractDirectory(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "bin":
		return true
	default:
		return false
	}
}

func formatGORMContractViolation(path string, source []byte, offset int, name string) string {
	line := 1 + strings.Count(string(source[:offset]), "\n")
	return path + ":" + strconv.Itoa(line) + ": " + name
}
