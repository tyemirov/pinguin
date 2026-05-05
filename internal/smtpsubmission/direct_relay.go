package smtpsubmission

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"sort"
	"strings"
	"time"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
)

const defaultDirectRelayOperationTimeout = 30 * time.Second

// DirectMXRelay delivers submitted messages directly to recipient-domain MX hosts.
type DirectMXRelay struct {
	logger           *slog.Logger
	hostname         string
	resolver         mxResolver
	dialer           smtpDialer
	operationTimeout time.Duration
}

type mxResolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
}

type smtpDialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
}

type netMXResolver struct{}

func (netMXResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	return net.DefaultResolver.LookupMX(ctx, name)
}

// NewDirectMXRelay constructs a direct recipient-MX relay.
func NewDirectMXRelay(logger *slog.Logger, cfg config.Config) *DirectMXRelay {
	connectionTimeout := time.Duration(cfg.ConnectionTimeoutSec) * time.Second
	if connectionTimeout <= 0 {
		connectionTimeout = defaultDirectRelayOperationTimeout
	}
	operationTimeout := time.Duration(cfg.OperationTimeoutSec) * time.Second
	if operationTimeout <= 0 {
		operationTimeout = defaultDirectRelayOperationTimeout
	}
	return &DirectMXRelay{
		logger:           logger,
		hostname:         cfg.SMTPSubmission.Hostname,
		resolver:         netMXResolver{},
		dialer:           &net.Dialer{Timeout: connectionTimeout},
		operationTimeout: operationTimeout,
	}
}

// Relay forwards a validated raw message to each recipient domain's MX hosts.
func (relay *DirectMXRelay) Relay(ctx context.Context, message RawMessage) error {
	recipientsByDomain := groupRecipientsByDomain(message.Recipients)
	domains := make([]string, 0, len(recipientsByDomain))
	for domain := range recipientsByDomain {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	for _, domain := range domains {
		if err := relay.relayDomain(ctx, domain, message.From, recipientsByDomain[domain], message.Data); err != nil {
			relay.logger.Error("smtp_submission_direct_delivery_failed", "identity_id", message.IdentityID, "domain", domain, "error", err)
			return err
		}
	}
	return nil
}

func (relay *DirectMXRelay) relayDomain(ctx context.Context, domain string, from smtpidentity.Address, recipients []smtpidentity.Address, data []byte) error {
	targets, resolveErr := relay.lookupTargets(ctx, domain)
	if resolveErr != nil {
		return fmt.Errorf("%w: resolve recipient mx %s: %v", ErrRelayTemporary, domain, resolveErr)
	}
	var lastErr error
	for _, target := range targets {
		if err := relay.relayTarget(ctx, target, from, recipients, data); err != nil {
			lastErr = err
			if isPermanentSMTPError(err) {
				return directRelayError(err)
			}
			continue
		}
		return nil
	}
	return directRelayError(lastErr)
}

func (relay *DirectMXRelay) lookupTargets(ctx context.Context, domain string) ([]string, error) {
	records, lookupErr := relay.resolver.LookupMX(ctx, domain)
	if lookupErr != nil {
		return []string{domain}, nil
	}
	if len(records) == 0 {
		return []string{domain}, nil
	}
	sort.SliceStable(records, func(i int, j int) bool {
		if records[i].Pref == records[j].Pref {
			return records[i].Host < records[j].Host
		}
		return records[i].Pref < records[j].Pref
	})
	targets := make([]string, 0, len(records))
	for _, record := range records {
		host := strings.TrimSpace(record.Host)
		if host == "" {
			continue
		}
		targets = append(targets, host)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("recipient mx records for %s contain no hosts", domain)
	}
	return targets, nil
}

func (relay *DirectMXRelay) relayTarget(ctx context.Context, target string, from smtpidentity.Address, recipients []smtpidentity.Address, data []byte) error {
	host := strings.TrimSuffix(target, ".")
	connection, dialErr := relay.dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, "25"))
	if dialErr != nil {
		return dialErr
	}
	defer connection.Close()
	if relay.operationTimeout > 0 {
		if deadlineErr := connection.SetDeadline(time.Now().Add(relay.operationTimeout)); deadlineErr != nil {
			return deadlineErr
		}
	}

	client, clientErr := smtp.NewClient(connection, host)
	if clientErr != nil {
		return clientErr
	}
	defer client.Close()
	if helloErr := client.Hello(relay.hostname); helloErr != nil {
		return helloErr
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: host,
		}
		if startTLSErr := client.StartTLS(tlsConfig); startTLSErr != nil {
			return startTLSErr
		}
	}
	if mailErr := client.Mail(from.String()); mailErr != nil {
		return mailErr
	}
	for _, recipient := range recipients {
		if rcptErr := client.Rcpt(recipient.String()); rcptErr != nil {
			return rcptErr
		}
	}
	writer, dataErr := client.Data()
	if dataErr != nil {
		return dataErr
	}
	_, writeErr := writer.Write(data)
	if finishErr := finishSMTPData(writer, writeErr); finishErr != nil {
		return finishErr
	}
	return client.Quit()
}

type smtpDataCloser interface {
	Close() error
}

func finishSMTPData(writer smtpDataCloser, writeErr error) error {
	if writeErr != nil {
		_ = writer.Close()
		return writeErr
	}
	return writer.Close()
}

func groupRecipientsByDomain(recipients []smtpidentity.Address) map[string][]smtpidentity.Address {
	grouped := make(map[string][]smtpidentity.Address)
	for _, recipient := range recipients {
		grouped[recipient.Domain()] = append(grouped[recipient.Domain()], recipient)
	}
	return grouped
}

func directRelayError(err error) error {
	if isPermanentSMTPError(err) {
		return fmt.Errorf("%w: direct smtp: %v", ErrRelayPermanent, err)
	}
	return fmt.Errorf("%w: direct smtp: %v", ErrRelayTemporary, err)
}
