package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/tenant"
	"github.com/tyemirov/pinguin/internal/tenantconversion"
	"gorm.io/gorm"
)

func TestMainUsesRunExitCode(t *testing.T) {
	originalArgs := os.Args
	originalExit := exitProcess
	t.Cleanup(func() {
		os.Args = originalArgs
		exitProcess = originalExit
	})
	os.Args = []string{"convert-managed-tenants"}
	exitCode := -1
	exitProcess = func(code int) { exitCode = code }
	main()
	if exitCode != 2 {
		t.Fatalf("main exit code = %d", exitCode)
	}
}

func TestRunValidationAndIOFailures(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	getenv := func(string) string { return strings.Repeat("a", 64) }
	baseArgs := []string{"-database", filepath.Join(t.TempDir(), "legacy.db"), "-tenant-source", "source.yml", "-mapping", "mapping.yml", "-confirm", conversionConfirmation}

	if code := run([]string{"-unknown"}, &stdout, &stderr, getenv); code != 2 {
		t.Fatalf("parse failure code = %d", code)
	}
	stderr.Reset()
	if code := run(nil, &stdout, &stderr, getenv); code != 2 || !strings.Contains(stderr.String(), "are required") {
		t.Fatalf("required flags = %d %s", code, stderr.String())
	}
	stderr.Reset()
	badConfirmation := append([]string{}, baseArgs...)
	badConfirmation[len(badConfirmation)-1] = "wrong"
	if code := run(badConfirmation, &stdout, &stderr, getenv); code != 2 || !strings.Contains(stderr.String(), "confirm must equal") {
		t.Fatalf("confirmation = %d %s", code, stderr.String())
	}
	stderr.Reset()
	if code := run(baseArgs, &stdout, &stderr, func(string) string { return "" }); code != 2 || !strings.Contains(stderr.String(), "environment variable") {
		t.Fatalf("missing environment = %d %s", code, stderr.String())
	}
	stderr.Reset()
	if code := run(baseArgs, &stdout, &stderr, getenv); code != 1 || !strings.Contains(stderr.String(), "read tenant source") {
		t.Fatalf("source read = %d %s", code, stderr.String())
	}

	sourcePath := filepath.Join(t.TempDir(), "source.yml")
	if err := os.WriteFile(sourcePath, []byte("tenants: []\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	args := append([]string{}, baseArgs...)
	args[3] = sourcePath
	stderr.Reset()
	if code := run(args, &stdout, &stderr, getenv); code != 1 || !strings.Contains(stderr.String(), "read conversion mapping") {
		t.Fatalf("mapping read = %d %s", code, stderr.String())
	}

	mappingPath := filepath.Join(t.TempDir(), "mapping.yml")
	if err := os.WriteFile(mappingPath, []byte("tenants: []\nsmtpSenderDomains: []\nsmtpIdentities: []\n"), 0o600); err != nil {
		t.Fatalf("write mapping: %v", err)
	}
	args[5] = mappingPath
	if err := os.WriteFile(sourcePath, []byte("unknown: true\n"), 0o600); err != nil {
		t.Fatalf("write invalid source: %v", err)
	}
	stderr.Reset()
	if code := run(args, &stdout, &stderr, getenv); code != 1 || !strings.Contains(stderr.String(), "source yaml") {
		t.Fatalf("source decode = %d %s", code, stderr.String())
	}
	if err := os.WriteFile(sourcePath, []byte("tenants: []\n"), 0o600); err != nil {
		t.Fatalf("restore source: %v", err)
	}
	if err := os.WriteFile(mappingPath, []byte("unknown: true\n"), 0o600); err != nil {
		t.Fatalf("write invalid mapping: %v", err)
	}
	stderr.Reset()
	if code := run(args, &stdout, &stderr, getenv); code != 1 || !strings.Contains(stderr.String(), "mapping yaml") {
		t.Fatalf("mapping decode = %d %s", code, stderr.String())
	}
	if err := os.WriteFile(mappingPath, []byte("tenants: []\nsmtpSenderDomains: []\nsmtpIdentities: []\n"), 0o600); err != nil {
		t.Fatalf("restore mapping: %v", err)
	}
	stderr.Reset()
	if code := run(args, &stdout, &stderr, func(string) string { return "short" }); code != 1 || !strings.Contains(stderr.String(), "initialize secret keeper failed") {
		t.Fatalf("keeper failure = %d %s", code, stderr.String())
	}
	stderr.Reset()
	if code := run(args, &stdout, &stderr, getenv); code != 1 || !strings.Contains(stderr.String(), "source schema") {
		t.Fatalf("conversion failure = %d %s", code, stderr.String())
	}
}

func TestRunDependencyFailuresAndSuccess(t *testing.T) {
	keeper, err := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("keeper: %v", err)
	}
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "conversion.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("database: %v", err)
	}
	dependencies := commandDependencies{
		readFile:      func(string) ([]byte, error) { return []byte("fixture"), nil },
		decodeSource:  func([]byte) (tenantconversion.SourceConfig, error) { return tenantconversion.SourceConfig{}, nil },
		decodeMapping: func([]byte) (tenantconversion.Mapping, error) { return tenantconversion.Mapping{}, nil },
		newKeeper:     func(string) (*tenant.SecretKeeper, error) { return keeper, nil },
		openDatabase:  func(string) (*gorm.DB, error) { return database, nil },
		convert: func(context.Context, *gorm.DB, *tenant.SecretKeeper, tenantconversion.SourceConfig, tenantconversion.Mapping) (tenantconversion.Result, error) {
			return tenantconversion.Result{Tenants: 1, Notifications: 2, Attachments: 3, SMTPSenderDomains: 4, SMTPIdentities: 5, ForwardingRoutes: 6}, nil
		},
	}
	args := []string{"-database", "database.db", "-tenant-source", "source.yml", "-mapping", "mapping.yml", "-confirm", conversionConfirmation}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runWithDependencies(args, &stdout, &stderr, func(string) string { return "key" }, dependencies)
	if code != 0 || !strings.Contains(stdout.String(), "tenants=1 notifications=2 attachments=3 smtp_domains=4 smtp_identities=5 forwarding_routes=6") {
		t.Fatalf("success = %d %s %s", code, stdout.String(), stderr.String())
	}

	dependencies.openDatabase = func(string) (*gorm.DB, error) { return nil, errors.New("open blocked") }
	stderr.Reset()
	if code := runWithDependencies(args, &stdout, &stderr, func(string) string { return "key" }, dependencies); code != 1 || !strings.Contains(stderr.String(), "open database") {
		t.Fatalf("open failure = %d %s", code, stderr.String())
	}
	dependencies.openDatabase = func(string) (*gorm.DB, error) { return database, nil }
	dependencies.convert = func(context.Context, *gorm.DB, *tenant.SecretKeeper, tenantconversion.SourceConfig, tenantconversion.Mapping) (tenantconversion.Result, error) {
		return tenantconversion.Result{}, errors.New("conversion blocked")
	}
	stderr.Reset()
	if code := runWithDependencies(args, &stdout, &stderr, func(string) string { return "key" }, dependencies); code != 1 || !strings.Contains(stderr.String(), "conversion blocked") {
		t.Fatalf("convert failure = %d %s", code, stderr.String())
	}
}
