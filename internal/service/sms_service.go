package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"log/slog"

	"github.com/tyemirov/pinguin/internal/config"
)

type SmsSender interface {
	SendSms(ctx context.Context, recipient string, message string) (string, error)
}

type TwilioSmsSender struct {
	AccountSID string
	AuthToken  string
	FromNumber string
	HTTPClient *http.Client
	Logger     *slog.Logger
}

func NewTwilioSmsSender(accountSID string, authToken string, fromNumber string, logger *slog.Logger, cfg config.Config) *TwilioSmsSender {
	return &TwilioSmsSender{
		AccountSID: accountSID,
		AuthToken:  authToken,
		FromNumber: fromNumber,
		HTTPClient: &http.Client{Timeout: time.Duration(cfg.ConnectionTimeoutSec) * time.Second},
		Logger:     logger,
	}
}

func (senderInstance *TwilioSmsSender) SendSms(ctx context.Context, recipient string, message string) (string, error) {
	formData := url.Values{}
	formData.Set("To", recipient)
	formData.Set("From", senderInstance.FromNumber)
	formData.Set("Body", message)

	apiEndpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", senderInstance.AccountSID)
	requestInstance, requestError := http.NewRequestWithContext(ctx, http.MethodPost, apiEndpoint, strings.NewReader(formData.Encode()))
	if requestError != nil {
		senderInstance.Logger.Error("Failed to create Twilio request", "error", requestError)
		return "", requestError
	}
	requestInstance.SetBasicAuth(senderInstance.AccountSID, senderInstance.AuthToken)
	requestInstance.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	responseInstance, responseError := senderInstance.HTTPClient.Do(requestInstance)
	if responseError != nil {
		senderInstance.Logger.Error("Twilio request error", "error", responseError)
		return "", responseError
	}
	defer responseInstance.Body.Close()

	responseBody, _ := io.ReadAll(responseInstance.Body)
	if responseInstance.StatusCode >= 300 {
		senderInstance.Logger.Error("Twilio API returned error", "status", responseInstance.StatusCode, "body", string(responseBody))
		return "", fmt.Errorf("twilio API error: %s", string(responseBody))
	}

	return string(responseBody), nil
}
