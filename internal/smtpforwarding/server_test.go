package smtpforwarding

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/smtpidentity"
)

func TestServerAcceptsAndForwardsInboundMessage(testHandle *testing.T) {
	fixture := newForwardingServerFixture(testHandle, nil)
	client := fixture.dial(testHandle)
	defer client.close()
	client.expectCode(testHandle, "220")
	client.send(testHandle, "EHLO sender.example")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "NOOP")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "AUTH PLAIN ignored")
	client.expectCode(testHandle, "502")
	client.send(testHandle, "STARTTLS")
	client.expectCode(testHandle, "502")
	client.send(testHandle, "MAIL FROM:<customer@example.net>")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "RCPT TO:<support@help.example.com>")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "DATA")
	client.expectCode(testHandle, "354")
	client.sendData(testHandle, "From: Customer <customer@example.net>\r\nTo: support@help.example.com\r\nSubject: Help\r\n\r\n..dot")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "QUIT")
	client.expectCode(testHandle, "221")

	if len(fixture.forwarder.messages) != 1 {
		testHandle.Fatalf("expected one forwarded message, got %d", len(fixture.forwarder.messages))
	}
	forwarded := fixture.forwarder.messages[0]
	if forwarded.route.Address().String() != "support@help.example.com" {
		testHandle.Fatalf("unexpected route %s", forwarded.route.Address().String())
	}
	if forwarded.message.From.String() != "customer@example.net" {
		testHandle.Fatalf("unexpected sender %s", forwarded.message.From.String())
	}
	if len(forwarded.message.Recipients) != 1 || forwarded.message.Recipients[0].String() != "support@help.example.com" {
		testHandle.Fatalf("unexpected recipients %+v", forwarded.message.Recipients)
	}
	if !strings.Contains(string(forwarded.message.Data), "\r\n.dot") {
		testHandle.Fatalf("expected dot-stuffed message to be unescaped: %q", string(forwarded.message.Data))
	}
}

func TestServerAcceptsNullReversePath(testHandle *testing.T) {
	fixture := newForwardingServerFixture(testHandle, nil)
	client := fixture.dial(testHandle)
	defer client.close()
	client.expectCode(testHandle, "220")
	client.send(testHandle, "MAIL FROM:<>")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "RCPT TO:<support@help.example.com>")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "DATA")
	client.expectCode(testHandle, "354")
	client.sendData(testHandle, "From: postmaster@example.net\r\nTo: support@help.example.com\r\nSubject: Delivery status\r\n\r\nDSN")
	client.expectCode(testHandle, "250")

	if len(fixture.forwarder.messages) != 1 {
		testHandle.Fatalf("expected one forwarded message, got %d", len(fixture.forwarder.messages))
	}
	if !fixture.forwarder.messages[0].message.From.IsNull() {
		testHandle.Fatalf("expected null reverse path, got %q", fixture.forwarder.messages[0].message.From.String())
	}
}

func TestServerCommandOrderingAndRecipientValidation(testHandle *testing.T) {
	fixture := newForwardingServerFixture(testHandle, nil)
	client := fixture.dial(testHandle)
	defer client.close()
	client.expectCode(testHandle, "220")
	client.send(testHandle, "DATA")
	client.expectCode(testHandle, "503")
	client.send(testHandle, "RCPT TO:<support@help.example.com>")
	client.expectCode(testHandle, "503")
	client.send(testHandle, "MAIL BODY:<customer@example.net>")
	client.expectCode(testHandle, "501")
	client.send(testHandle, "MAIL FROM:<customer@example.net")
	client.expectCode(testHandle, "501")
	client.send(testHandle, "MAIL FROM:customer@example.net extra")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "RCPT BODY:<support@help.example.com>")
	client.expectCode(testHandle, "501")
	client.send(testHandle, "RCPT TO:<missing@help.example.com>")
	client.expectCode(testHandle, "550")
	client.send(testHandle, "RCPT TO:<support@help.example.com>")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "RCPT TO:<sales@help.example.com>")
	client.expectCode(testHandle, "452")
	client.send(testHandle, "RSET")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "DATA")
	client.expectCode(testHandle, "503")
	client.send(testHandle, "VRFY support")
	client.expectCode(testHandle, "502")
	client.send(testHandle, "QUIT")
	client.expectCode(testHandle, "221")
}

func TestServerTreatsRouteLookupFailuresAsTemporary(testHandle *testing.T) {
	fixture := newForwardingServerFixture(testHandle, func(configValues Config) Config {
		configValues.RouteResolver = failingRouteResolver{}
		return configValues
	})
	client := fixture.dial(testHandle)
	defer client.close()
	client.expectCode(testHandle, "220")
	client.send(testHandle, "MAIL FROM:<customer@example.net>")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "RCPT TO:<support@help.example.com>")
	client.expectCode(testHandle, "451")
}

func TestServerRejectsOversizedMessageAndPropagatesForwardFailure(testHandle *testing.T) {
	fixture := newForwardingServerFixture(testHandle, func(configValues Config) Config {
		configValues.MaxMessageBytes = 12
		return configValues
	})
	client := fixture.dial(testHandle)
	defer client.close()
	client.expectCode(testHandle, "220")
	client.send(testHandle, "MAIL FROM:<customer@example.net>")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "RCPT TO:<support@help.example.com>")
	client.expectCode(testHandle, "250")
	client.send(testHandle, "DATA")
	client.expectCode(testHandle, "354")
	client.sendData(testHandle, "From: customer@example.net\r\n\r\nThis message is too large")
	client.expectCode(testHandle, "552")
	if len(fixture.forwarder.messages) != 0 {
		testHandle.Fatalf("oversized message should not forward")
	}

	failingFixture := newForwardingServerFixture(testHandle, nil)
	failingFixture.forwarder.err = errors.New("relay failed")
	failingClient := failingFixture.dial(testHandle)
	defer failingClient.close()
	failingClient.expectCode(testHandle, "220")
	failingClient.send(testHandle, "MAIL FROM:<customer@example.net>")
	failingClient.expectCode(testHandle, "250")
	failingClient.send(testHandle, "RCPT TO:<support@help.example.com>")
	failingClient.expectCode(testHandle, "250")
	failingClient.send(testHandle, "DATA")
	failingClient.expectCode(testHandle, "354")
	failingClient.sendData(testHandle, "From: customer@example.net\r\n\r\nHello")
	failingClient.expectCode(testHandle, "451")
}

func TestServerValidationStartAndLimiter(testHandle *testing.T) {
	routeSet := mustRouteSet(testHandle)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	for _, testCase := range []struct {
		name   string
		config Config
	}{
		{name: "missing hostname", config: Config{ListenAddr: "127.0.0.1:0", RouteResolver: routeSet, Forwarder: &recordingForwarder{}, Logger: logger}},
		{name: "missing listen", config: Config{Hostname: "mx.test", RouteResolver: routeSet, Forwarder: &recordingForwarder{}, Logger: logger}},
		{name: "missing resolver", config: Config{Hostname: "mx.test", ListenAddr: "127.0.0.1:0", Forwarder: &recordingForwarder{}, Logger: logger}},
		{name: "missing forwarder", config: Config{Hostname: "mx.test", ListenAddr: "127.0.0.1:0", RouteResolver: routeSet, Logger: logger}},
		{name: "missing logger", config: Config{Hostname: "mx.test", ListenAddr: "127.0.0.1:0", RouteResolver: routeSet, Forwarder: &recordingForwarder{}}},
	} {
		testCase := testCase
		testHandle.Run(testCase.name, func(t *testing.T) {
			if _, err := NewServer(testCase.config); err == nil {
				t.Fatalf("expected server validation error")
			}
		})
	}

	server, serverErr := NewServer(Config{
		Hostname:      "mx.test",
		ListenAddr:    "127.0.0.1:0",
		RouteResolver: routeSet,
		Forwarder:     &recordingForwarder{},
		Logger:        logger,
	})
	if serverErr != nil {
		testHandle.Fatalf("new server: %v", serverErr)
	}
	if server.config.MaxMessageBytes != defaultMaxMessageBytes || server.config.MaxRecipients != defaultMaxRecipients || server.config.CommandTimeout != defaultCommandTimeout {
		testHandle.Fatalf("expected server defaults, got %+v", server.config)
	}
	invalidListenServer, invalidListenErr := NewServer(Config{
		Hostname:      "mx.test",
		ListenAddr:    "bad address",
		RouteResolver: routeSet,
		Forwarder:     &recordingForwarder{},
		Logger:        logger,
	})
	if invalidListenErr != nil {
		testHandle.Fatalf("new invalid listen server: %v", invalidListenErr)
	}
	if err := invalidListenServer.Start(context.Background()); err == nil {
		testHandle.Fatalf("expected invalid listen error")
	}
	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()
	if err := server.Start(cancelledContext); err != nil {
		testHandle.Fatalf("expected canceled start to return nil, got %v", err)
	}

	listener := &errorListener{err: errors.New("accept failed")}
	if err := server.Serve(context.Background(), listener); err == nil {
		testHandle.Fatalf("expected accept error")
	}

	limitedServer, limitedErr := NewServer(Config{
		Hostname:                 "mx.test",
		ListenAddr:               "127.0.0.1:0",
		MaxConcurrentSessions:    1,
		MaxSessionsPerRemoteHost: 1,
		RouteResolver:            routeSet,
		Forwarder:                &recordingForwarder{},
		Logger:                   logger,
	})
	if limitedErr != nil {
		testHandle.Fatalf("limited server: %v", limitedErr)
	}
	release, acquireErr := limitedServer.limiter.acquire("127.0.0.1")
	if acquireErr != nil {
		testHandle.Fatalf("acquire limiter: %v", acquireErr)
	}
	defer release()
	testListener, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		testHandle.Fatalf("listen: %v", listenErr)
	}
	serveContext, serveCancel := context.WithCancel(context.Background())
	defer serveCancel()
	go func() {
		_ = limitedServer.Serve(serveContext, testListener)
	}()
	throttledClient := newForwardingTestClient(mustDial(testHandle, testListener.Addr().String()))
	defer throttledClient.close()
	throttledClient.expectCode(testHandle, "421")
	serveCancel()
	_ = testListener.Close()
}

func TestHelpersAndReadDataErrors(testHandle *testing.T) {
	server := newForwardingServerFixture(testHandle, nil).server
	server.handleConnection(context.Background(), &fakeConnection{remoteAddress: fakeAddress("127.0.0.1:1234")})
	server.handleConnection(context.Background(), &deadlineErrorConnection{remoteAddress: fakeAddress("127.0.0.1:1234")})

	session := newSessionState()
	mailFrom := mustReversePath(testHandle, "customer@example.net")
	session.mailFrom = &mailFrom
	session.recipients = []smtpidentity.Address{mustAddress(testHandle, "support@help.example.com")}
	session.routesByAddress["support@help.example.com"] = mustRoute(testHandle)
	server.handleData(
		context.Background(),
		&fakeConnection{remoteAddress: fakeAddress("127.0.0.1:1234")},
		bufio.NewReader(strings.NewReader("From: customer@example.net\r\n\r\npartial")),
		bufio.NewWriter(io.Discard),
		session,
	)
	session.mailFrom = &mailFrom
	session.recipients = []smtpidentity.Address{mustAddress(testHandle, "support@help.example.com")}
	session.routesByAddress["support@help.example.com"] = mustRoute(testHandle)
	server.handleData(
		context.Background(),
		&deadlineErrorConnection{remoteAddress: fakeAddress("127.0.0.1:1234")},
		bufio.NewReader(strings.NewReader("")),
		bufio.NewWriter(io.Discard),
		session,
	)
	session.mailFrom = &mailFrom
	session.recipients = []smtpidentity.Address{mustAddress(testHandle, "support@help.example.com")}
	session.routesByAddress["support@help.example.com"] = mustRoute(testHandle)
	server.handleData(
		context.Background(),
		&fakeConnection{remoteAddress: fakeAddress("127.0.0.1:1234")},
		bufio.NewReader(strings.NewReader("")),
		bufio.NewWriterSize(errorWriter{}, 1),
		session,
	)

	if command, argument := splitCommand("NOOP\r\n"); command != "NOOP" || argument != "" {
		testHandle.Fatalf("unexpected command split")
	}
	if isSMTPDataTerminator([]byte(".\n")) != true || isSMTPDataTerminator([]byte("..\n")) != false {
		testHandle.Fatalf("unexpected terminator result")
	}
	if _, err := parseSMTPPath("missing@example.com", "FROM:"); err == nil {
		testHandle.Fatalf("expected missing prefix error")
	}
	if _, err := parseSMTPPath("FROM:<customer@example.net>", "FROM:"); err != nil {
		testHandle.Fatalf("expected bracket path to parse: %v", err)
	}
	reversePath, reversePathErr := parseSMTPReversePath("FROM:<>", "FROM:")
	if reversePathErr != nil {
		testHandle.Fatalf("expected null reverse path to parse: %v", reversePathErr)
	}
	if !reversePath.IsNull() {
		testHandle.Fatalf("expected null reverse path, got %q", reversePath.String())
	}
	if _, err := parseSMTPPath("FROM:", "FROM:"); err == nil {
		testHandle.Fatalf("expected empty path to fail")
	}
	if _, err := parseSMTPReversePath("FROM:", "FROM:"); err == nil {
		testHandle.Fatalf("expected bare empty reverse path to fail")
	}
	if err := writeSMTPLine(bufio.NewWriterSize(errorWriter{}, 1), strings.Repeat("x", 5000)); err == nil {
		testHandle.Fatalf("expected write error")
	}
	if err := setSMTPReadDeadline(&fakeConnection{remoteAddress: fakeAddress("bad address")}, 0); err != nil {
		testHandle.Fatalf("zero deadline should be ignored")
	}
	if remoteHostForConnection(&fakeConnection{remoteAddress: fakeAddress("bad address")}) != "bad address" {
		testHandle.Fatalf("expected unsplittable remote address")
	}
	if _, _, err := server.readData(bufio.NewReader(strings.NewReader("partial"))); !errors.Is(err, io.EOF) {
		testHandle.Fatalf("expected EOF, got %v", err)
	}
	if _, _, err := server.readData(bufio.NewReader(errorReader{})); err == nil || errors.Is(err, io.EOF) {
		testHandle.Fatalf("expected non-EOF reader error, got %v", err)
	}
	longLine := strings.Repeat("a", 5000) + "\r\n.\r\n"
	if data, tooLarge, err := server.readData(bufio.NewReader(strings.NewReader(longLine))); err != nil || tooLarge || len(data) != 5002 {
		testHandle.Fatalf("expected long line data, len=%d tooLarge=%v err=%v", len(data), tooLarge, err)
	}

	limiter := newSessionLimiter(2, 1)
	firstRelease, firstErr := limiter.acquire("")
	if firstErr != nil {
		testHandle.Fatalf("first acquire: %v", firstErr)
	}
	if _, secondErr := limiter.acquire(""); secondErr == nil {
		testHandle.Fatalf("expected remote limit error")
	}
	firstRelease()
	firstRelease()
	secondRelease, secondErr := limiter.acquire("remote-a")
	if secondErr != nil {
		testHandle.Fatalf("second acquire: %v", secondErr)
	}
	thirdRelease, thirdErr := limiter.acquire("remote-b")
	if thirdErr != nil {
		testHandle.Fatalf("third acquire: %v", thirdErr)
	}
	if _, fourthErr := limiter.acquire("remote-c"); fourthErr == nil {
		testHandle.Fatalf("expected global limit error")
	}
	secondRelease()
	thirdRelease()
	limiter.release("missing")

	sameRemoteLimiter := newSessionLimiter(3, 2)
	sameRemoteFirstRelease, sameRemoteFirstErr := sameRemoteLimiter.acquire("remote")
	if sameRemoteFirstErr != nil {
		testHandle.Fatalf("same remote first acquire: %v", sameRemoteFirstErr)
	}
	sameRemoteSecondRelease, sameRemoteSecondErr := sameRemoteLimiter.acquire("remote")
	if sameRemoteSecondErr != nil {
		testHandle.Fatalf("same remote second acquire: %v", sameRemoteSecondErr)
	}
	sameRemoteFirstRelease()
	sameRemoteSecondRelease()
}

type forwardingServerFixture struct {
	server    *Server
	listener  net.Listener
	forwarder *recordingForwarder
	cancel    context.CancelFunc
}

func newForwardingServerFixture(testHandle *testing.T, mutate func(Config) Config) *forwardingServerFixture {
	testHandle.Helper()
	routeSet := mustRouteSet(testHandle)
	forwarder := &recordingForwarder{}
	configValues := Config{
		Hostname:        "mx-forward.test",
		ListenAddr:      "127.0.0.1:0",
		MaxMessageBytes: 1024 * 1024,
		MaxRecipients:   1,
		CommandTimeout:  time.Second,
		RouteResolver:   routeSet,
		Forwarder:       forwarder,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}
	if mutate != nil {
		configValues = mutate(configValues)
	}
	server, serverErr := NewServer(configValues)
	if serverErr != nil {
		testHandle.Fatalf("new server: %v", serverErr)
	}
	listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		testHandle.Fatalf("listen: %v", listenErr)
	}
	contextValue, cancel := context.WithCancel(context.Background())
	fixture := &forwardingServerFixture{server: server, listener: listener, forwarder: forwarder, cancel: cancel}
	go func() {
		_ = server.Serve(contextValue, listener)
	}()
	testHandle.Cleanup(func() {
		cancel()
		_ = listener.Close()
	})
	return fixture
}

func (fixture *forwardingServerFixture) dial(testHandle *testing.T) *forwardingTestClient {
	testHandle.Helper()
	return newForwardingTestClient(mustDial(testHandle, fixture.listener.Addr().String()))
}

type recordingForwarder struct {
	messages []forwardedMessage
	err      error
}

type failingRouteResolver struct{}

func (failingRouteResolver) Resolve(context.Context, smtpidentity.Address) (Route, bool, error) {
	return Route{}, false, errors.New("route lookup failed")
}

type forwardedMessage struct {
	route   Route
	message Message
}

func (forwarder *recordingForwarder) Forward(_ context.Context, route Route, message Message) error {
	if forwarder.err != nil {
		return forwarder.err
	}
	forwarder.messages = append(forwarder.messages, forwardedMessage{
		route: route,
		message: Message{
			From:       message.From,
			Recipients: append([]smtpidentity.Address(nil), message.Recipients...),
			Data:       append([]byte(nil), message.Data...),
		},
	})
	return nil
}

type forwardingTestClient struct {
	connection net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
}

func newForwardingTestClient(connection net.Conn) *forwardingTestClient {
	return &forwardingTestClient{
		connection: connection,
		reader:     bufio.NewReader(connection),
		writer:     bufio.NewWriter(connection),
	}
}

func (client *forwardingTestClient) send(testHandle *testing.T, line string) {
	testHandle.Helper()
	if _, err := client.writer.WriteString(line + "\r\n"); err != nil {
		testHandle.Fatalf("write command: %v", err)
	}
	if err := client.writer.Flush(); err != nil {
		testHandle.Fatalf("flush command: %v", err)
	}
}

func (client *forwardingTestClient) sendData(testHandle *testing.T, data string) {
	testHandle.Helper()
	if _, err := client.writer.WriteString(data + "\r\n.\r\n"); err != nil {
		testHandle.Fatalf("write data: %v", err)
	}
	if err := client.writer.Flush(); err != nil {
		testHandle.Fatalf("flush data: %v", err)
	}
}

func (client *forwardingTestClient) expectCode(testHandle *testing.T, prefix string) {
	testHandle.Helper()
	for {
		line, err := client.reader.ReadString('\n')
		if err != nil {
			testHandle.Fatalf("read response: %v", err)
		}
		if !strings.HasPrefix(line, prefix) {
			testHandle.Fatalf("expected code %s, got %q", prefix, line)
		}
		if len(line) >= 4 && line[3] == '-' {
			continue
		}
		return
	}
}

func (client *forwardingTestClient) close() {
	_ = client.connection.Close()
}

func mustRouteSet(testHandle *testing.T) RouteSet {
	testHandle.Helper()
	supportRoute, supportErr := NewRoute(
		mustAddress(testHandle, "support@help.example.com"),
		[]smtpidentity.Address{mustAddress(testHandle, "owner@example.com")},
	)
	if supportErr != nil {
		testHandle.Fatalf("support route: %v", supportErr)
	}
	salesRoute, salesErr := NewRoute(
		mustAddress(testHandle, "sales@help.example.com"),
		[]smtpidentity.Address{mustAddress(testHandle, "sales-owner@example.com")},
	)
	if salesErr != nil {
		testHandle.Fatalf("sales route: %v", salesErr)
	}
	routeSet, routeSetErr := NewRouteSet([]Route{supportRoute, salesRoute})
	if routeSetErr != nil {
		testHandle.Fatalf("route set: %v", routeSetErr)
	}
	return routeSet
}

func mustDial(testHandle *testing.T, address string) net.Conn {
	testHandle.Helper()
	connection, dialErr := net.Dial("tcp", address)
	if dialErr != nil {
		testHandle.Fatalf("dial: %v", dialErr)
	}
	return connection
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

type errorListener struct {
	err error
}

func (listener *errorListener) Accept() (net.Conn, error) {
	return nil, listener.err
}

func (listener *errorListener) Close() error {
	return nil
}

func (listener *errorListener) Addr() net.Addr {
	return fakeAddress("127.0.0.1:0")
}

type fakeConnection struct {
	remoteAddress net.Addr
}

func (connection *fakeConnection) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (connection *fakeConnection) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (connection *fakeConnection) Close() error {
	return nil
}

func (connection *fakeConnection) LocalAddr() net.Addr {
	return fakeAddress("local")
}

func (connection *fakeConnection) RemoteAddr() net.Addr {
	return connection.remoteAddress
}

func (connection *fakeConnection) SetDeadline(time.Time) error {
	return nil
}

func (connection *fakeConnection) SetReadDeadline(time.Time) error {
	return nil
}

func (connection *fakeConnection) SetWriteDeadline(time.Time) error {
	return nil
}

type deadlineErrorConnection struct {
	remoteAddress net.Addr
}

func (connection *deadlineErrorConnection) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (connection *deadlineErrorConnection) Write(bytes []byte) (int, error) {
	return len(bytes), nil
}

func (connection *deadlineErrorConnection) Close() error {
	return nil
}

func (connection *deadlineErrorConnection) LocalAddr() net.Addr {
	return fakeAddress("local")
}

func (connection *deadlineErrorConnection) RemoteAddr() net.Addr {
	return connection.remoteAddress
}

func (connection *deadlineErrorConnection) SetDeadline(time.Time) error {
	return nil
}

func (connection *deadlineErrorConnection) SetReadDeadline(time.Time) error {
	return errors.New("deadline failed")
}

func (connection *deadlineErrorConnection) SetWriteDeadline(time.Time) error {
	return nil
}

type fakeAddress string

func (address fakeAddress) Network() string {
	return "tcp"
}

func (address fakeAddress) String() string {
	return string(address)
}
