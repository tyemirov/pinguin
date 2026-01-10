// Command pinguin-doctor validates Pinguin configurations across projects.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tyemirov/pinguin/internal/doctor"
)

const (
	flagCrossValidate = "cross-validate"
	flagExpandEnv     = "expand-env"
	flagOutputJSON    = "json"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "pinguin-doctor [config-paths...]",
		Short: "Validate Pinguin configurations and report issues",
		Long: `Validate one or more Pinguin configuration files and report any issues.

The doctor command performs comprehensive validation including:
- Configuration file syntax and structure
- Server configuration requirements (database, auth token, encryption key)
- Web interface configuration (when enabled)
- Tenant configuration requirements (domains, identity, admins)
- Cross-config validation (when multiple configs are provided)

Examples:
  pinguin-doctor config.yml
  pinguin-doctor config.yml other-config.yml --cross-validate
  pinguin-doctor ./configs/*.yml --json
  pinguin-doctor config.yml --expand-env`,
		Args: cobra.MinimumNArgs(1),
		RunE: runDoctor,
	}

	command.Flags().Bool(flagCrossValidate, false, "Validate cross-config consistency (domains, google client IDs)")
	command.Flags().Bool(flagExpandEnv, false, "Expand environment variables in config files before validation")
	command.Flags().Bool(flagOutputJSON, false, "Output results as JSON instead of human-readable summary")

	return command
}

func runDoctor(command *cobra.Command, arguments []string) error {
	crossValidate, crossErr := command.Flags().GetBool(flagCrossValidate)
	if crossErr != nil {
		return crossErr
	}
	expandEnv, expandErr := command.Flags().GetBool(flagExpandEnv)
	if expandErr != nil {
		return expandErr
	}
	outputJSON, jsonErr := command.Flags().GetBool(flagOutputJSON)
	if jsonErr != nil {
		return jsonErr
	}

	options := doctor.Options{
		ConfigPaths:          arguments,
		ValidateCrossConfigs: crossValidate,
		ExpandEnv:            expandEnv,
	}

	report, runErr := doctor.Run(context.Background(), options)
	if runErr != nil {
		return runErr
	}

	var output []byte
	if outputJSON {
		formatted, formatErr := doctor.FormatReport(report)
		if formatErr != nil {
			return fmt.Errorf("doctor.format_json: %w", formatErr)
		}
		output = formatted
	} else {
		output = []byte(doctor.FormatSummary(report))
	}

	if _, writeErr := command.OutOrStdout().Write(output); writeErr != nil {
		return fmt.Errorf("doctor.write_output: %w", writeErr)
	}

	if report.Summary.InvalidConfigs > 0 || len(report.CrossValidation.Errors) > 0 {
		return fmt.Errorf("doctor: validation failed (%d invalid configs, %d cross-config errors)",
			report.Summary.InvalidConfigs, len(report.CrossValidation.Errors))
	}

	return nil
}
