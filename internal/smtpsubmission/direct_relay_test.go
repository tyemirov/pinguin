package smtpsubmission

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
)

func TestDirectMXRelayDeliversToRecipientMX(t *testing.T) {
	smtpServer := startFakeDirectSMTPServer(t, fakeSMTPBehavior{})
	resolver := fakeMXResolver{
		records: map[string][]*net.MX{
			"example.net": {{Host: "mx.example.net.", Pref: 10}},
		},
	}
	dialer := &fakeSMTPDialer{targetAddress: smtpServer.address}
	relay := &DirectMXRelay{
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		hostname:         "pinguin-api.mprlab.com",
		resolver:         resolver,
		dialer:           dialer,
		operationTimeout: time.Second,
	}
	message := RawMessage{
		IdentityID: "identity-one",
		From:       mustSMTPSubmissionAddress(t, "alice@example.com"),
		Recipients: []smtpidentity.Address{
			mustSMTPSubmissionAddress(t, "one@example.net"),
			mustSMTPSubmissionAddress(t, "two@example.net"),
		},
		Data: []byte("From: alice@example.com\r\nSubject: Test\r\n\r\nHello\r\n"),
	}

	if err := relay.Relay(context.Background(), message); err != nil {
		t.Fatalf("relay returned error: %v", err)
	}

	commands, data := smtpServer.result(t)
	if !stringListContains(commands, "EHLO pinguin-api.mprlab.com") {
		t.Fatalf("expected EHLO command, got %v", commands)
	}
	if !stringListContains(commands, "MAIL FROM:<alice@example.com>") {
		t.Fatalf("expected MAIL FROM command, got %v", commands)
	}
	if !stringListContains(commands, "RCPT TO:<one@example.net>") || !stringListContains(commands, "RCPT TO:<two@example.net>") {
		t.Fatalf("expected RCPT commands, got %v", commands)
	}
	if !strings.Contains(data, "Subject: Test") {
		t.Fatalf("expected raw message data, got %q", data)
	}
	if dialer.lastAddress != "mx.example.net:25" {
		t.Fatalf("expected MX target dial, got %q", dialer.lastAddress)
	}
}

func TestDirectMXRelayRejectsMultipleRecipientDomainsBeforeDelivery(t *testing.T) {
	dialer := &fakeSMTPDialer{err: errors.New("should not dial multiple recipient domains")}
	relay := &DirectMXRelay{
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		hostname:         "pinguin-api.mprlab.com",
		resolver:         fakeMXResolver{records: map[string][]*net.MX{}},
		dialer:           dialer,
		operationTimeout: time.Second,
	}

	err := relay.Relay(context.Background(), RawMessage{
		IdentityID: "identity-one",
		From:       mustSMTPSubmissionAddress(t, "alice@example.com"),
		Recipients: []smtpidentity.Address{
			mustSMTPSubmissionAddress(t, "one@example.net"),
			mustSMTPSubmissionAddress(t, "two@example.org"),
		},
		Data: []byte("From: alice@example.com\r\n\r\nHello\r\n"),
	})

	if !errors.Is(err, ErrRelayPermanent) {
		t.Fatalf("expected permanent relay error, got %v", err)
	}
	if dialer.lastAddress != "" {
		t.Fatalf("expected multi-domain message not to dial, got %q", dialer.lastAddress)
	}
}

func TestDirectMXRelayFallsBackToDomainWhenMXIsAbsent(t *testing.T) {
	smtpServer := startFakeDirectSMTPServer(t, fakeSMTPBehavior{})
	relay := &DirectMXRelay{
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		hostname:         "pinguin-api.mprlab.com",
		resolver:         fakeMXResolver{lookupErr: &net.DNSError{Err: "no such host", Name: "example.net", IsNotFound: true}},
		dialer:           &fakeSMTPDialer{targetAddress: smtpServer.address},
		operationTimeout: time.Second,
	}

	err := relay.Relay(context.Background(), RawMessage{
		IdentityID: "identity-one",
		From:       mustSMTPSubmissionAddress(t, "alice@example.com"),
		Recipients: []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
		Data:       []byte("From: alice@example.com\r\n\r\n" + strings.Repeat("Hello\r\n", 10000)),
	})
	if err != nil {
		t.Fatalf("relay returned error: %v", err)
	}
}

func TestDirectMXRelayBoundsMXLookupWithOperationTimeout(t *testing.T) {
	dialer := &fakeSMTPDialer{err: errors.New("should not dial after lookup timeout")}
	relay := &DirectMXRelay{
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		hostname:         "pinguin-api.mprlab.com",
		resolver:         blockingMXResolver{},
		dialer:           dialer,
		operationTimeout: 10 * time.Millisecond,
	}
	done := make(chan error, 1)
	go func() {
		done <- relay.Relay(context.Background(), RawMessage{
			IdentityID: "identity-one",
			From:       mustSMTPSubmissionAddress(t, "alice@example.com"),
			Recipients: []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
			Data:       []byte("From: alice@example.com\r\n\r\nHello\r\n"),
		})
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrRelayTemporary) {
			t.Fatalf("expected temporary relay error, got %v", err)
		}
		if dialer.lastAddress != "" {
			t.Fatalf("expected DNS timeout not to fall back to domain dial, got %q", dialer.lastAddress)
		}
	case <-time.After(time.Second):
		t.Fatalf("direct relay did not bound MX lookup with operation timeout")
	}
}

func TestDirectMXRelayMapsFailures(t *testing.T) {
	permanentServer := startFakeDirectSMTPServer(t, fakeSMTPBehavior{rcptResponse: "550 no such user"})
	temporaryServer := startFakeDirectSMTPServer(t, fakeSMTPBehavior{dataResponse: "451 try later"})
	testCases := []struct {
		name    string
		address string
		wantErr error
	}{
		{name: "permanent", address: permanentServer.address, wantErr: ErrRelayPermanent},
		{name: "temporary", address: temporaryServer.address, wantErr: ErrRelayTemporary},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			relay := &DirectMXRelay{
				logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
				hostname:         "pinguin-api.mprlab.com",
				resolver:         fakeMXResolver{records: map[string][]*net.MX{"example.net": {{Host: "mx.example.net.", Pref: 10}}}},
				dialer:           &fakeSMTPDialer{targetAddress: testCase.address},
				operationTimeout: time.Second,
			}
			err := relay.Relay(context.Background(), RawMessage{
				IdentityID: "identity-one",
				From:       mustSMTPSubmissionAddress(t, "alice@example.com"),
				Recipients: []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
				Data:       []byte("From: alice@example.com\r\n\r\nHello\r\n"),
			})
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("expected %v, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestDirectMXRelayTreatsAcceptedDataAsDeliveredWhenQuitFails(t *testing.T) {
	smtpServer := startFakeDirectSMTPServer(t, fakeSMTPBehavior{quitResponse: "421 closing"})
	relay := &DirectMXRelay{
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		hostname:         "pinguin-api.mprlab.com",
		resolver:         fakeMXResolver{records: map[string][]*net.MX{"example.net": {{Host: "mx.example.net.", Pref: 10}}}},
		dialer:           &fakeSMTPDialer{targetAddress: smtpServer.address},
		operationTimeout: time.Second,
	}

	err := relay.Relay(context.Background(), RawMessage{
		IdentityID: "identity-one",
		From:       mustSMTPSubmissionAddress(t, "alice@example.com"),
		Recipients: []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
		Data:       []byte("From: alice@example.com\r\n\r\nHello\r\n"),
	})

	if err != nil {
		t.Fatalf("expected accepted DATA to count as delivered despite QUIT error, got %v", err)
	}
	commands, data := smtpServer.result(t)
	if !stringListContains(commands, "QUIT") || !strings.Contains(data, "Hello") {
		t.Fatalf("expected DATA and best-effort QUIT, commands=%v data=%q", commands, data)
	}
}

func TestDirectMXRelayCoversTargetAndLookupErrorBranches(t *testing.T) {
	testCases := []struct {
		name    string
		relay   *DirectMXRelay
		wantErr error
	}{
		{
			name: "blank mx records",
			relay: &DirectMXRelay{
				logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
				hostname:         "pinguin-api.mprlab.com",
				resolver:         fakeMXResolver{records: map[string][]*net.MX{"example.net": {{Host: " "}}}},
				dialer:           &fakeSMTPDialer{err: errors.New("should not dial")},
				operationTimeout: time.Second,
			},
			wantErr: ErrRelayTemporary,
		},
		{
			name: "null mx records",
			relay: &DirectMXRelay{
				logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
				hostname:         "pinguin-api.mprlab.com",
				resolver:         fakeMXResolver{records: map[string][]*net.MX{"example.net": {{Host: ".", Pref: 0}}}},
				dialer:           &fakeSMTPDialer{err: errors.New("should not dial null mx")},
				operationTimeout: time.Second,
			},
			wantErr: ErrRelayPermanent,
		},
		{
			name: "dial error",
			relay: &DirectMXRelay{
				logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
				hostname:         "pinguin-api.mprlab.com",
				resolver:         fakeMXResolver{records: map[string][]*net.MX{"example.net": {{Host: "mx.example.net.", Pref: 10}}}},
				dialer:           &fakeSMTPDialer{err: errors.New("dial failed")},
				operationTimeout: time.Second,
			},
			wantErr: ErrRelayTemporary,
		},
		{
			name: "deadline error",
			relay: &DirectMXRelay{
				logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
				hostname: "pinguin-api.mprlab.com",
				resolver: fakeMXResolver{records: map[string][]*net.MX{"example.net": {{Host: "mx.example.net.", Pref: 10}}}},
				dialer: &fakeSMTPDialer{
					connection: newDeadlineFailingConn(),
				},
				operationTimeout: time.Second,
			},
			wantErr: ErrRelayTemporary,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.relay.Relay(context.Background(), RawMessage{
				IdentityID: "identity-one",
				From:       mustSMTPSubmissionAddress(t, "alice@example.com"),
				Recipients: []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
				Data:       []byte("From: alice@example.com\r\n\r\nHello\r\n"),
			})
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("expected %v, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestDirectMXRelayCoversSMTPCommandErrorBranches(t *testing.T) {
	testCases := []struct {
		name     string
		behavior fakeSMTPBehavior
		wantErr  error
	}{
		{name: "bad greeting", behavior: fakeSMTPBehavior{greeting: "500 unavailable"}, wantErr: ErrRelayPermanent},
		{name: "hello error", behavior: fakeSMTPBehavior{ehloResponse: "500 hello rejected"}, wantErr: ErrRelayTemporary},
		{name: "starttls error", behavior: fakeSMTPBehavior{advertiseStartTLS: true, startTLSResponse: "454 tls unavailable"}, wantErr: ErrRelayTemporary},
		{name: "mail error", behavior: fakeSMTPBehavior{mailResponse: "550 sender rejected"}, wantErr: ErrRelayPermanent},
		{name: "data command error", behavior: fakeSMTPBehavior{dataCommandResponse: "554 data rejected"}, wantErr: ErrRelayPermanent},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			smtpServer := startFakeDirectSMTPServer(t, testCase.behavior)
			relay := &DirectMXRelay{
				logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
				hostname:         "pinguin-api.mprlab.com",
				resolver:         fakeMXResolver{records: map[string][]*net.MX{"example.net": {{Host: "mx.example.net.", Pref: 10}}}},
				dialer:           &fakeSMTPDialer{targetAddress: smtpServer.address},
				operationTimeout: time.Second,
			}
			err := relay.Relay(context.Background(), RawMessage{
				IdentityID: "identity-one",
				From:       mustSMTPSubmissionAddress(t, "alice@example.com"),
				Recipients: []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
				Data:       []byte("From: alice@example.com\r\n\r\nHello\r\n"),
			})
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("expected %v, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestDirectMXRelayMapsDataWriteError(t *testing.T) {
	connection := newClosingAfterDataConn(t)
	relay := &DirectMXRelay{
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		hostname:         "pinguin-api.mprlab.com",
		resolver:         fakeMXResolver{records: map[string][]*net.MX{"example.net": {{Host: "mx.example.net.", Pref: 10}}}},
		dialer:           &fakeSMTPDialer{connection: connection},
		operationTimeout: time.Second,
	}
	err := relay.Relay(context.Background(), RawMessage{
		IdentityID: "identity-one",
		From:       mustSMTPSubmissionAddress(t, "alice@example.com"),
		Recipients: []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
		Data:       []byte("From: alice@example.com\r\n\r\nHello\r\n"),
	})
	if !errors.Is(err, ErrRelayTemporary) {
		t.Fatalf("expected temporary relay error, got %v", err)
	}
}

func TestDirectMXRelayLookupSortingAndDefaults(t *testing.T) {
	relay := &DirectMXRelay{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		hostname: "pinguin-api.mprlab.com",
		resolver: fakeMXResolver{records: map[string][]*net.MX{
			"example.net": {
				{Host: "mx-b.example.net.", Pref: 20},
				{Host: "mx-a.example.net.", Pref: 20},
				{Host: "mx-priority.example.net.", Pref: 10},
			},
			"empty.example": {},
		}},
	}
	targets, err := relay.lookupTargets(context.Background(), "example.net")
	if err != nil {
		t.Fatalf("lookup targets: %v", err)
	}
	expectedTargets := []string{"mx-priority.example.net.", "mx-a.example.net.", "mx-b.example.net."}
	for index, expectedTarget := range expectedTargets {
		if targets[index] != expectedTarget {
			t.Fatalf("expected target %d=%q, got %q", index, expectedTarget, targets[index])
		}
	}
	fallbackTargets, fallbackErr := relay.lookupTargets(context.Background(), "empty.example")
	if fallbackErr != nil || len(fallbackTargets) != 1 || fallbackTargets[0] != "empty.example" {
		t.Fatalf("expected empty MX fallback, targets=%v err=%v", fallbackTargets, fallbackErr)
	}
	if _, err := (netMXResolver{}).LookupMX(canceledContext(), "example.net"); err == nil {
		t.Fatalf("expected canceled resolver context error")
	}
}

func TestFinishSMTPDataPrefersWriteErrorAndCloses(t *testing.T) {
	expectedErr := errors.New("write failed")
	closer := &recordingDataCloser{}
	if err := finishSMTPData(closer, expectedErr); !errors.Is(err, expectedErr) {
		t.Fatalf("expected write error, got %v", err)
	}
	if !closer.closed {
		t.Fatalf("expected closer to run after write error")
	}

	closeErr := errors.New("close failed")
	closer = &recordingDataCloser{err: closeErr}
	if err := finishSMTPData(closer, nil); !errors.Is(err, closeErr) {
		t.Fatalf("expected close error, got %v", err)
	}
}

func TestDirectMXRelayBuildsProductionDependencies(t *testing.T) {
	relay := NewDirectMXRelay(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  7,
		SMTPSubmission: config.SMTPSubmissionConfig{
			Hostname: "pinguin-api.mprlab.com",
		},
	})
	if relay.logger == nil || relay.resolver == nil || relay.dialer == nil {
		t.Fatalf("expected production direct relay dependencies")
	}
	if relay.hostname != "pinguin-api.mprlab.com" || relay.operationTimeout != 7*time.Second {
		t.Fatalf("unexpected relay config: %+v", relay)
	}
	defaultRelay := NewDirectMXRelay(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{
		SMTPSubmission: config.SMTPSubmissionConfig{Hostname: "smtp.default.test"},
	})
	if defaultRelay.operationTimeout != defaultDirectRelayOperationTimeout {
		t.Fatalf("expected default operation timeout, got %s", defaultRelay.operationTimeout)
	}
}

type fakeMXResolver struct {
	records   map[string][]*net.MX
	lookupErr error
}

func (resolver fakeMXResolver) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	if resolver.lookupErr != nil {
		return nil, resolver.lookupErr
	}
	return resolver.records[name], nil
}

type blockingMXResolver struct{}

func (blockingMXResolver) LookupMX(ctx context.Context, _ string) ([]*net.MX, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type fakeSMTPDialer struct {
	targetAddress string
	lastAddress   string
	connection    net.Conn
	err           error
}

func (dialer *fakeSMTPDialer) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	dialer.lastAddress = address
	if dialer.err != nil {
		return nil, dialer.err
	}
	if dialer.connection != nil {
		return dialer.connection, nil
	}
	var netDialer net.Dialer
	return netDialer.DialContext(ctx, network, dialer.targetAddress)
}

type fakeSMTPBehavior struct {
	greeting            string
	ehloResponse        string
	advertiseStartTLS   bool
	startTLSResponse    string
	mailResponse        string
	rcptResponse        string
	dataCommandResponse string
	dataResponse        string
	quitResponse        string
	closeAfterDataReady bool
}

type recordingDataCloser struct {
	closed bool
	err    error
}

func (closer *recordingDataCloser) Close() error {
	closer.closed = true
	return closer.err
}

type fakeDirectSMTPServer struct {
	address  string
	done     chan struct{}
	err      error
	commands []string
	data     string
	mutex    sync.Mutex
}

func startFakeDirectSMTPServer(t *testing.T, behavior fakeSMTPBehavior) *fakeDirectSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake smtp: %v", err)
	}
	server := &fakeDirectSMTPServer{
		address: listener.Addr().String(),
		done:    make(chan struct{}),
	}
	go func() {
		defer close(server.done)
		defer listener.Close()
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			server.setErr(acceptErr)
			return
		}
		defer connection.Close()
		reader := bufio.NewReader(connection)
		writer := textproto.NewWriter(bufio.NewWriter(connection))
		greeting := behavior.greeting
		if greeting == "" {
			greeting = "220 mx.example.net ESMTP"
		}
		if writeErr := writer.PrintfLine("%s", greeting); writeErr != nil {
			server.setErr(writeErr)
			return
		}
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				server.setErr(readErr)
				return
			}
			command := strings.TrimRight(line, "\r\n")
			server.appendCommand(command)
			switch {
			case strings.HasPrefix(command, "EHLO "):
				if behavior.ehloResponse != "" {
					if writeErr := writer.PrintfLine("%s", behavior.ehloResponse); writeErr != nil {
						server.setErr(writeErr)
					}
					if strings.HasPrefix(behavior.ehloResponse, "5") {
						return
					}
					continue
				}
				if behavior.advertiseStartTLS {
					if writeErr := writer.PrintfLine("250-mx.example.net"); writeErr != nil {
						server.setErr(writeErr)
						return
					}
					if writeErr := writer.PrintfLine("250 STARTTLS"); writeErr != nil {
						server.setErr(writeErr)
						return
					}
					continue
				}
				if writeErr := writer.PrintfLine("250 mx.example.net"); writeErr != nil {
					server.setErr(writeErr)
					return
				}
			case command == "STARTTLS":
				response := behavior.startTLSResponse
				if response == "" {
					response = "454 tls unavailable"
				}
				if writeErr := writer.PrintfLine("%s", response); writeErr != nil {
					server.setErr(writeErr)
				}
				return
			case strings.HasPrefix(command, "MAIL FROM:"):
				response := behavior.mailResponse
				if response == "" {
					response = "250 OK"
				}
				if writeErr := writer.PrintfLine("%s", response); writeErr != nil {
					server.setErr(writeErr)
					return
				}
				if strings.HasPrefix(response, "5") {
					return
				}
			case strings.HasPrefix(command, "RCPT TO:"):
				response := behavior.rcptResponse
				if response == "" {
					response = "250 OK"
				}
				if writeErr := writer.PrintfLine("%s", response); writeErr != nil {
					server.setErr(writeErr)
					return
				}
				if strings.HasPrefix(response, "5") {
					return
				}
			case command == "DATA":
				response := behavior.dataCommandResponse
				if response == "" {
					response = "354 End data"
				}
				if writeErr := writer.PrintfLine("%s", response); writeErr != nil {
					server.setErr(writeErr)
					return
				}
				if strings.HasPrefix(response, "4") || strings.HasPrefix(response, "5") {
					return
				}
				if behavior.closeAfterDataReady {
					return
				}
				data, dataErr := readSMTPData(reader)
				if dataErr != nil {
					server.setErr(dataErr)
					return
				}
				server.setData(data)
				response = behavior.dataResponse
				if response == "" {
					response = "250 queued"
				}
				if writeErr := writer.PrintfLine("%s", response); writeErr != nil {
					server.setErr(writeErr)
					return
				}
				if strings.HasPrefix(response, "4") || strings.HasPrefix(response, "5") {
					return
				}
			case command == "QUIT":
				response := behavior.quitResponse
				if response == "" {
					response = "221 bye"
				}
				_ = writer.PrintfLine("%s", response)
				return
			default:
				if writeErr := writer.PrintfLine("502 unsupported"); writeErr != nil {
					server.setErr(writeErr)
					return
				}
			}
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-server.done
	})
	return server
}

func (server *fakeDirectSMTPServer) appendCommand(command string) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.commands = append(server.commands, command)
}

func (server *fakeDirectSMTPServer) setData(data string) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.data = data
}

func (server *fakeDirectSMTPServer) setErr(err error) {
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return
	}
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.err = err
}

func (server *fakeDirectSMTPServer) result(t *testing.T) ([]string, string) {
	t.Helper()
	select {
	case <-server.done:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for fake smtp server")
	}
	server.mutex.Lock()
	defer server.mutex.Unlock()
	if server.err != nil {
		t.Fatalf("fake smtp server failed: %v", server.err)
	}
	return append([]string(nil), server.commands...), server.data
}

func readSMTPData(reader *bufio.Reader) (string, error) {
	var builder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		if line == ".\r\n" || line == ".\n" {
			return builder.String(), nil
		}
		builder.WriteString(line)
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

type deadlineFailingConn struct {
	net.Conn
}

func newDeadlineFailingConn() net.Conn {
	clientConn, serverConn := net.Pipe()
	_ = serverConn.Close()
	return deadlineFailingConn{Conn: clientConn}
}

func (conn deadlineFailingConn) SetDeadline(time.Time) error {
	return errors.New("deadline failed")
}

func newClosingAfterDataConn(t *testing.T) net.Conn {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverConn.Close()
		reader := bufio.NewReader(serverConn)
		writer := textproto.NewWriter(bufio.NewWriter(serverConn))
		_ = writer.PrintfLine("220 mx.example.net ESMTP")
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			command := strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(command, "EHLO "):
				_ = writer.PrintfLine("250 mx.example.net")
			case strings.HasPrefix(command, "MAIL FROM:"):
				_ = writer.PrintfLine("250 OK")
			case strings.HasPrefix(command, "RCPT TO:"):
				_ = writer.PrintfLine("250 OK")
			case command == "DATA":
				_ = writer.PrintfLine("354 End data")
				return
			default:
				_ = writer.PrintfLine("502 unsupported")
			}
		}
	}()
	t.Cleanup(func() {
		_ = clientConn.Close()
		<-done
	})
	return clientConn
}

func stringListContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
