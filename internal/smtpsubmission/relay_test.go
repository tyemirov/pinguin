package smtpsubmission

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/textproto"
	"testing"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
)

func TestUpstreamRelayBuildsSenderFromConfig(t *testing.T) {
	relay := NewUpstreamRelay(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{
		SMTPSubmission: config.SMTPSubmissionConfig{
			Relay: config.SMTPSubmissionRelayConfig{
				Host:     "smtp.example.com",
				Port:     587,
				Username: "smtp-user",
				Password: "smtp-pass",
			},
		},
	})
	if relay.logger == nil || relay.sender == nil {
		t.Fatalf("expected relay dependencies")
	}
}

func TestUpstreamRelayForwardsRawMessagesAndMapsErrors(t *testing.T) {
	from := mustSMTPSubmissionAddress(t, "alice@example.com")
	recipient := mustSMTPSubmissionAddress(t, "recipient@example.net")
	message := RawMessage{
		IdentityID: "identity-one",
		From:       from,
		Recipients: []smtpidentity.Address{recipient},
		Data:       []byte("From: alice@example.com\r\n\r\nHello"),
	}

	testCases := []struct {
		name       string
		sendErr    error
		wantErr    error
		wantCalled bool
	}{
		{name: "success", wantCalled: true},
		{name: "temporary", sendErr: &textproto.Error{Code: 451, Msg: "try later"}, wantErr: ErrRelayTemporary, wantCalled: true},
		{name: "permanent", sendErr: &textproto.Error{Code: 550, Msg: "rejected"}, wantErr: ErrRelayPermanent, wantCalled: true},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			sender := &recordingRawEmailSender{err: testCase.sendErr}
			relay := &UpstreamRelay{
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
				sender: sender,
			}
			relayErr := relay.Relay(context.Background(), message)
			if testCase.wantErr == nil && relayErr != nil {
				t.Fatalf("relay returned error: %v", relayErr)
			}
			if testCase.wantErr != nil && !errors.Is(relayErr, testCase.wantErr) {
				t.Fatalf("expected %v, got %v", testCase.wantErr, relayErr)
			}
			if sender.called != testCase.wantCalled {
				t.Fatalf("unexpected sender call state")
			}
			if sender.from != "alice@example.com" || len(sender.recipients) != 1 || sender.recipients[0] != "recipient@example.net" {
				t.Fatalf("unexpected relay envelope from=%s recipients=%v", sender.from, sender.recipients)
			}
			if string(sender.data) != string(message.Data) {
				t.Fatalf("raw message data was not preserved")
			}
		})
	}
}

func TestRawMessageRecipientStrings(t *testing.T) {
	first := mustSMTPSubmissionAddress(t, "one@example.net")
	second := mustSMTPSubmissionAddress(t, "two@example.net")
	recipients := (RawMessage{Recipients: []smtpidentity.Address{first, second}}).RecipientStrings()
	if len(recipients) != 2 || recipients[0] != "one@example.net" || recipients[1] != "two@example.net" {
		t.Fatalf("unexpected recipient strings %v", recipients)
	}
}

func TestIsPermanentSMTPErrorRejectsNonSMTPError(t *testing.T) {
	if isPermanentSMTPError(errors.New("plain error")) {
		t.Fatalf("plain errors should not be treated as permanent SMTP errors")
	}
}

type recordingRawEmailSender struct {
	called     bool
	from       string
	recipients []string
	data       []byte
	err        error
}

func (sender *recordingRawEmailSender) SendRawEmail(_ context.Context, fromAddress string, recipients []string, rawMessage []byte) error {
	sender.called = true
	sender.from = fromAddress
	sender.recipients = append([]string(nil), recipients...)
	sender.data = append([]byte(nil), rawMessage...)
	return sender.err
}

func mustSMTPSubmissionAddress(t *testing.T, rawAddress string) smtpidentity.Address {
	t.Helper()
	address, addressErr := smtpidentity.NewAddress(rawAddress)
	if addressErr != nil {
		t.Fatalf("new address: %v", addressErr)
	}
	return address
}
