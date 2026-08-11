package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/tenant"
	"github.com/tyemirov/pinguin/internal/tenantconversion"
	"gorm.io/gorm"
)

const conversionConfirmation = "managed-tenant-conversion"

var exitProcess = os.Exit

func main() {
	exitProcess(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

func run(args []string, stdout io.Writer, stderr io.Writer, getenv func(string) string) int {
	return runWithDependencies(args, stdout, stderr, getenv, commandDependencies{
		readFile:      os.ReadFile,
		decodeSource:  tenantconversion.DecodeSource,
		decodeMapping: tenantconversion.DecodeMapping,
		newKeeper:     tenant.NewSecretKeeper,
		openDatabase: func(path string) (*gorm.DB, error) {
			return gorm.Open(sqlite.Open(path+"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"), &gorm.Config{})
		},
		convert: tenantconversion.Convert,
	})
}

type commandDependencies struct {
	readFile      func(string) ([]byte, error)
	decodeSource  func([]byte) (tenantconversion.SourceConfig, error)
	decodeMapping func([]byte) (tenantconversion.Mapping, error)
	newKeeper     func(string) (*tenant.SecretKeeper, error)
	openDatabase  func(string) (*gorm.DB, error)
	convert       func(context.Context, *gorm.DB, *tenant.SecretKeeper, tenantconversion.SourceConfig, tenantconversion.Mapping) (tenantconversion.Result, error)
}

func runWithDependencies(args []string, stdout io.Writer, stderr io.Writer, getenv func(string) string, dependencies commandDependencies) int {
	flags := flag.NewFlagSet("convert-managed-tenants", flag.ContinueOnError)
	flags.SetOutput(stderr)
	databasePath := flags.String("database", "", "path to the write-stopped Pinguin SQLite database")
	sourcePath := flags.String("tenant-source", "", "path to the current tenant YAML")
	mappingPath := flags.String("mapping", "", "path to the complete conversion mapping YAML")
	masterKeyEnvironment := flags.String("master-key-env", "PINGUIN_MASTER_ENCRYPTION_KEY", "environment variable containing the current master encryption key")
	confirmation := flags.String("confirm", "", "required destructive-operation confirmation")
	if parseErr := flags.Parse(args); parseErr != nil {
		return 2
	}
	if strings.TrimSpace(*databasePath) == "" || strings.TrimSpace(*sourcePath) == "" || strings.TrimSpace(*mappingPath) == "" || strings.TrimSpace(*masterKeyEnvironment) == "" {
		fmt.Fprintln(stderr, "database, tenant-source, mapping, and master-key-env are required")
		return 2
	}
	if *confirmation != conversionConfirmation {
		fmt.Fprintf(stderr, "confirm must equal %s\n", conversionConfirmation)
		return 2
	}
	masterKey := strings.TrimSpace(getenv(*masterKeyEnvironment))
	if masterKey == "" {
		fmt.Fprintf(stderr, "environment variable %s is required\n", *masterKeyEnvironment)
		return 2
	}
	sourceContents, sourceReadErr := dependencies.readFile(*sourcePath)
	if sourceReadErr != nil {
		fmt.Fprintf(stderr, "read tenant source: %v\n", sourceReadErr)
		return 1
	}
	mappingContents, mappingReadErr := dependencies.readFile(*mappingPath)
	if mappingReadErr != nil {
		fmt.Fprintf(stderr, "read conversion mapping: %v\n", mappingReadErr)
		return 1
	}
	source, sourceErr := dependencies.decodeSource(sourceContents)
	if sourceErr != nil {
		fmt.Fprintln(stderr, sourceErr)
		return 1
	}
	mapping, mappingErr := dependencies.decodeMapping(mappingContents)
	if mappingErr != nil {
		fmt.Fprintln(stderr, mappingErr)
		return 1
	}
	keeper, keeperErr := dependencies.newKeeper(masterKey)
	if keeperErr != nil {
		fmt.Fprintln(stderr, "initialize secret keeper failed")
		return 1
	}
	database, databaseErr := dependencies.openDatabase(*databasePath)
	if databaseErr != nil {
		fmt.Fprintf(stderr, "open database: %v\n", databaseErr)
		return 1
	}
	result, conversionErr := dependencies.convert(context.Background(), database, keeper, source, mapping)
	if conversionErr != nil {
		fmt.Fprintln(stderr, conversionErr)
		return 1
	}
	fmt.Fprintf(stdout, "managed tenant conversion complete: tenants=%d notifications=%d attachments=%d smtp_domains=%d smtp_identities=%d forwarding_routes=%d\n",
		result.Tenants, result.Notifications, result.Attachments, result.SMTPSenderDomains, result.SMTPIdentities, result.ForwardingRoutes)
	return 0
}
