package model

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

const (
	sampleRecipient   = "user@example.com"
	sampleMessage     = "Body"
	sampleFilename    = "file.txt"
	sampleContentType = "text/plain"
)

func TestNewNotificationRequestValidation(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name             string
		notificationType NotificationType
		recipient        string
		message          string
		attachments      []EmailAttachment
		expectedError    error
	}{
		{
			name:             "MissingRecipient",
			notificationType: NotificationEmail,
			recipient:        " ",
			message:          sampleMessage,
			expectedError:    ErrNotificationRecipientRequired,
		},
		{
			name:             "MissingMessage",
			notificationType: NotificationEmail,
			recipient:        sampleRecipient,
			message:          " ",
			expectedError:    ErrNotificationMessageRequired,
		},
		{
			name:             "UnsupportedType",
			notificationType: NotificationType("push"),
			recipient:        sampleRecipient,
			message:          sampleMessage,
			expectedError:    ErrNotificationTypeUnsupported,
		},
		{
			name:             "AttachmentsNotAllowedForSMS",
			notificationType: NotificationSMS,
			recipient:        "+15555550100",
			message:          sampleMessage,
			attachments: []EmailAttachment{
				{
					Filename:    "note.txt",
					ContentType: sampleContentType,
					Data:        []byte("data"),
				},
			},
			expectedError: ErrNotificationAttachmentsNotAllowed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			_, requestErr := NewNotificationRequest(
				testCase.notificationType,
				testCase.recipient,
				"",
				testCase.message,
				nil,
				testCase.attachments,
			)
			if !errors.Is(requestErr, testCase.expectedError) {
				t.Fatalf("expected error %v, got %v", testCase.expectedError, requestErr)
			}
		})
	}
}

func TestNewNotificationRequestNormalizesAttachments(t *testing.T) {
	t.Helper()

	originalData := []byte{0x01, 0x02}
	request, requestErr := NewNotificationRequest(
		NotificationEmail,
		sampleRecipient,
		"",
		sampleMessage,
		nil,
		[]EmailAttachment{
			{
				Filename: "report.txt",
				Data:     originalData,
			},
		},
	)
	if requestErr != nil {
		t.Fatalf("notification request error: %v", requestErr)
	}

	originalData[0] = 0x03
	attachments := request.Attachments()
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	if attachments[0].ContentType != defaultAttachmentContentType {
		t.Fatalf("expected default content type, got %s", attachments[0].ContentType)
	}
	if attachments[0].Data[0] == originalData[0] {
		t.Fatalf("expected attachment data to be copied")
	}

	attachments[0].Data[0] = 0x04
	attachmentsAgain := request.Attachments()
	if attachmentsAgain[0].Data[0] == attachments[0].Data[0] {
		t.Fatalf("expected attachment copies to be independent")
	}
}

func TestNewNotificationRequestAttachmentValidation(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name          string
		attachments   []EmailAttachment
		expectedError error
	}{
		{
			name:          "MissingFilename",
			attachments:   []EmailAttachment{{Filename: " ", Data: []byte("x")}},
			expectedError: ErrNotificationAttachmentFilenameRequired,
		},
		{
			name:          "EmptyData",
			attachments:   []EmailAttachment{{Filename: sampleFilename}},
			expectedError: ErrNotificationAttachmentDataRequired,
		},
		{
			name: "TooManyAttachments",
			attachments: func() []EmailAttachment {
				result := make([]EmailAttachment, 0, maxNotificationAttachmentCount+1)
				for attachmentIndex := 0; attachmentIndex < maxNotificationAttachmentCount+1; attachmentIndex++ {
					result = append(result, EmailAttachment{
						Filename:    sampleFilename,
						ContentType: sampleContentType,
						Data:        []byte("x"),
					})
				}
				return result
			}(),
			expectedError: ErrNotificationAttachmentsTooMany,
		},
		{
			name: "AttachmentTooLarge",
			attachments: []EmailAttachment{
				{
					Filename: "big.bin",
					Data:     bytes.Repeat([]byte("x"), maxNotificationAttachmentSizeBytes+1),
				},
			},
			expectedError: ErrNotificationAttachmentTooLarge,
		},
		{
			name: "AggregateTooLarge",
			attachments: func() []EmailAttachment {
				chunkSize := (maxNotificationAttachmentsTotalBytes / 5) + 1
				if chunkSize >= maxNotificationAttachmentSizeBytes {
					chunkSize = maxNotificationAttachmentSizeBytes - 10
				}
				result := make([]EmailAttachment, 0, 6)
				for attachmentIndex := 0; attachmentIndex < 6; attachmentIndex++ {
					result = append(result, EmailAttachment{
						Filename:    fmt.Sprintf("file-%d", attachmentIndex),
						ContentType: sampleContentType,
						Data:        bytes.Repeat([]byte("x"), chunkSize),
					})
				}
				return result
			}(),
			expectedError: ErrNotificationAttachmentsTooLarge,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			_, requestErr := NewNotificationRequest(
				NotificationEmail,
				sampleRecipient,
				"",
				sampleMessage,
				nil,
				testCase.attachments,
			)
			if !errors.Is(requestErr, testCase.expectedError) {
				t.Fatalf("expected error %v, got %v", testCase.expectedError, requestErr)
			}
		})
	}
}
