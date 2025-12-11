package service

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tyemirov/pinguin/internal/model"
)

func TestNormalizeAttachmentsEmail(t *testing.T) {
	t.Helper()
	attachments := []model.EmailAttachment{
		{Filename: " report .pdf ", ContentType: "", Data: []byte("payload")},
	}
	result, err := normalizeAttachments(model.NotificationEmail, attachments)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected single attachment")
	}
	if result[0].Filename != "report .pdf" {
		t.Fatalf("filename was not preserved")
	}
	if result[0].ContentType != defaultAttachmentContentType {
		t.Fatalf("expected default content type, got %s", result[0].ContentType)
	}
	result[0].Data[0] = 'z'
	if attachments[0].Data[0] == 'z' {
		t.Fatalf("expected data copy")
	}
}

func TestNormalizeAttachmentsRejectsSMS(t *testing.T) {
	t.Helper()
	_, err := normalizeAttachments(model.NotificationSMS, []model.EmailAttachment{{Filename: "file", Data: []byte("x")}})
	if err == nil {
		t.Fatalf("expected error for SMS attachments")
	}
}

func TestNormalizeAttachmentsValidation(t *testing.T) {
	t.Helper()
	testCases := []struct {
		name        string
		attachments []model.EmailAttachment
		expectErr   string
	}{
		{
			name:        "MissingFilename",
			attachments: []model.EmailAttachment{{Filename: " ", Data: []byte("x")}},
			expectErr:   "missing filename",
		},
		{
			name:        "EmptyData",
			attachments: []model.EmailAttachment{{Filename: "file.txt"}},
			expectErr:   "empty data",
		},
		{
			name: "TooManyAttachments",
			attachments: func() []model.EmailAttachment {
				var result []model.EmailAttachment
				for i := 0; i < maxAttachmentCount+1; i++ {
					result = append(result, model.EmailAttachment{
						Filename:    "f",
						ContentType: "text/plain",
						Data:        []byte("x"),
					})
				}
				return result
			}(),
			expectErr: "too many attachments",
		},
		{
			name: "AttachmentTooLarge",
			attachments: []model.EmailAttachment{
				{Filename: "big.bin", Data: bytes.Repeat([]byte("x"), maxAttachmentSizeBytes+1)},
			},
			expectErr: "exceeds",
		},
		{
			name: "AggregateTooLarge",
			attachments: func() []model.EmailAttachment {
				chunk := (maxTotalAttachmentSizeBytes / 5) + 1
				if chunk >= maxAttachmentSizeBytes {
					chunk = maxAttachmentSizeBytes - 10
				}
				var attachments []model.EmailAttachment
				for i := 0; i < 6; i++ {
					attachments = append(attachments, model.EmailAttachment{
						Filename:    fmt.Sprintf("file-%d", i),
						ContentType: "text/plain",
						Data:        bytes.Repeat([]byte("x"), chunk),
					})
				}
				return attachments
			}(),
			expectErr: "total limit",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()
			_, err := normalizeAttachments(model.NotificationEmail, testCase.attachments)
			if err == nil || !strings.Contains(err.Error(), testCase.expectErr) {
				t.Fatalf("expected error containing %q, got %v", testCase.expectErr, err)
			}
		})
	}
}
