package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledGatewayPlansCurrentTrackedManifest(t *testing.T) {
	trapPath := filepath.Join(t.TempDir(), "inherited-transport.sh")
	trapFile, err := os.Create(trapPath)
	if err != nil {
		t.Fatalf("create inherited transport trap: %v", err)
	}
	if _, err := trapFile.WriteString("printf 'inherited Git transport escaped fixture isolation\\n' >&2\nexit 91\n"); err != nil {
		t.Fatalf("write inherited transport trap: %v", err)
	}
	if err := trapFile.Close(); err != nil {
		t.Fatalf("close inherited transport trap: %v", err)
	}
	globalConfigPath := filepath.Join(t.TempDir(), "inherited-gitconfig")
	globalConfig, err := os.Create(globalConfigPath)
	if err != nil {
		t.Fatalf("create inherited Git configuration: %v", err)
	}
	if _, err := globalConfig.WriteString("[url \"ssh://global.fixture.invalid/\"]\n\tinsteadOf = git@github.com:\n"); err != nil {
		t.Fatalf("write inherited Git configuration: %v", err)
	}
	if err := globalConfig.Close(); err != nil {
		t.Fatalf("close inherited Git configuration: %v", err)
	}
	command := exec.Command("bash", repoPath("tests", "config", "installed-gateway-plan.sh"))
	command.Env = append(os.Environ(),
		"CI=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=sh '"+strings.ReplaceAll(trapPath, "'", "'\\''")+"'",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=url.ssh://fixture.invalid/.insteadOf",
		"GIT_CONFIG_VALUE_0=git@github.com:",
		"GIT_CONFIG_GLOBAL="+globalConfigPath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("installed Gateway selected-manifest plan: %v\n%s", err, output)
	}
}
