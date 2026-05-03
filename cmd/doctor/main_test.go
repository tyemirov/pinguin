package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandWritesSummaryForValidConfig(t *testing.T) {
	configPath := writeDoctorConfig(t, validDoctorConfig)
	var stdout bytes.Buffer
	command := newRootCommand()
	command.SetOut(&stdout)
	command.SetErr(io.Discard)
	command.SetArgs([]string{configPath})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute doctor: %v", err)
	}
	if !strings.Contains(stdout.String(), "Pinguin Doctor Report") {
		t.Fatalf("expected summary output, got %q", stdout.String())
	}
}

func TestRootCommandWritesJSON(t *testing.T) {
	configPath := writeDoctorConfig(t, validDoctorConfig)
	var stdout bytes.Buffer
	command := newRootCommand()
	command.SetOut(&stdout)
	command.SetErr(io.Discard)
	command.SetArgs([]string{configPath, "--json"})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute doctor json: %v", err)
	}
	if !strings.Contains(stdout.String(), `"schema_version"`) {
		t.Fatalf("expected JSON output, got %q", stdout.String())
	}
}

func TestRunDoctorReturnsValidationFailure(t *testing.T) {
	configPath := writeDoctorConfig(t, "server: {}\n")
	var stdout bytes.Buffer
	command := newRootCommand()
	command.SetOut(&stdout)
	command.SetErr(io.Discard)
	command.SetArgs([]string{configPath})

	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("expected validation failure, got %v", err)
	}
	if !strings.Contains(stdout.String(), "INVALID") {
		t.Fatalf("expected invalid summary, got %q", stdout.String())
	}
}

func TestRunDoctorReturnsNoConfigError(t *testing.T) {
	command := newRootCommand()
	command.SetOut(io.Discard)

	err := runDoctor(command, nil)
	if err == nil || !strings.Contains(err.Error(), "no config paths provided") {
		t.Fatalf("expected no config error, got %v", err)
	}
}

func TestRunDoctorReturnsFlagErrors(t *testing.T) {
	testCases := []struct {
		name      string
		flagName  string
		arguments []string
	}{
		{name: "cross validate", flagName: flagCrossValidate, arguments: []string{"config.yml"}},
		{name: "expand env", flagName: flagExpandEnv, arguments: []string{"config.yml"}},
		{name: "json", flagName: flagOutputJSON, arguments: []string{"config.yml"}},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			command := &cobra.Command{}
			for _, flagName := range []string{flagCrossValidate, flagExpandEnv, flagOutputJSON} {
				if flagName == testCase.flagName {
					continue
				}
				command.Flags().Bool(flagName, false, "bool flag")
			}
			command.Flags().String(testCase.flagName, "not-bool", "bad flag")
			err := runDoctor(command, testCase.arguments)
			if err == nil {
				t.Fatalf("expected flag error")
			}
		})
	}
}

func TestRunDoctorReturnsWriteError(t *testing.T) {
	configPath := writeDoctorConfig(t, validDoctorConfig)
	command := newRootCommand()
	command.SetOut(failingDoctorWriter{})

	err := runDoctor(command, []string{configPath})
	if err == nil || !strings.Contains(err.Error(), "doctor.write_output") {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestRunAndExitUsesExitForFailures(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var exitCode int
	runAndExit([]string{}, &stdout, &stderr, func(code int) {
		exitCode = code
	})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if stderr.Len() == 0 {
		t.Fatalf("expected error output")
	}
}

func TestRunReturnsForHelp(t *testing.T) {
	var stdout bytes.Buffer
	if err := run([]string{"--help"}, &stdout, io.Discard); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Validate one or more Pinguin configuration files") {
		t.Fatalf("unexpected help output %q", stdout.String())
	}
}

func TestDoctorMainHelpReturns(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	readPipe, writePipe, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("create pipe: %v", pipeErr)
	}
	os.Args = []string{"pinguin-doctor", "--help"}
	os.Stdout = writePipe
	defer func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
		_ = readPipe.Close()
	}()

	main()
	_ = writePipe.Close()
	output, readErr := io.ReadAll(readPipe)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if !strings.Contains(string(output), "Validate one or more Pinguin configuration files") {
		t.Fatalf("unexpected help output %q", string(output))
	}
}

type failingDoctorWriter struct{}

func (writer failingDoctorWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func writeDoctorConfig(t *testing.T, content string) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

const validDoctorConfig = `
server:
  databasePath: /data/pinguin.db
  grpcAuthToken: test-token-123
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 60
  masterEncryptionKey: test-encryption-key-at-least-32-chars
  connectionTimeoutSec: 30
  operationTimeoutSec: 60
web:
  enabled: false
tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
`
