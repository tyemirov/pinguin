package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/tyemirov/pinguin/internal/config"
	"log/slog"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestTwilioSmsSenderSuccess(t *testing.T) {
	t.Helper()
	var captured struct {
		method string
		url    string
		body   string
		auth   string
	}
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			captured.method = req.Method
			captured.url = req.URL.String()
			body, _ := io.ReadAll(req.Body)
			captured.body = string(body)
			user, pass, _ := req.BasicAuth()
			captured.auth = user + ":" + pass
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	sender := &TwilioSmsSender{
		AccountSID: "sid",
		AuthToken:  "token",
		FromNumber: "+1000",
		HTTPClient: client,
		Logger:     newDiscardLogger(),
	}

	resp, err := sender.SendSms(context.Background(), "+1222", "Hello")
	if err != nil {
		t.Fatalf("SendSms returned error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response %q", resp)
	}
	if captured.method != http.MethodPost {
		t.Fatalf("expected POST, got %s", captured.method)
	}
	if captured.auth != "sid:token" {
		t.Fatalf("unexpected auth %s", captured.auth)
	}
	if captured.body == "" {
		t.Fatalf("expected body to be populated")
	}
}

func TestTwilioSmsSenderErrorStatus(t *testing.T) {
	t.Helper()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 500,
				Body:       io.NopCloser(bytes.NewBufferString("fail")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	sender := &TwilioSmsSender{
		AccountSID: "sid",
		AuthToken:  "token",
		FromNumber: "+1000",
		HTTPClient: client,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if _, err := sender.SendSms(context.Background(), "+1222", "Hello"); err == nil {
		t.Fatalf("expected error for non-2xx response")
	}
}

func TestTwilioSmsSenderConstructorAndRequestFailures(t *testing.T) {
	constructed := NewTwilioSmsSender("sid", "token", "+1000", newDiscardLogger(), config.Config{ConnectionTimeoutSec: 3})
	if constructed.AccountSID != "sid" || constructed.HTTPClient == nil {
		t.Fatalf("unexpected constructed sender %+v", constructed)
	}

	badURLSender := &TwilioSmsSender{
		AccountSID: "bad\nsid",
		AuthToken:  "token",
		FromNumber: "+1000",
		HTTPClient: http.DefaultClient,
		Logger:     newDiscardLogger(),
	}
	if _, err := badURLSender.SendSms(context.Background(), "+1222", "Hello"); err == nil {
		t.Fatalf("expected request creation error")
	}

	doError := errors.New("twilio unavailable")
	httpErrorSender := &TwilioSmsSender{
		AccountSID: "sid",
		AuthToken:  "token",
		FromNumber: "+1000",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, doError
		})},
		Logger: newDiscardLogger(),
	}
	if _, err := httpErrorSender.SendSms(context.Background(), "+1222", "Hello"); !errors.Is(err, doError) {
		t.Fatalf("expected HTTP client error, got %v", err)
	}
}
