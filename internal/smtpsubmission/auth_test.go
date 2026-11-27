package smtpsubmission

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tyemirov/pingwin/internal/smtpidentity"
)

func TestSMTPSubmissionTemporaryAuthFailureAllowsRetry(t *testing.T) {
	server := newBareSMTPSubmissionServer(t)
	server.throttle.authFailureLimit = 2
	identity := smtpidentity.AuthenticatedIdentity{ID: "identity-1"}
	storageFailure := errors.New("identity store unavailable")
	testCases := []struct {
		authErr error
		reply   string
	}{
		{smtpidentity.ErrAuthenticationFailed, "535 Authentication failed"},
		{storageFailure, "454 4.7.0 Temporary authentication failure"},
		{storageFailure, "454 4.7.0 Temporary authentication failure"},
		{nil, "235 Authentication successful"},
	}
	ctx := context.Background()
	authArgument := "PLAIN " + plainAuthPayload("smtp-user", "smtp-pass")
	session := &sessionState{}
	for _, testCase := range testCases {
		server.config.Authenticator = authenticatorFunc(func(context.Context, string, string) (smtpidentity.AuthenticatedIdentity, error) {
			return identity, testCase.authErr
		})
		var response bytes.Buffer
		server.handleAuth(ctx, nil, bufio.NewWriter(&response), session, authArgument, "remote", nil)
		if response.String() != testCase.reply+"\r\n" {
			t.Fatalf("AUTH reply = %q, want %q", response.String(), testCase.reply)
		}
		if testCase.authErr != nil && session.authenticated != nil {
			t.Fatal("failed attempt established an authenticated session")
		}
	}
	if session.authenticated == nil || session.authenticated.ID != identity.ID {
		t.Fatalf("successful retry did not establish the identity: %+v", session.authenticated)
	}
}

type authenticatorFunc func(context.Context, string, string) (smtpidentity.AuthenticatedIdentity, error)

func (authenticate authenticatorFunc) Authenticate(ctx context.Context, username, password string) (smtpidentity.AuthenticatedIdentity, error) {
	return authenticate(ctx, username, password)
}

func TestSMTPSubmissionAcceptsMailWithFullUsageQueueAndStopsWorker(t *testing.T) {
	started := make(chan struct{}, 1)
	stopped := make(chan struct{}, 1)
	fixture := newSMTPServerFixtureWithConfig(t, true, nil, func(config *Config) {
		config.MaxConcurrentSessions = 1
		config.IdentityUsageRecorder = identityUsageRecorderFunc(func(ctx context.Context, _ smtpidentity.AuthenticatedIdentity, _ time.Time) error {
			select {
			case started <- struct{}{}:
			default:
			}
			<-ctx.Done()
			select {
			case stopped <- struct{}{}:
			default:
			}
			return ctx.Err()
		})
	})

	for attempt := 0; attempt < 3; attempt++ {
		client := fixture.dial(t)
		defer client.close()
		if err := client.connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set SMTP client deadline: %v", err)
		}
		client.expectCode(t, "220")
		client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
		client.expectCode(t, "235")
		if attempt == 0 {
			receiveWithin(t, started)
		}
		client.send(t, "MAIL FROM:<alice@example.com>")
		client.expectCode(t, "250")
		client.close()
		waitForActiveSMTPSessions(t, fixture.server, 0)
	}

	fixture.cancel()
	receiveWithin(t, fixture.done)
	receiveWithin(t, stopped)
}

func TestSMTPAuthThrottleKeepsOtherReservationsAndCredentialFailures(t *testing.T) {
	throttle := newSMTPThrottle(Config{AuthFailureLimit: 2, AuthFailureWindow: time.Minute})
	reserve := func() smtpAuthReservation {
		t.Helper()
		reservation, allowed := throttle.reserveAuthAttempt("remote", "user")
		if !allowed {
			t.Fatal("expected an available auth reservation")
		}
		return reservation
	}
	assertDenied := func() {
		t.Helper()
		if _, allowed := throttle.reserveAuthAttempt("remote", "user"); allowed {
			t.Fatal("expected auth reservations and failures to fill the limit")
		}
	}
	waiting := reserve()
	successful := reserve()
	assertDenied()
	successful.finish(smtpAuthAccepted)

	rejected := reserve()
	assertDenied()
	rejected.finish(smtpAuthRejected)
	assertDenied()
	waiting.finish(smtpAuthUnavailable)

	secondRejection := reserve()
	secondRejection.finish(smtpAuthRejected)
	assertDenied()
}

type identityUsageRecorderFunc func(context.Context, smtpidentity.AuthenticatedIdentity, time.Time) error

func (record identityUsageRecorderFunc) RecordIdentityUse(ctx context.Context, identity smtpidentity.AuthenticatedIdentity, usedAt time.Time) error {
	return record(ctx, identity, usedAt)
}

func receiveWithin[T any](t *testing.T, channel <-chan T) T {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP test result")
		var zero T
		return zero
	}
}
