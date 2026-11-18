package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// CloudWatchConfig explicitly identifies the delivery log and its read-only credential.
type CloudWatchConfig struct {
	Region, AccountID, LogGroup, ConfigurationSet string
	AccessKeyID, SecretAccessKey, SessionToken    string
}

// CloudWatchReader reads SES metadata through the official CloudWatch Logs client.
type CloudWatchReader struct {
	client *cloudwatchlogs.Client
	config CloudWatchConfig
}

const maximumProviderPages = 100
const maximumProviderEvents = 50000

// NewCloudWatchReader builds a client without an ambient credential discovery chain.
func NewCloudWatchReader(config CloudWatchConfig, options ...func(*cloudwatchlogs.Options)) (*CloudWatchReader, error) {
	for _, value := range []string{config.Region, config.AccountID, config.LogGroup, config.ConfigurationSet, config.AccessKeyID, config.SecretAccessKey} {
		if strings.TrimSpace(value) == "" {
			return nil, errors.New("delivery CloudWatch: region, account, log group, configuration set and credentials are required")
		}
	}
	client := cloudwatchlogs.NewFromConfig(aws.Config{Region: config.Region, Credentials: credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, config.SessionToken)}, options...)
	return &CloudWatchReader{client: client, config: config}, nil
}

// Read retrieves every provider page within the caller's time window.
func (reader *CloudWatchReader) Read(ctx context.Context, start, end time.Time) ([]Event, error) {
	input := &cloudwatchlogs.FilterLogEventsInput{LogGroupName: aws.String(reader.config.LogGroup), StartTime: aws.Int64(start.UnixMilli()), EndTime: aws.Int64(end.UnixMilli())}
	events := []Event{}
	seenTokens := map[string]bool{}
	pages, recordCount := 0, 0
	for {
		pages++
		if pages > maximumProviderPages {
			return nil, errors.New("delivery CloudWatch: event scan exceeds page limit")
		}
		page, err := reader.client.FilterLogEvents(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("delivery CloudWatch filter events: %w", err)
		}
		recordCount += len(page.Events)
		if recordCount > maximumProviderEvents {
			return nil, errors.New("delivery CloudWatch: event scan exceeds record limit")
		}
		for _, record := range page.Events {
			event, relevant, decodeErr := decodeEvent(aws.ToString(record.Message), reader.config)
			if decodeErr != nil {
				return nil, fmt.Errorf("delivery CloudWatch event: %w", decodeErr)
			}
			if relevant {
				events = append(events, event)
			}
		}
		token := aws.ToString(page.NextToken)
		if token == "" {
			break
		}
		if seenTokens[token] {
			return nil, errors.New("delivery CloudWatch: repeated pagination token")
		}
		seenTokens[token] = true
		input.NextToken = aws.String(token)
	}
	return events, nil
}

type sesEnvelope struct {
	Source  string    `json:"source"`
	Account string    `json:"account"`
	Region  string    `json:"region"`
	Time    time.Time `json:"time"`
	Detail  struct {
		EventType string `json:"eventType"`
		Mail      struct {
			Timestamp        time.Time `json:"timestamp"`
			Source           string    `json:"source"`
			MessageID        string    `json:"messageId"`
			Destination      []string  `json:"destination"`
			HeadersTruncated bool      `json:"headersTruncated"`
			Headers          []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
			CommonHeaders struct {
				Subject string `json:"subject"`
			} `json:"commonHeaders"`
			Tags map[string][]string `json:"tags"`
		} `json:"mail"`
		Delivery struct {
			Timestamp    time.Time `json:"timestamp"`
			Recipients   []string  `json:"recipients"`
			SMTPResponse string    `json:"smtpResponse"`
		} `json:"delivery"`
		Bounce struct {
			Timestamp  time.Time      `json:"timestamp"`
			Recipients []sesRecipient `json:"bouncedRecipients"`
		} `json:"bounce"`
		Complaint struct {
			Timestamp  time.Time      `json:"timestamp"`
			Recipients []sesRecipient `json:"complainedRecipients"`
			Type       string         `json:"complaintFeedbackType"`
		} `json:"complaint"`
		Delay struct {
			Timestamp  time.Time      `json:"timestamp"`
			Recipients []sesRecipient `json:"delayedRecipients"`
			Type       string         `json:"delayType"`
		} `json:"deliveryDelay"`
		Reject struct {
			Reason string `json:"reason"`
		} `json:"reject"`
		Failure struct {
			Message string `json:"errorMessage"`
		} `json:"failure"`
	} `json:"detail"`
}
type sesRecipient struct {
	Address    string `json:"emailAddress"`
	Diagnostic string `json:"diagnosticCode"`
}

func decodeEvent(raw string, config CloudWatchConfig) (Event, bool, error) {
	var envelope sesEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return Event{}, false, errors.New("invalid SES event JSON")
	}
	tags := envelope.Detail.Mail.Tags["ses:configuration-set"]
	if envelope.Source != "aws.ses" || envelope.Account != config.AccountID || envelope.Region != config.Region || len(tags) != 1 || tags[0] != config.ConfigurationSet {
		return Event{}, false, nil
	}
	detail := envelope.Detail
	sender, err := mail.ParseAddress(detail.Mail.Source)
	if err != nil || detail.Mail.MessageID == "" || detail.Mail.Timestamp.IsZero() || envelope.Time.IsZero() || len(detail.Mail.Destination) == 0 {
		return Event{}, false, errors.New("invalid SES event metadata")
	}
	event := Event{id: detail.Mail.MessageID, sender: strings.ToLower(sender.Address), subject: detail.Mail.CommonHeaders.Subject, kind: detail.EventType, sentAt: detail.Mail.Timestamp.UTC(), occurredAt: envelope.Time.UTC(), destinations: []string{}}
	for _, rawAddress := range detail.Mail.Destination {
		address, err := mail.ParseAddress(rawAddress)
		if err != nil {
			return Event{}, false, errors.New("invalid SES destination")
		}
		event.destinations = append(event.destinations, strings.ToLower(address.Address))
	}
	add := func(address, status, response string, at time.Time) {
		event.outcomes = append(event.outcomes, outcome{address: strings.ToLower(address), status: status, response: response, at: at})
		if at.After(event.occurredAt) {
			event.occurredAt = at
		}
	}
	switch detail.EventType {
	case "Send":
	case "Delivery":
		if detail.Delivery.Timestamp.IsZero() {
			return Event{}, false, errors.New("invalid SES delivery timestamp")
		}
		for _, address := range detail.Delivery.Recipients {
			add(address, statusDelivered, detail.Delivery.SMTPResponse, detail.Delivery.Timestamp)
		}
	case "Bounce":
		if detail.Bounce.Timestamp.IsZero() {
			return Event{}, false, errors.New("invalid SES bounce timestamp")
		}
		for _, recipient := range detail.Bounce.Recipients {
			add(recipient.Address, statusBounced, recipient.Diagnostic, detail.Bounce.Timestamp)
		}
	case "Complaint":
		if detail.Complaint.Timestamp.IsZero() {
			return Event{}, false, errors.New("invalid SES complaint timestamp")
		}
		for _, recipient := range detail.Complaint.Recipients {
			add(recipient.Address, statusComplaint, detail.Complaint.Type, detail.Complaint.Timestamp)
		}
	case "DeliveryDelay":
		if detail.Delay.Timestamp.IsZero() {
			return Event{}, false, errors.New("invalid SES delay timestamp")
		}
		for _, recipient := range detail.Delay.Recipients {
			add(recipient.Address, statusDelayed, recipient.Diagnostic, detail.Delay.Timestamp)
		}
	case "Reject", "Rendering Failure":
		status, response := statusRejected, detail.Reject.Reason
		if detail.EventType == "Rendering Failure" {
			status, response = statusRenderingFailed, detail.Failure.Message
		}
		for _, address := range event.destinations {
			add(address, status, response, event.occurredAt)
		}
	default:
		return Event{}, false, errors.New("unsupported SES delivery event type")
	}
	if !detail.Mail.HeadersTruncated {
		forwarded := false
		messageID := ""
		for _, header := range detail.Mail.Headers {
			if strings.EqualFold(header.Name, "X-Pingwin-Original-From") {
				forwarded = true
			}
			if strings.EqualFold(header.Name, "Message-ID") {
				messageID = strings.Trim(header.Value, "<> \t")
			}
		}
		id, host, found := strings.Cut(messageID, "@")
		if forwarded && found && strings.HasSuffix(host, ".amazonses.com") && id != event.id {
			event.parent = id
		}
	}
	return event, true, nil
}
