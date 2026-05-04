package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/tyemirov/pinguin/pkg/client"
	"github.com/tyemirov/pinguin/pkg/grpcapi"
)

type recordingSender struct {
	request *grpcapi.NotificationRequest
	err     error
}

func (sender *recordingSender) SendNotification(_ context.Context, request *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	sender.request = request
	if sender.err != nil {
		return nil, sender.err
	}
	return &grpcapi.NotificationResponse{
		NotificationId: "notif-123",
		Status:         grpcapi.Status_SENT,
	}, nil
}

type recordingCloser struct {
	closed bool
}

func (closer *recordingCloser) Close() error {
	closer.closed = true
	return nil
}

type failingWriter struct{}

func (writer failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestSendCommandSubmitsEmailWithScheduleAndAttachment(t *testing.T) {
	attachmentPath := filepath.Join(t.TempDir(), "hello.txt")
	if err := os.WriteFile(attachmentPath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	scheduledAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	sender := &recordingSender{}
	closer := &recordingCloser{}
	var stdout bytes.Buffer

	command := NewRootCommand(Dependencies{
		NewSender: func(_ *slog.Logger, settings client.Settings) (NotificationSender, io.Closer, error) {
			if settings.ServerAddress() != "smtp.local:50051" {
				t.Fatalf("unexpected server address %s", settings.ServerAddress())
			}
			if settings.AuthToken() != "token" || settings.TenantID() != "tenant-one" {
				t.Fatalf("unexpected settings token=%s tenant=%s", settings.AuthToken(), settings.TenantID())
			}
			return sender, closer, nil
		},
	})
	command.SetOut(&stdout)
	command.SetErr(io.Discard)
	command.SetArgs([]string{
		"send",
		"--grpc-server-addr", "smtp.local:50051",
		"--grpc-auth-token", "token",
		"--tenant-id", "tenant-one",
		"--connection-timeout-sec", "7",
		"--operation-timeout-sec", "9",
		"--log-level", "debug",
		"--type", "email",
		"--recipient", "user@example.com",
		"--subject", "Subject",
		"--message", "Body",
		"--scheduled-time", scheduledAt.Format(time.RFC3339),
		"--attachment", attachmentPath + "::text/plain",
	})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute send: %v", err)
	}
	if !closer.closed {
		t.Fatalf("expected injected closer to be closed")
	}
	if sender.request == nil {
		t.Fatalf("expected request")
	}
	if sender.request.GetNotificationType() != grpcapi.NotificationType_EMAIL {
		t.Fatalf("unexpected notification type %v", sender.request.GetNotificationType())
	}
	if sender.request.GetTenantId() != "tenant-one" || sender.request.GetRecipient() != "user@example.com" {
		t.Fatalf("unexpected request %+v", sender.request)
	}
	if sender.request.GetScheduledTime().AsTime() != scheduledAt {
		t.Fatalf("unexpected scheduled time %s", sender.request.GetScheduledTime().AsTime())
	}
	if len(sender.request.GetAttachments()) != 1 || string(sender.request.GetAttachments()[0].GetData()) != "hello" {
		t.Fatalf("unexpected attachments %+v", sender.request.GetAttachments())
	}
	if !strings.Contains(stdout.String(), "Notification notif-123 sent with status SENT") {
		t.Fatalf("unexpected stdout %q", stdout.String())
	}
}

func TestSendCommandUsesExplicitFlagsForSMS(t *testing.T) {
	t.Setenv("GRPC_SERVER_ADDR", "env.local:50051")
	t.Setenv("GRPC_AUTH_TOKEN", "env-token")
	t.Setenv("TENANT_ID", "tenant-env")
	t.Setenv("CONNECTION_TIMEOUT_SEC", "6")
	t.Setenv("OPERATION_TIMEOUT_SEC", "8")
	t.Setenv("LOG_LEVEL", "warn")

	sender := &recordingSender{}
	command := NewRootCommand(Dependencies{
		NewSender: func(_ *slog.Logger, settings client.Settings) (NotificationSender, io.Closer, error) {
			if settings.ServerAddress() != "flag.local:50051" || settings.AuthToken() != "flag-token" || settings.TenantID() != "tenant-flag" {
				t.Fatalf("unexpected settings from flags")
			}
			return sender, nil, nil
		},
	})
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{
		"send",
		"--grpc-server-addr", "flag.local:50051",
		"--grpc-auth-token", "flag-token",
		"--tenant-id", "tenant-flag",
		"--type", "sms",
		"--recipient", "+15551234567",
		"--message", "OTP",
	})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute send: %v", err)
	}
	if sender.request.GetNotificationType() != grpcapi.NotificationType_SMS {
		t.Fatalf("expected SMS request, got %v", sender.request.GetNotificationType())
	}
	if sender.request.GetSubject() != "" {
		t.Fatalf("expected SMS subject to be empty")
	}
}

func TestSendCommandValidationErrors(t *testing.T) {
	attachmentPath := filepath.Join(t.TempDir(), "hello.txt")
	if err := os.WriteFile(attachmentPath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	senderErr := errors.New("sender factory failed")
	sendErr := errors.New("send failed")

	testCases := []struct {
		name       string
		args       []string
		sender     *recordingSender
		factoryErr error
		wantErr    string
	}{
		{name: "missing auth", args: []string{"send", "--tenant-id", "tenant"}, wantErr: "grpc-auth-token is required"},
		{name: "missing tenant", args: []string{"send", "--grpc-auth-token", "token"}, wantErr: "tenant-id is required"},
		{name: "invalid timeout", args: validSendArgs("--connection-timeout-sec", "0"), wantErr: "connection-timeout-sec must be positive"},
		{name: "invalid type", args: validSendArgs("--type", "push"), wantErr: "invalid notification type"},
		{name: "missing recipient", args: validSendArgs("--recipient", ""), wantErr: "recipient is required"},
		{name: "missing message", args: validSendArgs("--message", ""), wantErr: "message is required"},
		{name: "missing subject", args: validSendArgs("--subject", ""), wantErr: "subject is required"},
		{name: "invalid schedule", args: validSendArgs("--scheduled-time", "tomorrow"), wantErr: "invalid scheduled time"},
		{name: "missing attachment", args: validSendArgs("--attachment", filepath.Join(t.TempDir(), "missing.txt")), wantErr: "open"},
		{name: "sms attachment", args: validSendArgs("--type", "sms", "--subject", "", "--attachment", attachmentPath), wantErr: "attachments are only supported"},
		{name: "factory error", args: validSendArgs(), factoryErr: senderErr, wantErr: senderErr.Error()},
		{name: "send error", args: validSendArgs(), sender: &recordingSender{err: sendErr}, wantErr: sendErr.Error()},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			sender := testCase.sender
			if sender == nil {
				sender = &recordingSender{}
			}
			command := NewRootCommand(Dependencies{
				NewSender: func(_ *slog.Logger, _ client.Settings) (NotificationSender, io.Closer, error) {
					return sender, nil, testCase.factoryErr
				},
			})
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			command.SetArgs(testCase.args)

			err := command.Execute()
			if err == nil {
				t.Fatalf("expected error containing %q", testCase.wantErr)
			}
			if !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected error containing %q, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestSendCommandReturnsWriteError(t *testing.T) {
	command := NewRootCommand(Dependencies{
		NewSender: func(_ *slog.Logger, _ client.Settings) (NotificationSender, io.Closer, error) {
			return &recordingSender{}, nil, nil
		},
	})
	command.SetOut(failingWriter{})
	command.SetErr(io.Discard)
	command.SetArgs(validSendArgs())

	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected write failure, got %v", err)
	}
}

func TestSendCommandDefaultSenderReportsRPCFailure(t *testing.T) {
	command := NewRootCommand(Dependencies{})
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs(validSendArgs(
		"--grpc-server-addr", "127.0.0.1:1",
		"--operation-timeout-sec", "1",
	))

	err := command.Execute()
	if err == nil {
		t.Fatalf("expected default sender RPC failure")
	}
}

func TestSendCommandDefaultSenderReportsConstructorFailure(t *testing.T) {
	command := NewRootCommand(Dependencies{})
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs(validSendArgs("--grpc-server-addr", "dns:///%"))

	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid URL escape") {
		t.Fatalf("expected default sender constructor failure, got %v", err)
	}
}

func TestSendCommandReportsEnvironmentLoadError(t *testing.T) {
	t.Setenv("CONNECTION_TIMEOUT_SEC", "not-an-int")
	command := NewRootCommand(Dependencies{
		NewSender: func(_ *slog.Logger, _ client.Settings) (NotificationSender, io.Closer, error) {
			t.Fatalf("sender should not be constructed when config load fails")
			return nil, nil, nil
		},
	})
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"send"})
	if err := command.Execute(); err == nil {
		t.Fatalf("expected environment config error")
	}
}

func TestSendCommandReportsInternalFlagResolutionErrors(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(*cobra.Command)
	}{
		{name: "server address", setup: func(*cobra.Command) {}},
		{name: "auth token", setup: func(cmd *cobra.Command) {
			cmd.Flags().String("grpc-server-addr", "localhost:50051", "")
		}},
		{name: "tenant id", setup: func(cmd *cobra.Command) {
			cmd.Flags().String("grpc-server-addr", "localhost:50051", "")
			cmd.Flags().String("grpc-auth-token", "", "")
			_ = cmd.Flags().Set("grpc-auth-token", "token")
		}},
		{name: "connection timeout", setup: func(cmd *cobra.Command) {
			cmd.Flags().String("grpc-server-addr", "localhost:50051", "")
			cmd.Flags().String("grpc-auth-token", "", "")
			cmd.Flags().String("tenant-id", "", "")
			_ = cmd.Flags().Set("grpc-auth-token", "token")
			_ = cmd.Flags().Set("tenant-id", "tenant")
		}},
		{name: "operation timeout", setup: func(cmd *cobra.Command) {
			cmd.Flags().String("grpc-server-addr", "localhost:50051", "")
			cmd.Flags().String("grpc-auth-token", "", "")
			cmd.Flags().String("tenant-id", "", "")
			cmd.Flags().Int("connection-timeout-sec", 5, "")
			_ = cmd.Flags().Set("grpc-auth-token", "token")
			_ = cmd.Flags().Set("tenant-id", "tenant")
		}},
		{name: "log level", setup: func(cmd *cobra.Command) {
			cmd.Flags().String("grpc-server-addr", "localhost:50051", "")
			cmd.Flags().String("grpc-auth-token", "", "")
			cmd.Flags().String("tenant-id", "", "")
			cmd.Flags().Int("connection-timeout-sec", 5, "")
			cmd.Flags().Int("operation-timeout-sec", 30, "")
			_ = cmd.Flags().Set("grpc-auth-token", "token")
			_ = cmd.Flags().Set("tenant-id", "tenant")
		}},
		{name: "settings", setup: func(cmd *cobra.Command) {
			cmd.Flags().String("grpc-server-addr", "", "")
			cmd.Flags().String("grpc-auth-token", "", "")
			cmd.Flags().String("tenant-id", "", "")
			cmd.Flags().Int("connection-timeout-sec", 5, "")
			cmd.Flags().Int("operation-timeout-sec", 30, "")
			cmd.Flags().String("log-level", "INFO", "")
			_ = cmd.Flags().Set("grpc-server-addr", " ")
			_ = cmd.Flags().Set("grpc-auth-token", "token")
			_ = cmd.Flags().Set("tenant-id", "tenant")
		}},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			command := buildSendCommand(Dependencies{
				NewSender: func(_ *slog.Logger, _ client.Settings) (NotificationSender, io.Closer, error) {
					t.Fatalf("sender should not be constructed")
					return nil, nil, nil
				},
			})
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			command.SetArgs(validStandaloneSendArgs())
			testCase.setup(command)
			if err := command.Execute(); err == nil {
				t.Fatalf("expected internal flag resolution error")
			}
		})
	}
}

func TestParseNotificationTypeDefault(t *testing.T) {
	notificationType, err := parseNotificationType("")
	if err != nil {
		t.Fatalf("parse empty type: %v", err)
	}
	if notificationType != grpcapi.NotificationType_EMAIL {
		t.Fatalf("expected email default, got %v", notificationType)
	}
}

func TestFlagHelpersReportUnknownAndTypedErrors(t *testing.T) {
	rootCommand := &cobra.Command{Use: "root"}
	childCommand := &cobra.Command{Use: "child"}
	rootCommand.AddCommand(childCommand)
	rootCommand.PersistentFlags().Bool("bad-bool", false, "bad bool")
	if err := rootCommand.PersistentFlags().Set("bad-bool", "true"); err != nil {
		t.Fatalf("set bool flag: %v", err)
	}
	childCommand.Flags().Bool("local-bool", false, "local bool")
	if err := childCommand.Flags().Set("local-bool", "true"); err != nil {
		t.Fatalf("set local bool: %v", err)
	}
	childCommand.Flags().String("local-string", "value", "local string")
	if err := childCommand.Flags().Set("local-string", "value"); err != nil {
		t.Fatalf("set local string: %v", err)
	}

	if _, err := valueOrConfig(childCommand, "missing", ""); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown string flag error, got %v", err)
	}
	if _, err := valueOrConfig(childCommand, "local-bool", ""); err == nil {
		t.Fatalf("expected local string type error")
	}
	if _, err := valueOrConfig(childCommand, "bad-bool", ""); err == nil {
		t.Fatalf("expected inherited string type error")
	}
	directInheritedString := &cobra.Command{Use: "direct"}
	directInheritedString.InheritedFlags().String("direct-string", "default", "")
	if err := directInheritedString.InheritedFlags().Set("direct-string", "changed"); err != nil {
		t.Fatalf("set direct inherited string: %v", err)
	}
	if value, err := valueOrConfig(directInheritedString, "direct-string", "config"); err != nil || value != "changed" {
		t.Fatalf("expected direct inherited string, got value=%q err=%v", value, err)
	}

	if _, err := intOrConfig(childCommand, "missing-int", 1); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown int flag error, got %v", err)
	}
	if _, err := intOrConfig(childCommand, "local-string", 1); err == nil {
		t.Fatalf("expected local int type error")
	}
	rootCommand.PersistentFlags().String("bad-string", "value", "bad string")
	if err := rootCommand.PersistentFlags().Set("bad-string", "value"); err != nil {
		t.Fatalf("set inherited string: %v", err)
	}
	if _, err := intOrConfig(childCommand, "bad-string", 1); err == nil {
		t.Fatalf("expected inherited int type error")
	}
	directInheritedInt := &cobra.Command{Use: "direct"}
	directInheritedInt.InheritedFlags().Int("direct-int", 1, "")
	if err := directInheritedInt.InheritedFlags().Set("direct-int", "8"); err != nil {
		t.Fatalf("set direct inherited int: %v", err)
	}
	if value, err := intOrConfig(directInheritedInt, "direct-int", 3); err != nil || value != 8 {
		t.Fatalf("expected direct inherited int, got value=%d err=%v", value, err)
	}
	if _, err := intOrConfig(childCommand, "connection-timeout-sec", 0); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown flag on synthetic command, got %v", err)
	}
}

func TestFlagHelpersReturnLocalInheritedAndConfigValues(t *testing.T) {
	rootCommand := &cobra.Command{Use: "root"}
	childCommand := &cobra.Command{Use: "child"}
	rootCommand.AddCommand(childCommand)
	rootCommand.PersistentFlags().String("inherited-string", "default", "string")
	rootCommand.PersistentFlags().Int("inherited-int", 1, "int")
	childCommand.Flags().String("local-string", "default", "string")
	childCommand.Flags().Int("local-int", 1, "int")

	if err := childCommand.Flags().Set("local-string", "local"); err != nil {
		t.Fatalf("set local string: %v", err)
	}
	if value, err := valueOrConfig(childCommand, "local-string", "config"); err != nil || value != "local" {
		t.Fatalf("expected local string, got value=%q err=%v", value, err)
	}

	if value, err := valueOrConfig(childCommand, "inherited-string", "config"); err != nil || value != "config" {
		t.Fatalf("expected inherited config fallback, got value=%q err=%v", value, err)
	}
	if err := rootCommand.PersistentFlags().Set("inherited-string", "inherited"); err != nil {
		t.Fatalf("set inherited string: %v", err)
	}
	if value, err := valueOrConfig(childCommand, "inherited-string", "config"); err != nil || value != "inherited" {
		t.Fatalf("expected inherited string, got value=%q err=%v", value, err)
	}

	if err := childCommand.Flags().Set("local-int", "7"); err != nil {
		t.Fatalf("set local int: %v", err)
	}
	if value, err := intOrConfig(childCommand, "local-int", 3); err != nil || value != 7 {
		t.Fatalf("expected local int, got value=%d err=%v", value, err)
	}
	childCommand.Flags().Lookup("local-int").Changed = false
	if value, err := intOrConfig(childCommand, "local-int", 3); err != nil || value != 3 {
		t.Fatalf("expected local config int, got value=%d err=%v", value, err)
	}

	if err := rootCommand.PersistentFlags().Set("inherited-int", "9"); err != nil {
		t.Fatalf("set inherited int: %v", err)
	}
	if value, err := intOrConfig(childCommand, "inherited-int", 3); err != nil || value != 9 {
		t.Fatalf("expected inherited int, got value=%d err=%v", value, err)
	}

	inheritedOnlyCommand := &cobra.Command{Use: "child"}
	inheritedOnlyCommand.InheritedFlags().Int("inherited-config-int", 1, "int")
	if _, err := intOrConfig(inheritedOnlyCommand, "inherited-config-int", 0); err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Fatalf("expected positive config validation, got %v", err)
	}
	if value, err := intOrConfig(inheritedOnlyCommand, "inherited-config-int", 7); err != nil || value != 7 {
		t.Fatalf("expected inherited config int, got value=%d err=%v", value, err)
	}

	if err := childCommand.Flags().Set("local-int", "0"); err != nil {
		t.Fatalf("set local int zero: %v", err)
	}
	if _, err := intOrConfig(childCommand, "local-int", 3); err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Fatalf("expected positive local int validation, got %v", err)
	}
	childCommand.Flags().Lookup("local-int").Changed = false
	if err := rootCommand.PersistentFlags().Set("inherited-int", "0"); err != nil {
		t.Fatalf("set inherited int zero: %v", err)
	}
	zeroInheritedCommand := &cobra.Command{Use: "child"}
	zeroInheritedCommand.InheritedFlags().Int("inherited-int", 1, "int")
	if err := zeroInheritedCommand.InheritedFlags().Set("inherited-int", "0"); err != nil {
		t.Fatalf("set direct inherited int zero: %v", err)
	}
	if _, err := intOrConfig(zeroInheritedCommand, "inherited-int", 3); err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Fatalf("expected positive inherited int validation, got %v", err)
	}
}

func validSendArgs(overrides ...string) []string {
	baseFlags := map[string]struct{}{
		"--grpc-server-addr":       {},
		"--grpc-auth-token":        {},
		"--tenant-id":              {},
		"--connection-timeout-sec": {},
		"--operation-timeout-sec":  {},
		"--type":                   {},
		"--recipient":              {},
		"--subject":                {},
		"--message":                {},
	}
	values := map[string]string{
		"--grpc-server-addr":       "localhost:50051",
		"--grpc-auth-token":        "token",
		"--tenant-id":              "tenant",
		"--connection-timeout-sec": "5",
		"--operation-timeout-sec":  "30",
		"--type":                   "email",
		"--recipient":              "user@example.com",
		"--subject":                "Subject",
		"--message":                "Body",
	}
	for index := 0; index < len(overrides); index += 2 {
		values[overrides[index]] = overrides[index+1]
	}
	args := []string{"send"}
	for _, flagName := range []string{
		"--grpc-server-addr",
		"--grpc-auth-token",
		"--tenant-id",
		"--connection-timeout-sec",
		"--operation-timeout-sec",
		"--type",
		"--recipient",
		"--subject",
		"--message",
	} {
		args = append(args, flagName, values[flagName])
	}
	for index := 0; index < len(overrides); index += 2 {
		if _, known := baseFlags[overrides[index]]; !known {
			args = append(args, overrides[index], overrides[index+1])
		}
	}
	return args
}

func validStandaloneSendArgs() []string {
	return []string{
		"--type", "email",
		"--recipient", "user@example.com",
		"--subject", "Subject",
		"--message", "Body",
	}
}
