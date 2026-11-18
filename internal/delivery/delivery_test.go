package delivery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

var deliveryNow = time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
var testCloudConfig = CloudWatchConfig{Region: "us-east-1", AccountID: "123456789012", LogGroup: "/delivery", ConfigurationSet: "delivery", AccessKeyID: "test-key", SecretAccessKey: "test-secret"}

type eventReader struct {
	events []Event
	err    error
	calls  int
}

func (reader *eventReader) Read(context.Context, time.Time, time.Time) ([]Event, error) {
	reader.calls++
	return append([]Event(nil), reader.events...), reader.err
}

func TestDeliveryConstructors(t *testing.T) {
	if _, err := NewScope("", nil); err == nil {
		t.Fatal("empty tenant accepted")
	}
	for _, domain := range []Domain{{Name: "bad domain", RegisteredAt: deliveryNow}, {Name: "", RegisteredAt: deliveryNow}, {Name: "example.com"}} {
		if _, err := NewScope("tenant", []Domain{domain}); err == nil {
			t.Fatal("bad domain registration accepted")
		}
	}
	for _, arguments := range [][3]string{{strings.Repeat("a", 201), "", ""}, {"", "0", ""}, {"", "101", ""}, {"", "bad", ""}, {"", "", strings.Repeat("a", 4097)}} {
		if _, err := NewQuery(arguments[0], arguments[1], arguments[2]); err == nil {
			t.Fatal("bad query accepted")
		}
	}
	if _, err := NewService(nil, time.Now); err == nil {
		t.Fatal("nil reader accepted")
	}
	if _, err := NewService(&eventReader{}, nil); err == nil {
		t.Fatal("nil clock accepted")
	}
}

func TestDeliveryHistoryScopesRetentionOutcomesAndForwarding(t *testing.T) {
	parent := testEvent(t, "parent", "sender@example.com", "original", deliveryNow.Add(-time.Hour), "Send", []string{"relay@example.com", "other@destination.com"})
	child := testEvent(t, "child", "relay@example.com", "original", deliveryNow.Add(-30*time.Minute), "Delivery", []string{"inbox@gmail.com"})
	child.parent = "parent"
	parentDelivery := parent
	parentDelivery.kind = "Delivery"
	parentDelivery.occurredAt = parent.sentAt.Add(time.Minute)
	parentDelivery.outcomes = []outcome{{address: "relay@example.com", status: "delivered", response: "250 accepted", at: parentDelivery.occurredAt}}
	complaint := parentDelivery
	complaint.kind = "Complaint"
	complaint.occurredAt = parent.sentAt.Add(2 * time.Minute)
	complaint.outcomes = []outcome{{address: "relay@example.com", status: "complaint", response: "abuse", at: complaint.occurredAt}}
	foreign := testEvent(t, "foreign", "sender@elsewhere.com", "secret", deliveryNow.Add(-time.Hour), "Delivery", []string{"inbox@gmail.com"})
	subdomain := foreign
	subdomain.id = "subdomain"
	subdomain.sender = "sender@sub.example.com"
	beforeRegistration := parent
	beforeRegistration.id = "before-registration"
	beforeRegistration.sentAt = deliveryNow.Add(-3 * time.Hour)
	expired := parent
	expired.id = "expired"
	expired.sentAt = deliveryNow.Add(-Retention)
	future := parent
	future.id = "future"
	future.sentAt = deliveryNow.Add(time.Minute)
	oldEvent := parent
	oldEvent.id = "old-event"
	oldEvent.occurredAt = deliveryNow.Add(-Retention)
	futureEvent := parent
	futureEvent.id = "future-event"
	futureEvent.occurredAt = deliveryNow.Add(time.Minute)
	unknownOutcome := parent
	unknownOutcome.outcomes = []outcome{{address: "not-a-recipient@example.com", status: "delivered", at: deliveryNow}}
	forgedChild := child
	forgedChild.id = "forged"
	forgedChild.sender = "not-a-route@example.com"
	tooEarlyChild := child
	tooEarlyChild.id = "early"
	tooEarlyChild.sentAt = parent.sentAt.Add(-time.Minute)
	missingParent := child
	missingParent.id = "missing-parent"
	missingParent.parent = "foreign"
	reader := &eventReader{events: []Event{complaint, child, parentDelivery, parent, parentDelivery, foreign, subdomain, beforeRegistration, expired, future, oldEvent, futureEvent, unknownOutcome, forgedChild, tooEarlyChild, missingParent}}
	service, _ := NewService(reader, func() time.Time { return deliveryNow })
	scope, _ := NewScope("tenant", []Domain{{Name: "EXAMPLE.com", RegisteredAt: deliveryNow.Add(-2 * time.Hour)}})
	query, _ := NewQuery("", "100", "")
	page, err := service.List(context.Background(), scope, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 5 {
		t.Fatalf("messages=%+v", page.Messages)
	}
	byID := map[string]Message{}
	for _, message := range page.Messages {
		byID[message.ID] = message
		if !message.ExpiresAt.Equal(message.SentAt.Add(Retention)) {
			t.Fatal("wrong expiry")
		}
	}
	if byID["child"].ParentMessageID != "parent" || len(byID["parent"].ForwardedMessageIDs) != 1 {
		t.Fatalf("chain=%+v", page.Messages)
	}
	if byID["forged"].ParentMessageID != "" || byID["early"].ParentMessageID != "" || byID["missing-parent"].ParentMessageID != "" {
		t.Fatal("forged chain linked")
	}
	for _, recipient := range byID["parent"].Recipients {
		if recipient.Address == "relay@example.com" && (recipient.Status != "complaint" || recipient.DeliveredAt == nil) {
			t.Fatalf("delivery fact lost: %+v", recipient)
		}
		if recipient.Address == "other@destination.com" && recipient.Status != "unconfirmed" {
			t.Fatal("partial delivery marked all accepted")
		}
	}
	for _, search := range []string{"inbox@gmail.com", "parent", "original"} {
		query, _ = NewQuery(search, "100", "")
		searched, err := service.List(context.Background(), scope, query)
		if err != nil || len(searched.Messages) < 2 {
			t.Fatalf("search %s=%+v,%v", search, searched, err)
		}
	}
	query, _ = NewQuery("secret", "100", "")
	page, err = service.List(context.Background(), scope, query)
	if err != nil || len(page.Messages) != 0 {
		t.Fatal("foreign search leaked")
	}
	emptyScope, _ := NewScope("empty", nil)
	query, _ = NewQuery("", "", "")
	before := reader.calls
	page, err = service.List(context.Background(), emptyScope, query)
	if err != nil || len(page.Messages) != 0 || reader.calls != before {
		t.Fatal("empty domain scope contacted provider")
	}
	reader.err = errors.New("provider down")
	if _, err = service.List(context.Background(), scope, query); err == nil {
		t.Fatal("provider failure swallowed")
	}
}

func TestDeliveryCursorScopesAndExpires(t *testing.T) {
	first := testEvent(t, "b", "sender@example.com", "same", deliveryNow.Add(-time.Hour), "Send", []string{"to@example.net"})
	second := first
	second.id = "a"
	older := first
	older.id = "older"
	older.sentAt = deliveryNow.Add(-2 * time.Hour)
	reader := &eventReader{events: []Event{older, first, second}}
	current := deliveryNow
	service, _ := NewService(reader, func() time.Time { return current })
	scope, _ := NewScope("tenant", []Domain{{Name: "example.com", RegisteredAt: deliveryNow.Add(-3 * time.Hour)}})
	query, _ := NewQuery("", "1", "")
	page, err := service.List(context.Background(), scope, query)
	if err != nil || page.Messages[0].ID != "b" || page.NextCursor == "" {
		t.Fatalf("first=%+v,%v", page, err)
	}
	query, _ = NewQuery("", "1", page.NextCursor)
	page, err = service.List(context.Background(), scope, query)
	if err != nil || page.Messages[0].ID != "a" {
		t.Fatalf("second=%+v,%v", page, err)
	}
	query, _ = NewQuery("", "1", page.NextCursor)
	page, err = service.List(context.Background(), scope, query)
	if err != nil || page.Messages[0].ID != "older" || page.NextCursor != "" {
		t.Fatalf("last=%+v,%v", page, err)
	}
	query, _ = NewQuery("", "1", "")
	initial, _ := service.List(context.Background(), scope, query)
	for _, encoded := range []string{"!", base64.RawURLEncoding.EncodeToString([]byte("broken")), base64.RawURLEncoding.EncodeToString([]byte(`{}`))} {
		query, _ = NewQuery("", "1", encoded)
		if _, err := service.List(context.Background(), scope, query); !errors.Is(err, ErrInvalidQuery) {
			t.Fatal("invalid cursor accepted")
		}
	}
	query, _ = NewQuery("different", "1", initial.NextCursor)
	if _, err := service.List(context.Background(), scope, query); !errors.Is(err, ErrInvalidQuery) {
		t.Fatal("cursor search rebound")
	}
	otherScope, _ := NewScope("other-tenant", []Domain{{Name: "example.com", RegisteredAt: deliveryNow.Add(-3 * time.Hour)}})
	query, _ = NewQuery("", "1", initial.NextCursor)
	if _, err := service.List(context.Background(), otherScope, query); !errors.Is(err, ErrInvalidQuery) {
		t.Fatal("cursor tenant rebound")
	}
	current = deliveryNow.Add(Retention)
	if _, err := service.List(context.Background(), scope, query); !errors.Is(err, ErrInvalidQuery) {
		t.Fatal("expired cursor accepted")
	}
	current = deliveryNow.Add(-time.Minute)
	if _, err := service.List(context.Background(), scope, query); !errors.Is(err, ErrInvalidQuery) {
		t.Fatal("future cursor accepted")
	}
}

func TestDeliveryBounceWithoutSendSurvivesDelayedEnvelope(t *testing.T) {
	envelope := testEnvelope("bounce-only", "sender@example.com", "bounce", deliveryNow.Add(-time.Hour), "Bounce", []string{"recipient@example.net"})
	envelope["time"] = deliveryNow.Add(-30 * time.Minute)
	envelope["detail"].(map[string]interface{})["bounce"] = map[string]interface{}{"timestamp": deliveryNow.Add(-59 * time.Minute), "bouncedRecipients": []map[string]string{{"emailAddress": "recipient@example.net", "diagnosticCode": "550 unknown recipient"}}}
	raw, _ := json.Marshal(envelope)
	event, _, err := decodeEvent(string(raw), testCloudConfig)
	if err != nil {
		t.Fatal(err)
	}
	service, _ := NewService(&eventReader{events: []Event{event}}, func() time.Time { return deliveryNow })
	scope, _ := NewScope("tenant", []Domain{{Name: "example.com", RegisteredAt: deliveryNow.Add(-2 * time.Hour)}})
	query, _ := NewQuery("", "", "")
	page, err := service.List(context.Background(), scope, query)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].Recipients[0].Status != "bounced" {
		t.Fatalf("bounce evidence lost: %+v,%v", page, err)
	}
}

func testEnvelope(id, sender, subject string, sent time.Time, kind string, destinations []string) map[string]interface{} {
	detail := map[string]interface{}{"eventType": kind, "mail": map[string]interface{}{"timestamp": sent, "source": sender, "messageId": id, "destination": destinations, "commonHeaders": map[string]interface{}{"subject": subject}, "tags": map[string]interface{}{"ses:configuration-set": []string{testCloudConfig.ConfigurationSet}}}}
	if kind == "Delivery" {
		detail["delivery"] = map[string]interface{}{"timestamp": sent.Add(time.Second), "recipients": destinations, "smtpResponse": "250 accepted"}
	}
	return map[string]interface{}{"source": "aws.ses", "account": testCloudConfig.AccountID, "region": testCloudConfig.Region, "time": sent, "detail": detail}
}

func testEvent(t *testing.T, id, sender, subject string, sent time.Time, kind string, destinations []string) Event {
	t.Helper()
	encoded, _ := json.Marshal(testEnvelope(id, sender, subject, sent, kind, destinations))
	event, relevant, err := decodeEvent(string(encoded), testCloudConfig)
	if err != nil || !relevant {
		t.Fatalf("fixture=%v", err)
	}
	return event
}
