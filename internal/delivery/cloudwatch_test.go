package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

type cloudWatchHTTPClient http.HandlerFunc

func (client cloudWatchHTTPClient) Do(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	client(recorder, request)
	return recorder.Result(), nil
}

func TestCloudWatchReadsAllPagesAndUsesExplicitCredential(t *testing.T) {
	count := 0
	provider := cloudWatchHTTPClient(func(writer http.ResponseWriter, request *http.Request) {
		count++
		if !strings.Contains(request.Header.Get("Authorization"), "Credential=test-key/") {
			t.Error("explicit credential not used")
		}
		var input map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if input["logGroupName"] != "/delivery" || input["startTime"] != float64(deliveryNow.Add(-Retention).UnixMilli()) || input["endTime"] != float64(deliveryNow.UnixMilli()) {
			t.Errorf("wrong bounded query: %+v", input)
		}
		writer.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if count%2 == 1 {
			_, _ = io.WriteString(writer, `{"events":[],"nextToken":"next"}`)
			return
		}
		if input["nextToken"] != "next" {
			t.Error("empty page cursor lost")
		}
		envelope := testEnvelope("delivered", "sender@example.com", "test", deliveryNow.Add(-time.Hour), "Delivery", []string{"to@example.net"})
		envelope["time"] = deliveryNow.Add(-30 * time.Minute)
		raw, _ := json.Marshal(envelope)
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"events": []map[string]interface{}{{"message": string(raw)}}})
	})
	reader, err := NewCloudWatchReader(testCloudConfig, func(options *cloudwatchlogs.Options) {
		options.HTTPClient = provider
	})
	if err != nil {
		t.Fatal(err)
	}
	events, err := reader.Read(context.Background(), deliveryNow.Add(-Retention), deliveryNow)
	if err != nil || len(events) != 1 || events[0].id != "delivered" || count != 2 {
		t.Fatalf("events=%+v, error=%v, requests=%d", events, err, count)
	}
	service, _ := NewService(reader, func() time.Time { return deliveryNow })
	scope, _ := NewScope("tenant", []Domain{{Name: "example.com", RegisteredAt: deliveryNow.Add(-2 * time.Hour)}})
	query, _ := NewQuery("", "", "")
	page, err := service.List(context.Background(), scope, query)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].Recipients[0].Status != "delivered" {
		t.Fatalf("missing Send and delayed envelope lost delivery: %+v, %v", page, err)
	}
}

func TestCloudWatchProviderFailures(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		response string
		status   int
	}{
		{name: "malformed event", response: `{"events":[{"message":"broken"}]}`},
		{name: "repeated token", response: `{"events":[],"nextToken":"repeat"}`},
		{name: "provider denied", response: `{"__type":"AccessDeniedException","message":"denied"}`, status: 400},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			provider := cloudWatchHTTPClient(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/x-amz-json-1.1")
				if testCase.status != 0 {
					writer.WriteHeader(testCase.status)
				}
				_, _ = io.WriteString(writer, testCase.response)
			})
			reader, _ := NewCloudWatchReader(testCloudConfig, func(options *cloudwatchlogs.Options) {
				options.HTTPClient = provider
				options.RetryMaxAttempts = 1
			})
			if _, err := reader.Read(context.Background(), deliveryNow.Add(-Retention), deliveryNow); err == nil {
				t.Fatal("provider failure hidden")
			}
		})
	}
	reader, _ := NewCloudWatchReader(testCloudConfig)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Read(ctx, deliveryNow.Add(-Retention), deliveryNow); err == nil {
		t.Fatal("cancellation hidden")
	}
	bad := testCloudConfig
	bad.AccessKeyID = ""
	if _, err := NewCloudWatchReader(bad); err == nil {
		t.Fatal("missing credential accepted")
	}
}

func TestCloudWatchRejectsIncompleteScans(t *testing.T) {
	for _, mode := range []string{"pages", "records"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			provider := cloudWatchHTTPClient(func(writer http.ResponseWriter, _ *http.Request) {
				calls++
				if mode == "pages" {
					_ = json.NewEncoder(writer).Encode(map[string]interface{}{"events": []interface{}{}, "nextToken": fmt.Sprint(calls)})
					return
				}
				events := make([]map[string]string, maximumProviderEvents+1)
				for index := range events {
					events[index] = map[string]string{"message": `{"source":"unrelated"}`}
				}
				_ = json.NewEncoder(writer).Encode(map[string]interface{}{"events": events})
			})
			reader, _ := NewCloudWatchReader(testCloudConfig, func(options *cloudwatchlogs.Options) { options.HTTPClient = provider })
			if _, err := reader.Read(context.Background(), deliveryNow.Add(-Retention), deliveryNow); err == nil {
				t.Fatal("incomplete scan returned success")
			}
		})
	}
}

func TestCloudWatchValidatesSESBoundaryAndEventOutcomes(t *testing.T) {
	for _, field := range []string{"source", "account", "region"} {
		t.Run("foreign "+field, func(t *testing.T) {
			envelope := testEnvelope("id", "sender@example.com", "test", deliveryNow, "Send", []string{"to@example.net"})
			envelope[field] = "foreign"
			raw, _ := json.Marshal(envelope)
			_, relevant, err := decodeEvent(string(raw), testCloudConfig)
			if err != nil || relevant {
				t.Fatal("foreign envelope accepted")
			}
		})
	}
	base := func() map[string]interface{} {
		return testEnvelope("id", "sender@example.com", "test", deliveryNow.Add(-time.Hour), "Send", []string{"to@example.net"})
	}
	for _, testCase := range []struct {
		name   string
		change func(map[string]interface{})
	}{
		{name: "foreign config", change: func(e map[string]interface{}) {
			e["detail"].(map[string]interface{})["mail"].(map[string]interface{})["tags"] = map[string]interface{}{"ses:configuration-set": []string{"other"}}
		}},
		{name: "missing config", change: func(e map[string]interface{}) {
			delete(e["detail"].(map[string]interface{})["mail"].(map[string]interface{}), "tags")
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			envelope := base()
			testCase.change(envelope)
			raw, _ := json.Marshal(envelope)
			_, relevant, err := decodeEvent(string(raw), testCloudConfig)
			if err != nil || relevant {
				t.Fatal("configuration envelope accepted")
			}
		})
	}
	for _, field := range []string{"source", "messageId", "timestamp", "destination"} {
		t.Run("invalid "+field, func(t *testing.T) {
			envelope := base()
			mail := envelope["detail"].(map[string]interface{})["mail"].(map[string]interface{})
			delete(mail, field)
			raw, _ := json.Marshal(envelope)
			if _, _, err := decodeEvent(string(raw), testCloudConfig); err == nil {
				t.Fatal("invalid metadata accepted")
			}
		})
	}
	for _, testCase := range []struct {
		kind, key, status string
		details           map[string]interface{}
	}{
		{kind: "Bounce", key: "bounce", status: "bounced", details: map[string]interface{}{"bouncedRecipients": []map[string]interface{}{{"emailAddress": "to@example.net", "diagnosticCode": "550 unknown"}}}},
		{kind: "Complaint", key: "complaint", status: "complaint", details: map[string]interface{}{"complainedRecipients": []map[string]interface{}{{"emailAddress": "to@example.net"}}, "complaintFeedbackType": "abuse"}},
		{kind: "DeliveryDelay", key: "deliveryDelay", status: "delayed", details: map[string]interface{}{"delayedRecipients": []map[string]interface{}{{"emailAddress": "to@example.net", "diagnosticCode": "421 retry"}}}},
	} {
		t.Run(testCase.kind, func(t *testing.T) {
			envelope := base()
			detail := envelope["detail"].(map[string]interface{})
			detail["eventType"] = testCase.kind
			detail[testCase.key] = testCase.details
			raw, _ := json.Marshal(envelope)
			if _, _, err := decodeEvent(string(raw), testCloudConfig); err == nil {
				t.Fatal("missing outcome time accepted")
			}
			testCase.details["timestamp"] = deliveryNow
			raw, _ = json.Marshal(envelope)
			event, relevant, err := decodeEvent(string(raw), testCloudConfig)
			if err != nil || !relevant || len(event.outcomes) != 1 || event.outcomes[0].status != testCase.status {
				t.Fatalf("outcome=%+v,%v", event, err)
			}
		})
	}
	for _, kind := range []string{"Reject", "Rendering Failure"} {
		envelope := base()
		envelope["detail"].(map[string]interface{})["eventType"] = kind
		raw, _ := json.Marshal(envelope)
		event, relevant, err := decodeEvent(string(raw), testCloudConfig)
		if err != nil || !relevant || len(event.outcomes) != 1 {
			t.Fatalf("rejection=%+v,%v", event, err)
		}
	}
	for _, field := range []string{"unknown kind", "bad destination", "missing event time", "missing delivery time"} {
		t.Run(field, func(t *testing.T) {
			envelope := base()
			detail := envelope["detail"].(map[string]interface{})
			mail := detail["mail"].(map[string]interface{})
			switch field {
			case "unknown kind":
				detail["eventType"] = "Unknown"
			case "bad destination":
				mail["destination"] = []string{"bad address"}
			case "missing event time":
				delete(envelope, "time")
			case "missing delivery time":
				detail["eventType"] = "Delivery"
			}
			raw, _ := json.Marshal(envelope)
			if _, _, err := decodeEvent(string(raw), testCloudConfig); err == nil {
				t.Fatal("malformed event accepted")
			}
		})
	}
	for _, truncated := range []bool{false, true} {
		envelope := base()
		mail := envelope["detail"].(map[string]interface{})["mail"].(map[string]interface{})
		mail["headersTruncated"] = truncated
		mail["headers"] = []map[string]string{{"name": "Message-ID", "value": "<parent@email.amazonses.com>"}, {"name": "X-Pingwin-Original-From", "value": "original@example.com"}}
		raw, _ := json.Marshal(envelope)
		event, _, err := decodeEvent(string(raw), testCloudConfig)
		if err != nil || (!truncated && event.parent != "parent") || (truncated && event.parent != "") {
			t.Fatalf("parent=%s,err=%v", event.parent, err)
		}
	}
	// A raw header alone is not evidence that Pingwin forwarded a message.
	envelope := base()
	envelope["detail"].(map[string]interface{})["mail"].(map[string]interface{})["headers"] = []map[string]string{{"name": "Message-ID", "value": "<parent@email.amazonses.com>"}}
	raw, _ := json.Marshal(envelope)
	event, _, _ := decodeEvent(string(raw), testCloudConfig)
	if event.parent != "" {
		t.Fatal("non-forward header linked")
	}
	// The SDK reader ignores unrelated envelopes rather than returning them.
	provider := cloudWatchHTTPClient(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(writer, `{"events":[{"message":"{\"source\":\"other\"}"}]}`)
	})
	reader, _ := NewCloudWatchReader(testCloudConfig, func(options *cloudwatchlogs.Options) { options.HTTPClient = provider })
	events, err := reader.Read(context.Background(), deliveryNow.Add(-Retention), deliveryNow)
	if err != nil || len(events) != 0 {
		t.Fatal("foreign provider event returned")
	}
}
