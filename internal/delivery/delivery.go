// Package delivery provides short-lived, verified-domain email delivery history.
package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tyemirov/pingwin/internal/smtpidentity"
)

const (
	statusUnconfirmed     = "unconfirmed"
	statusDelayed         = "delayed"
	statusDelivered       = "delivered"
	statusRenderingFailed = "rendering_failed"
	statusRejected        = "rejected"
	statusBounced         = "bounced"
	statusComplaint       = "complaint"
	// Retention is the maximum age of messages and events exposed by Pingwin.
	Retention    = 24 * time.Hour
	defaultLimit = 50
	maximumLimit = 100
)

var (
	// ErrInvalidQuery indicates a malformed search, limit, or cursor.
	ErrInvalidQuery = errors.New("delivery.query.invalid")
)

// Domain is a currently verified domain and the start of its tenant registration.
type Domain struct {
	Name         string
	RegisteredAt time.Time
}

// Scope identifies the authenticated tenant's verified domains.
type Scope struct {
	tenant  string
	domains map[string]time.Time
}

// NewScope validates the authorization boundary supplied by the tenant repository.
func NewScope(tenantID string, domains []Domain) (Scope, error) {
	if tenantID == "" {
		return Scope{}, errors.New("delivery scope: tenant is required")
	}
	scope := Scope{tenant: tenantID, domains: make(map[string]time.Time, len(domains))}
	for _, domain := range domains {
		name, err := smtpidentity.NormalizeSenderDomain(domain.Name)
		if err != nil || name == "" || domain.RegisteredAt.IsZero() {
			return Scope{}, errors.New("delivery scope: invalid verified domain registration")
		}
		scope.domains[name] = domain.RegisteredAt.UTC()
	}
	return scope, nil
}

// Query is a validated search and pagination request.
type Query struct {
	search string
	limit  int
	cursor string
}

// NewQuery validates external query parameters once at the HTTP boundary.
func NewQuery(search, limit, cursor string) (Query, error) {
	search = strings.TrimSpace(search)
	if utf8.RuneCountInString(search) > 200 || len(cursor) > 4096 {
		return Query{}, ErrInvalidQuery
	}
	pageLimit := defaultLimit
	if limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil || parsed < 1 || parsed > maximumLimit {
			return Query{}, ErrInvalidQuery
		}
		pageLimit = parsed
	}
	return Query{search: strings.ToLower(search), limit: pageLimit, cursor: cursor}, nil
}

// Recipient contains recipient-specific observed delivery evidence.
type Recipient struct {
	Address     string     `json:"address"`
	Status      string     `json:"status"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Response    string     `json:"response"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

// Message is a sanitized SES sending attempt, optionally linked to its forwarding parent.
type Message struct {
	ID                  string      `json:"id"`
	From                string      `json:"from"`
	Subject             string      `json:"subject"`
	SentAt              time.Time   `json:"sent_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
	ExpiresAt           time.Time   `json:"expires_at"`
	Recipients          []Recipient `json:"recipients"`
	ParentMessageID     string      `json:"parent_message_id,omitempty"`
	ForwardedMessageIDs []string    `json:"forwarded_message_ids"`
}

// Page contains a bounded collection within the current one-day window.
type Page struct {
	Messages    []Message `json:"messages"`
	NextCursor  string    `json:"next_cursor,omitempty"`
	WindowStart time.Time `json:"window_start"`
	AsOf        time.Time `json:"as_of"`
}

// Event is a validated provider event. Its metadata remains private to this package.
type Event struct {
	id, sender, subject, kind, parent string
	sentAt, occurredAt                time.Time
	destinations                      []string
	outcomes                          []outcome
}

type outcome struct {
	address, status, response string
	at                        time.Time
}

// Reader reads provider events without retaining a second history copy.
type Reader interface {
	Read(context.Context, time.Time, time.Time) ([]Event, error)
}

// Service authorizes, aggregates, and paginates the retained provider history.
type Service struct {
	reader Reader
	clock  func() time.Time
}

// NewService constructs a delivery history service with an explicit source and clock.
func NewService(reader Reader, clock func() time.Time) (*Service, error) {
	if reader == nil || clock == nil {
		return nil, errors.New("delivery service: reader and clock are required")
	}
	return &Service{reader: reader, clock: clock}, nil
}

type listCursor struct {
	Binding string    `json:"binding"`
	AsOf    time.Time `json:"as_of"`
	SentAt  time.Time `json:"sent_at"`
	ID      string    `json:"id"`
}

// List returns only currently authorized, unexpired message metadata.
func (service *Service) List(ctx context.Context, scope Scope, query Query) (Page, error) {
	now := service.clock().UTC()
	start := now.Add(-Retention)
	page := Page{Messages: []Message{}, WindowStart: start, AsOf: now}
	binding := cursorBinding(scope, query.search)
	var cursor listCursor
	if query.cursor != "" {
		encoded, err := base64.RawURLEncoding.DecodeString(query.cursor)
		if err != nil {
			return Page{}, ErrInvalidQuery
		}
		if err := json.Unmarshal(encoded, &cursor); err != nil || cursor.Binding != binding || cursor.ID == "" || cursor.AsOf.After(now) || !cursor.AsOf.After(start) || cursor.SentAt.IsZero() || cursor.SentAt.After(cursor.AsOf) {
			return Page{}, ErrInvalidQuery
		}
		page.AsOf = cursor.AsOf
	}
	if len(scope.domains) == 0 {
		return page, nil
	}
	events, err := service.reader.Read(ctx, start, now)
	if err != nil {
		return Page{}, fmt.Errorf("delivery history read: %w", err)
	}
	messages := aggregate(events, scope, start, now, page.AsOf)
	messageByID := make(map[string]*Message, len(messages))
	for _, message := range messages {
		messageByID[message.ID] = message
	}
	for _, message := range messages {
		if query.search != "" && !matchesSearch(message, messageByID, query.search) {
			continue
		}
		if query.cursor != "" && (message.SentAt.After(cursor.SentAt) || (message.SentAt.Equal(cursor.SentAt) && message.ID >= cursor.ID)) {
			continue
		}
		page.Messages = append(page.Messages, *message)
		if len(page.Messages) > query.limit {
			page.Messages = page.Messages[:query.limit]
			last := page.Messages[len(page.Messages)-1]
			encoded, _ := json.Marshal(listCursor{Binding: binding, AsOf: page.AsOf, SentAt: last.SentAt, ID: last.ID})
			page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
			break
		}
	}
	return page, nil
}

func cursorBinding(scope Scope, search string) string {
	names := make([]string, 0, len(scope.domains))
	for name, registered := range scope.domains {
		names = append(names, name+"="+registered.Format(time.RFC3339Nano))
	}
	sort.Strings(names)
	digest := sha256.Sum256([]byte(scope.tenant + "\x00" + search + "\x00" + strings.Join(names, "\x00")))
	return hex.EncodeToString(digest[:])
}

func aggregate(events []Event, scope Scope, start, now, asOf time.Time) []*Message {
	byID := make(map[string]*Message)
	parents := make(map[string]string)
	sort.Slice(events, func(left, right int) bool {
		if events[left].occurredAt.Equal(events[right].occurredAt) {
			return events[left].kind < events[right].kind
		}
		return events[left].occurredAt.Before(events[right].occurredAt)
	})
	for _, event := range events {
		_, domain, found := strings.Cut(event.sender, "@")
		registered, owned := scope.domains[domain]
		if !found || !owned || event.sentAt.Before(registered) || !event.sentAt.After(start) || event.sentAt.After(asOf) || !event.occurredAt.After(start) || event.occurredAt.After(now) {
			continue
		}
		message := byID[event.id]
		if message == nil {
			message = &Message{ID: event.id, From: event.sender, Subject: event.subject, SentAt: event.sentAt, UpdatedAt: event.occurredAt, ExpiresAt: event.sentAt.Add(Retention), Recipients: []Recipient{}, ForwardedMessageIDs: []string{}}
			byID[event.id] = message
		}
		if event.occurredAt.After(message.UpdatedAt) {
			message.UpdatedAt = event.occurredAt
		}
		for _, address := range event.destinations {
			if recipientIndex(message, address) == -1 {
				message.Recipients = append(message.Recipients, Recipient{Address: address, Status: statusUnconfirmed, UpdatedAt: event.occurredAt})
			}
		}
		for _, outcome := range event.outcomes {
			index := recipientIndex(message, outcome.address)
			if index == -1 {
				continue
			}
			recipient := &message.Recipients[index]
			if outcome.status == statusDelivered {
				if recipient.DeliveredAt == nil || outcome.at.Before(*recipient.DeliveredAt) {
					at := outcome.at
					recipient.DeliveredAt = &at
				}
			}
			if recipient.Status == statusUnconfirmed || outcome.at.After(recipient.UpdatedAt) || (outcome.at.Equal(recipient.UpdatedAt) && statusRank(outcome.status) > statusRank(recipient.Status)) {
				recipient.Status, recipient.Response, recipient.UpdatedAt = outcome.status, outcome.response, outcome.at
			}
		}
		if event.parent != "" {
			parents[event.id] = event.parent
		}
	}
	for childID, parentID := range parents {
		child, parent := byID[childID], byID[parentID]
		if parent == nil || child.SentAt.Before(parent.SentAt) || recipientIndex(parent, child.From) == -1 {
			continue
		}
		child.ParentMessageID = parentID
		parent.ForwardedMessageIDs = append(parent.ForwardedMessageIDs, childID)
	}
	result := make([]*Message, 0, len(byID))
	for _, message := range byID {
		sort.Strings(message.ForwardedMessageIDs)
		sort.Slice(message.Recipients, func(left, right int) bool {
			return message.Recipients[left].Address < message.Recipients[right].Address
		})
		result = append(result, message)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].SentAt.Equal(result[right].SentAt) {
			return result[left].ID > result[right].ID
		}
		return result[left].SentAt.After(result[right].SentAt)
	})
	return result
}

func recipientIndex(message *Message, address string) int {
	for index, recipient := range message.Recipients {
		if recipient.Address == address {
			return index
		}
	}
	return -1
}

func statusRank(status string) int {
	return map[string]int{statusUnconfirmed: 0, statusDelayed: 1, statusDelivered: 2, statusRenderingFailed: 3, statusRejected: 4, statusBounced: 5, statusComplaint: 6}[status]
}

func matchesSearch(message *Message, messages map[string]*Message, search string) bool {
	if messageMatches(message, search) {
		return true
	}
	if message.ParentMessageID != "" && messageMatches(messages[message.ParentMessageID], search) {
		return true
	}
	for _, childID := range message.ForwardedMessageIDs {
		if messageMatches(messages[childID], search) {
			return true
		}
	}
	return false
}

func messageMatches(message *Message, search string) bool {
	values := []string{message.ID, message.From, message.Subject}
	for _, recipient := range message.Recipients {
		values = append(values, recipient.Address)
	}
	return strings.Contains(strings.ToLower(strings.Join(values, "\n")), search)
}
