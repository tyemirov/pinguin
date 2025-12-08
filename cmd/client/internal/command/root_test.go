package command

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/pkg/grpcapi"
)

type stubClient struct {
	requests []*grpcapi.NotificationRequest
	err      error
}

func (clientInstance *stubClient) SendNotification(_ context.Context, req *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	clientInstance.requests = append(clientInstance.requests, req)
	if clientInstance.err != nil {
		return nil, clientInstance.err
	}
	return &grpcapi.NotificationResponse{
		NotificationId: "test-id",
		Status:         grpcapi.Status_SENT,
	}, nil
}

func TestSendCommandBuildsRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		args           []string
		expectedType   grpcapi.NotificationType
		expectedErr    string
		expectSchedule bool
		expectedTime   time.Time
	}{
		{
			name: "email without schedule",
			args: []string{
				"send",
				"--type", "email",
				"--recipient", "user@example.com",
				"--subject", "Subj",
				"--message", "Body",
			},
			expectedType: grpcapi.NotificationType_EMAIL,
		},
		{
			name: "sms with schedule",
			args: []string{
				"send",
				"--type", "sms",
				"--recipient", "+15551234567",
				"--message", "OTP",
				"--scheduled-time", "2025-01-02T15:04:05Z",
			},
			expectedType:   grpcapi.NotificationType_SMS,
			expectSchedule: true,
			expectedTime:   time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC),
		},
		{
			name: "missing type fails",
			args: []string{
				"send",
				"--recipient", "user@example.com",
				"--subject", "Subj",
				"--message", "Body",
			},
			expectedErr: "required flag(s) \"type\" not set",
		},
		{
			name: "invalid type fails",
			args: []string{
				"send",
				"--type", "push",
				"--recipient", "user@example.com",
				"--subject", "Subj",
				"--message", "Body",
			},
			expectedErr: "invalid notification type \"push\"",
		},
		{
			name: "missing message fails",
			args: []string{
				"send",
				"--type", "email",
				"--recipient", "user@example.com",
				"--subject", "Subj",
			},
			expectedErr: "required flag(s) \"message\" not set",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			stub := &stubClient{}
			deps := Dependencies{
				Sender:           stub,
				OperationTimeout: 2 * time.Second,
				Output:           &bytes.Buffer{},
			}
			cmd := NewRootCommand(deps)
			cmd.SetArgs(testCase.args)

			err := cmd.Execute()
			if testCase.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error %q but got none", testCase.expectedErr)
				}
				if !strings.Contains(err.Error(), testCase.expectedErr) {
					t.Fatalf("expected error %q, got %q", testCase.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if len(stub.requests) != 1 {
				t.Fatalf("expected 1 request, got %d", len(stub.requests))
			}
			request := stub.requests[0]
			if request.NotificationType != testCase.expectedType {
				t.Fatalf("expected type %v, got %v", testCase.expectedType, request.NotificationType)
			}
			if testCase.expectSchedule && request.ScheduledTime == nil {
				t.Fatalf("expected schedule to be set")
			}
			if !testCase.expectSchedule && request.ScheduledTime != nil {
				t.Fatalf("expected schedule to be nil")
			}
			if testCase.expectSchedule {
				resultTime := request.ScheduledTime.AsTime()
				if !resultTime.Equal(testCase.expectedTime) {
					t.Fatalf("expected scheduled time %v, got %v", testCase.expectedTime, resultTime)
				}
			}
		})
	}
}

func TestSendCommandHandlesClientError(t *testing.T) {
	t.Parallel()

	stub := &stubClient{
		err: context.DeadlineExceeded,
	}
	deps := Dependencies{
		Sender:           stub,
		OperationTimeout: time.Second,
		Output:           &bytes.Buffer{},
	}
	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{
		"send",
		"--type", "email",
		"--recipient", "user@example.com",
		"--subject", "Subj",
		"--message", "Body",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error but got none")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("expected deadline exceeded error, got %v", err)
	}
	if len(stub.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(stub.requests))
	}
}

func TestSendCommandFormatsOutput(t *testing.T) {
	t.Parallel()

	stub := &stubClient{}
	output := &bytes.Buffer{}
	deps := Dependencies{
		Sender:           stub,
		OperationTimeout: time.Second,
		Output:           output,
	}
	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{
		"send",
		"--type", "email",
		"--recipient", "user@example.com",
		"--subject", "Subj",
		"--message", "Body",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(stub.requests) != 1 {
		t.Fatalf("expected one request to be recorded, got %d", len(stub.requests))
	}
	if !strings.Contains(output.String(), "test-id") {
		t.Fatalf("expected output to contain notification id, got %s", output.String())
	}
	if !strings.Contains(output.String(), grpcapi.Status_SENT.String()) {
		t.Fatalf("expected output to contain status, got %s", output.String())
	}
}

func TestSendCommandLoadsAttachments(t *testing.T) {
	t.Parallel()

	tempFile := filepath.Join(t.TempDir(), "hello.txt")
	if writeErr := os.WriteFile(tempFile, []byte("hello world"), 0o600); writeErr != nil {
		t.Fatalf("write temp file: %v", writeErr)
	}

	stub := &stubClient{}
	output := &bytes.Buffer{}
	deps := Dependencies{
		Sender:           stub,
		OperationTimeout: time.Second,
		Output:           output,
	}
	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{
		"send",
		"--type", "email",
		"--recipient", "user@example.com",
		"--subject", "Subj",
		"--message", "Body",
		"--attachment", tempFile + "::text/plain",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(stub.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(stub.requests))
	}
	req := stub.requests[0]
	if len(req.Attachments) != 1 {
		t.Fatalf("expected one attachment, got %d", len(req.Attachments))
	}
	if req.Attachments[0].Filename != "hello.txt" {
		t.Fatalf("unexpected filename %s", req.Attachments[0].Filename)
	}
	if string(req.Attachments[0].Data) != "hello world" {
		t.Fatalf("unexpected attachment content")
	}
}

func TestSendCommandRejectsAttachmentsForSms(t *testing.T) {
	t.Parallel()

	tempFile := filepath.Join(t.TempDir(), "hello.txt")
	if writeErr := os.WriteFile(tempFile, []byte("hello"), 0o600); writeErr != nil {
		t.Fatalf("write temp file: %v", writeErr)
	}

	stub := &stubClient{}
	deps := Dependencies{
		Sender:           stub,
		OperationTimeout: time.Second,
		Output:           &bytes.Buffer{},
	}
	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{
		"send",
		"--type", "sms",
		"--recipient", "+15551234567",
		"--message", "Body",
		"--attachment", tempFile,
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when using attachments with sms")
	}
	if !strings.Contains(err.Error(), "attachments are only supported for email") {
		t.Fatalf("unexpected error %v", err)
	}
}
