package integrationtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIHelpWorksWithoutEnv(t *testing.T) {
	workingDirectory, workingDirectoryErr := os.Getwd()
	if workingDirectoryErr != nil {
		t.Fatalf("failed to get working directory: %v", workingDirectoryErr)
	}

	repositoryRoot := filepath.Dir(filepath.Dir(workingDirectory))
	temporaryBinaryDirectory := t.TempDir()
	temporaryBinaryPath := filepath.Join(temporaryBinaryDirectory, "pinguin-cli")

	buildCommand := exec.Command("go", "build", "-o", temporaryBinaryPath, "./cmd/client")
	buildCommand.Dir = repositoryRoot
	commandOutput, buildErr := buildCommand.CombinedOutput()
	if buildErr != nil {
		t.Fatalf("go build failed: %v\n%s", buildErr, string(commandOutput))
	}

	helpCommand := exec.Command(temporaryBinaryPath, "--help")
	helpCommand.Dir = repositoryRoot
	helpOutput, helpErr := helpCommand.CombinedOutput()
	if helpErr != nil {
		t.Fatalf("expected --help to succeed: %v\n%s", helpErr, string(helpOutput))
	}
	if !strings.Contains(string(helpOutput), "send") {
		t.Fatalf("expected help output to mention send subcommand, got:\n%s", string(helpOutput))
	}
}
