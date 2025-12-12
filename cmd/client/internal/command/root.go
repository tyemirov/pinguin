package command

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cliConfig "github.com/tyemirov/pinguin/cmd/client/internal/config"
	"github.com/tyemirov/pinguin/pkg/attachments"
	"github.com/tyemirov/pinguin/pkg/client"
	"github.com/tyemirov/pinguin/pkg/grpcapi"
	"github.com/tyemirov/pinguin/pkg/logging"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log/slog"
)

type NotificationSender interface {
	SendNotification(context.Context, *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error)
}

type Dependencies struct {
	NewSender func(logger *slog.Logger, settings client.Settings) (NotificationSender, io.Closer, error)
}

func NewRootCommand(dependencies Dependencies) *cobra.Command {
	root := &cobra.Command{
		Use:           "pinguin-cli",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().String("grpc-server-addr", "localhost:50051", "Target gRPC endpoint")
	root.PersistentFlags().String("grpc-auth-token", "", "Bearer token used for gRPC authentication")
	root.PersistentFlags().String("tenant-id", "", "Tenant identifier used for requests")
	root.PersistentFlags().Int("connection-timeout-sec", 5, "Dial timeout in seconds")
	root.PersistentFlags().Int("operation-timeout-sec", 30, "Per-command timeout in seconds")
	root.PersistentFlags().String("log-level", "INFO", "CLI log level (DEBUG, INFO, WARN, ERROR)")

	root.AddCommand(buildSendCommand(dependencies))
	return root
}

func buildSendCommand(dependencies Dependencies) *cobra.Command {
	var (
		typeInput      string
		recipientInput string
		subjectInput   string
		messageInput   string
		scheduledInput string
		attachmentArgs []string
	)

	command := &cobra.Command{
		Use:   "send",
		Short: "Submit a notification to the Pinguin service",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFromEnv, err := cliConfig.Load(viper.New())
			if err != nil {
				return err
			}

			serverAddress, err := valueOrConfig(cmd, "grpc-server-addr", configFromEnv.ServerAddress())
			if err != nil {
				return err
			}

			authToken, err := valueOrConfig(cmd, "grpc-auth-token", configFromEnv.AuthToken())
			if err != nil {
				return err
			}
			if strings.TrimSpace(authToken) == "" {
				return fmt.Errorf("grpc-auth-token is required")
			}

			tenantID, err := valueOrConfig(cmd, "tenant-id", configFromEnv.TenantID())
			if err != nil {
				return err
			}
			tenantID = strings.TrimSpace(tenantID)
			if tenantID == "" {
				return fmt.Errorf("tenant-id is required")
			}

			connectionTimeoutSec, err := intOrConfig(cmd, "connection-timeout-sec", configFromEnv.ConnectionTimeoutSeconds())
			if err != nil {
				return err
			}
			operationTimeoutSec, err := intOrConfig(cmd, "operation-timeout-sec", configFromEnv.OperationTimeoutSeconds())
			if err != nil {
				return err
			}

			logLevel, err := valueOrConfig(cmd, "log-level", configFromEnv.LogLevel())
			if err != nil {
				return err
			}

			settings, err := client.NewSettings(serverAddress, authToken, tenantID, connectionTimeoutSec, operationTimeoutSec)
			if err != nil {
				return fmt.Errorf("invalid client settings: %w", err)
			}

			logger := logging.NewLogger(logLevel)

			newSender := dependencies.NewSender
			if newSender == nil {
				newSender = func(logger *slog.Logger, settings client.Settings) (NotificationSender, io.Closer, error) {
					notificationClient, err := client.NewNotificationClient(logger, settings)
					if err != nil {
						return nil, nil, err
					}
					return notificationClient, notificationClient, nil
				}
			}

			sender, closer, err := newSender(logger, settings)
			if err != nil {
				return err
			}
			if closer != nil {
				defer closer.Close()
			}

			notificationType, err := parseNotificationType(typeInput)
			if err != nil {
				return err
			}

			recipient := strings.TrimSpace(recipientInput)
			if recipient == "" {
				return fmt.Errorf("recipient is required")
			}

			message := strings.TrimSpace(messageInput)
			if message == "" {
				return fmt.Errorf("message is required")
			}

			subject := strings.TrimSpace(subjectInput)
			if notificationType == grpcapi.NotificationType_EMAIL && subject == "" {
				return fmt.Errorf("subject is required for email notifications")
			}

			request := &grpcapi.NotificationRequest{
				TenantId:         tenantID,
				NotificationType: notificationType,
				Recipient:        recipient,
				Subject:          subject,
				Message:          message,
			}

			attachmentPayloads, attachmentErr := attachments.Load(attachmentArgs)
			if attachmentErr != nil {
				return attachmentErr
			}
			if notificationType == grpcapi.NotificationType_SMS && len(attachmentPayloads) > 0 {
				return fmt.Errorf("attachments are only supported for email notifications")
			}
			request.Attachments = attachmentPayloads

			if scheduledInput != "" {
				scheduledTime, parseErr := time.Parse(time.RFC3339, scheduledInput)
				if parseErr != nil {
					return fmt.Errorf("invalid scheduled time %q: %w", scheduledInput, parseErr)
				}
				request.ScheduledTime = timestamppb.New(scheduledTime.UTC())
			}

			timeout := settings.OperationTimeout()

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			response, sendErr := sender.SendNotification(ctx, request)
			if sendErr != nil {
				return sendErr
			}

			_, writeErr := fmt.Fprintf(
				cmd.OutOrStdout(),
				"Notification %s sent with status %s\n",
				response.NotificationId,
				response.Status.String(),
			)
			if writeErr != nil {
				return writeErr
			}

			return nil
		},
	}

	command.Flags().StringVar(&typeInput, "type", "email", "Notification type (email or sms)")
	command.Flags().StringVar(&recipientInput, "recipient", "", "Notification recipient")
	command.Flags().StringVar(&recipientInput, "to", "", "Alias for --recipient")
	command.Flags().StringVar(&subjectInput, "subject", "", "Email subject (ignored for sms)")
	command.Flags().StringVar(&messageInput, "message", "", "Notification message")
	command.Flags().StringVar(&scheduledInput, "scheduled-time", "", "RFC3339 timestamp for scheduled delivery")
	command.Flags().StringArrayVar(&attachmentArgs, "attachment", nil, "Attachment path (repeatable). Use path::content-type to override MIME type")

	return command
}

func parseNotificationType(input string) (grpcapi.NotificationType, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "":
		return grpcapi.NotificationType_EMAIL, nil
	case "email":
		return grpcapi.NotificationType_EMAIL, nil
	case "sms":
		return grpcapi.NotificationType_SMS, nil
	default:
		return grpcapi.NotificationType_EMAIL, fmt.Errorf("invalid notification type %q", input)
	}
}

func valueOrConfig(cmd *cobra.Command, flagName string, configValue string) (string, error) {
	localFlag := cmd.Flags().Lookup(flagName)
	if localFlag != nil {
		if localFlag.Changed {
			return cmd.Flags().GetString(flagName)
		}
		return configValue, nil
	}

	inheritedFlag := cmd.InheritedFlags().Lookup(flagName)
	if inheritedFlag == nil {
		return "", fmt.Errorf("unknown flag %q", flagName)
	}
	if inheritedFlag.Changed {
		return cmd.InheritedFlags().GetString(flagName)
	}
	return configValue, nil
}

func intOrConfig(cmd *cobra.Command, flagName string, configValue int) (int, error) {
	localFlag := cmd.Flags().Lookup(flagName)
	if localFlag != nil {
		if localFlag.Changed {
			value, err := cmd.Flags().GetInt(flagName)
			if err != nil {
				return 0, err
			}
			if value <= 0 {
				return 0, fmt.Errorf("%s must be positive", flagName)
			}
			return value, nil
		}
		return configValue, nil
	}

	inheritedFlag := cmd.InheritedFlags().Lookup(flagName)
	if inheritedFlag == nil {
		return 0, fmt.Errorf("unknown flag %q", flagName)
	}
	if inheritedFlag.Changed {
		value, err := cmd.InheritedFlags().GetInt(flagName)
		if err != nil {
			return 0, err
		}
		if value <= 0 {
			return 0, fmt.Errorf("%s must be positive", flagName)
		}
		return value, nil
	}

	if configValue <= 0 {
		return 0, fmt.Errorf("%s must be positive", flagName)
	}
	return configValue, nil
}
