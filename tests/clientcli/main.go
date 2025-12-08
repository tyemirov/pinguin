package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tyemirov/pinguin/pkg/attachments"
	"github.com/tyemirov/pinguin/pkg/client"
	"github.com/tyemirov/pinguin/pkg/grpcapi"
	"log/slog"
)

func main() {
	recipient := flag.String("to", "", "Recipient email address")
	subject := flag.String("subject", "", "Email subject")
	message := flag.String("message", "", "Email message body")
	var attachmentInputs multiValueFlag
	flag.Var(&attachmentInputs, "attachment", "Attachment path (repeatable). Use path::content-type to override MIME type")
	flag.Parse()

	if *recipient == "" || *subject == "" || *message == "" {
		fmt.Fprintln(os.Stderr, "Usage: go run ./tests/clientcli --to <recipient> --subject <subject> --message <message>")
		os.Exit(1)
	}

	authToken := os.Getenv("GRPC_AUTH_TOKEN")
	if authToken == "" {
		fmt.Fprintln(os.Stderr, "GRPC_AUTH_TOKEN is required")
		os.Exit(1)
	}

	serverAddress := os.Getenv("GRPC_SERVER_ADDR")
	if serverAddress == "" {
		serverAddress = "localhost:50051"
	}

	connectionTimeoutSec, err := readIntEnv("CONNECTION_TIMEOUT_SEC", 5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid CONNECTION_TIMEOUT_SEC: %v\n", err)
		os.Exit(1)
	}

	operationTimeoutSec, err := readIntEnv("OPERATION_TIMEOUT_SEC", 30)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid OPERATION_TIMEOUT_SEC: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	settings, err := client.NewSettings(serverAddress, authToken, connectionTimeoutSec, operationTimeoutSec)
	if err != nil {
		logger.Error("Failed to validate client settings", "error", err)
		os.Exit(1)
	}

	notificationClient, err := client.NewNotificationClient(logger, settings)
	if err != nil {
		logger.Error("Failed to create notification client", "error", err)
		os.Exit(1)
	}
	defer notificationClient.Close()

	notificationRequest := &grpcapi.NotificationRequest{
		NotificationType: grpcapi.NotificationType_EMAIL,
		Recipient:        *recipient,
		Subject:          *subject,
		Message:          *message,
	}

	attachmentPayloads, err := attachments.Load([]string(attachmentInputs))
	if err != nil {
		logger.Error("Failed to load attachments", "error", err)
		os.Exit(1)
	}
	if len(attachmentPayloads) > 0 {
		notificationRequest.Attachments = attachmentPayloads
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(operationTimeoutSec)*time.Second)
	defer cancel()

	response, err := notificationClient.SendNotification(ctx, notificationRequest)
	if err != nil {
		logger.Error("Failed to send notification", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Notification sent successfully. Notification ID: %s\n", response.NotificationId)
}

func readIntEnv(key string, defaultValue int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("value must be positive")
	}
	return parsed, nil
}

type multiValueFlag []string

func (flagValues *multiValueFlag) String() string {
	if flagValues == nil || len(*flagValues) == 0 {
		return ""
	}
	return strings.Join(*flagValues, ",")
}

func (flagValues *multiValueFlag) Set(value string) error {
	*flagValues = append(*flagValues, value)
	return nil
}
