package main

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
	"github.com/tyemirov/pinguin/cmd/client/internal/command"
	cliConfig "github.com/tyemirov/pinguin/cmd/client/internal/config"
	"github.com/tyemirov/pinguin/pkg/client"
	"github.com/tyemirov/pinguin/pkg/logging"
)

func main() {
	v := viper.New()
	cfg, err := cliConfig.Load(v)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	settings, err := client.NewSettings(
		cfg.ServerAddress(),
		cfg.AuthToken(),
		cfg.TenantID(),
		cfg.ConnectionTimeoutSeconds(),
		cfg.OperationTimeoutSeconds(),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	logger := logging.NewLogger(cfg.LogLevel())

	notificationClient, err := client.NewNotificationClient(logger, settings)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer notificationClient.Close()

	root := command.NewRootCommand(command.Dependencies{
		Sender:           notificationClient,
		OperationTimeout: cfg.OperationTimeout(),
		Output:           os.Stdout,
		TenantID:         cfg.TenantID(),
	})
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	if execErr := root.Execute(); execErr != nil {
		fmt.Fprintln(os.Stderr, execErr)
		os.Exit(1)
	}
}
