package smtpsubmission

import (
	"errors"
	"fmt"
	"net/textproto"
	"testing"
)

func TestUpstreamRelayErrorClassifiesSMTPFailures(t *testing.T) {
	testCases := []struct {
		name        string
		upstreamErr error
		expectedErr error
	}{
		{
			name:        "PermanentSMTP",
			upstreamErr: fmt.Errorf("mail failed: %w", &textproto.Error{Code: 550, Msg: "mailbox unavailable"}),
			expectedErr: ErrRelayPermanent,
		},
		{
			name:        "TemporarySMTP",
			upstreamErr: fmt.Errorf("mail failed: %w", &textproto.Error{Code: 451, Msg: "try again later"}),
			expectedErr: ErrRelayTemporary,
		},
		{
			name:        "Generic",
			upstreamErr: errors.New("network timeout"),
			expectedErr: ErrRelayTemporary,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			classifiedErr := upstreamRelayError(testCase.upstreamErr)
			if !errors.Is(classifiedErr, testCase.expectedErr) {
				t.Fatalf("expected %v, got %v", testCase.expectedErr, classifiedErr)
			}
		})
	}
}
