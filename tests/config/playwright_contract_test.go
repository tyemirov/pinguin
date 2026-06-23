package tests

import (
	"strings"
	"testing"
)

func TestPlaywrightOwnsDevServerByDefault(t *testing.T) {
	t.Helper()

	playwrightConfig := string(readRepoFile(t, "playwright.config.ts"))
	requiredSnippets := []string{
		"const reuseExistingPlaywrightServer = process.env.PLAYWRIGHT_REUSE_EXISTING_SERVER === '1';",
		"reuseExistingServer: reuseExistingPlaywrightServer",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(playwrightConfig, requiredSnippet) {
			t.Fatalf("Playwright config missing dev server ownership snippet %q", requiredSnippet)
		}
	}
	if strings.Contains(playwrightConfig, "reuseExistingServer: true") {
		t.Fatalf("Playwright must not silently reuse an arbitrary server on the test port")
	}
}
