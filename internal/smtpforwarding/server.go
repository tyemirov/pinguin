package smtpforwarding

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tyemirov/pinguin/internal/smtpidentity"
)

const (
	defaultMaxMessageBytes          = int64(25 * 1024 * 1024)
	defaultMaxRecipients            = 100
	defaultCommandTimeout           = 2 * time.Minute
	defaultMaxConcurrentSessions    = 200
	defaultMaxSessionsPerRemoteHost = 20
)

// Config defines inbound SMTP forwarding server dependencies.
type Config struct {
	Hostname                 string
	ListenAddr               string
	MaxMessageBytes          int64
	MaxRecipients            int
	CommandTimeout           time.Duration
	MaxConcurrentSessions    int
	MaxSessionsPerRemoteHost int
	RouteResolver            RouteResolver
	Forwarder                Forwarder
	Logger                   *slog.Logger
}

// Server accepts inbound SMTP messages for dynamically resolved forwarding routes.
type Server struct {
	config  Config
	logger  *slog.Logger
	limiter *sessionLimiter
}

type sessionState struct {
	mailFrom        *ReversePath
	recipients      []smtpidentity.Address
	routesByAddress map[string]Route
}

type smtpPath struct {
	value     string
	bracketed bool
}

// NewServer constructs an inbound SMTP forwarding server.
func NewServer(configValues Config) (*Server, error) {
	if strings.TrimSpace(configValues.Hostname) == "" {
		return nil, errors.New("smtp forwarding: hostname is required")
	}
	if strings.TrimSpace(configValues.ListenAddr) == "" {
		return nil, errors.New("smtp forwarding: listen addr is required")
	}
	if configValues.RouteResolver == nil {
		return nil, errors.New("smtp forwarding: route resolver is required")
	}
	if configValues.Forwarder == nil {
		return nil, errors.New("smtp forwarding: forwarder is required")
	}
	if configValues.Logger == nil {
		return nil, errors.New("smtp forwarding: logger is required")
	}
	if configValues.MaxMessageBytes <= 0 {
		configValues.MaxMessageBytes = defaultMaxMessageBytes
	}
	if configValues.MaxRecipients <= 0 {
		configValues.MaxRecipients = defaultMaxRecipients
	}
	if configValues.CommandTimeout <= 0 {
		configValues.CommandTimeout = defaultCommandTimeout
	}
	if configValues.MaxConcurrentSessions <= 0 {
		configValues.MaxConcurrentSessions = defaultMaxConcurrentSessions
	}
	if configValues.MaxSessionsPerRemoteHost <= 0 {
		configValues.MaxSessionsPerRemoteHost = defaultMaxSessionsPerRemoteHost
	}
	return &Server{
		config:  configValues,
		logger:  configValues.Logger,
		limiter: newSessionLimiter(configValues.MaxConcurrentSessions, configValues.MaxSessionsPerRemoteHost),
	}, nil
}

// Start listens for inbound SMTP until the context is cancelled.
func (server *Server) Start(ctx context.Context) error {
	listener, listenErr := net.Listen("tcp", server.config.ListenAddr)
	if listenErr != nil {
		return fmt.Errorf("smtp forwarding: listen %s: %w", server.config.ListenAddr, listenErr)
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	return server.Serve(ctx, listener)
}

// Serve accepts inbound SMTP connections from an existing listener.
func (server *Server) Serve(ctx context.Context, listener net.Listener) error {
	for {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			if ctx.Err() != nil || errors.Is(acceptErr, net.ErrClosed) {
				return nil
			}
			return acceptErr
		}
		remoteHost := remoteHostForConnection(connection)
		releaseSession, acquireErr := server.limiter.acquire(remoteHost)
		if acquireErr != nil {
			server.logger.Warn("smtp_forwarding_session_throttled", "remote_host", remoteHost, "error", acquireErr)
			rejectSMTPConnection(connection, "421 Too many concurrent SMTP sessions")
			continue
		}
		go func() {
			defer releaseSession()
			server.handleConnection(ctx, connection)
		}()
	}
}

func (server *Server) handleConnection(ctx context.Context, connection net.Conn) {
	defer connection.Close()
	session := newSessionState()
	reader := bufio.NewReader(connection)
	writer := bufio.NewWriter(connection)
	if writeSMTPLine(writer, "220 "+server.config.Hostname+" Pinguin SMTP forwarding ready") != nil {
		return
	}
	for {
		if deadlineErr := setSMTPReadDeadline(connection, server.config.CommandTimeout); deadlineErr != nil {
			return
		}
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return
		}
		command, argument := splitCommand(line)
		switch command {
		case "EHLO", "HELO":
			server.handleHello(writer)
		case "MAIL":
			server.handleMail(writer, session, argument)
		case "RCPT":
			server.handleRecipient(ctx, writer, session, argument)
		case "DATA":
			server.handleData(ctx, connection, reader, writer, session)
		case "RSET":
			session.reset()
			writeSMTPLine(writer, "250 OK")
		case "NOOP":
			writeSMTPLine(writer, "250 OK")
		case "QUIT":
			writeSMTPLine(writer, "221 Bye")
			return
		case "AUTH", "STARTTLS":
			writeSMTPLine(writer, "502 Command not implemented")
		default:
			writeSMTPLine(writer, "502 Command not implemented")
		}
	}
}

func newSessionState() *sessionState {
	return &sessionState{routesByAddress: make(map[string]Route)}
}

func (session *sessionState) reset() {
	session.mailFrom = nil
	session.recipients = nil
	session.routesByAddress = make(map[string]Route)
}

func (server *Server) handleHello(writer *bufio.Writer) {
	lines := []string{
		"250-" + server.config.Hostname,
		fmt.Sprintf("250-SIZE %d", server.config.MaxMessageBytes),
		"250 OK",
	}
	for _, line := range lines {
		writeSMTPLine(writer, line)
	}
}

func (server *Server) handleMail(writer *bufio.Writer, session *sessionState, argument string) {
	reversePath, parseErr := parseSMTPReversePath(argument, "FROM:")
	if parseErr != nil {
		writeSMTPLine(writer, "501 Invalid sender")
		return
	}
	session.mailFrom = &reversePath
	session.recipients = nil
	session.routesByAddress = make(map[string]Route)
	writeSMTPLine(writer, "250 OK")
}

func (server *Server) handleRecipient(ctx context.Context, writer *bufio.Writer, session *sessionState, argument string) {
	if session.mailFrom == nil {
		writeSMTPLine(writer, "503 Need MAIL FROM first")
		return
	}
	if len(session.recipients) >= server.config.MaxRecipients {
		writeSMTPLine(writer, "452 Too many recipients")
		return
	}
	address, parseErr := parseSMTPPath(argument, "TO:")
	if parseErr != nil {
		writeSMTPLine(writer, "501 Invalid recipient")
		return
	}
	route, routeExists, routeErr := server.config.RouteResolver.Resolve(ctx, address)
	if routeErr != nil {
		writeSMTPLine(writer, "451 Requested action aborted")
		return
	}
	if !routeExists {
		writeSMTPLine(writer, "550 Recipient not configured")
		return
	}
	session.recipients = append(session.recipients, address)
	session.routesByAddress[route.Address().String()] = route
	writeSMTPLine(writer, "250 OK")
}

func (server *Server) handleData(ctx context.Context, connection net.Conn, reader *bufio.Reader, writer *bufio.Writer, session *sessionState) {
	if session.mailFrom == nil || len(session.recipients) == 0 {
		writeSMTPLine(writer, "503 Need MAIL FROM and RCPT TO first")
		return
	}
	if writeSMTPLine(writer, "354 End data with <CR><LF>.<CR><LF>") != nil {
		return
	}
	if deadlineErr := setSMTPReadDeadline(connection, server.config.CommandTimeout); deadlineErr != nil {
		return
	}
	data, tooLarge, readErr := server.readData(reader)
	if readErr != nil {
		return
	}
	if tooLarge {
		session.reset()
		writeSMTPLine(writer, "552 Message too large")
		return
	}
	message := Message{
		From:       *session.mailFrom,
		Recipients: append([]smtpidentity.Address(nil), session.recipients...),
		Data:       append([]byte(nil), data...),
	}
	forwardErr := server.forwardRoutes(ctx, session.routes(), message)
	session.reset()
	if forwardErr != nil {
		writeSMTPLine(writer, "451 Requested action aborted")
		return
	}
	writeSMTPLine(writer, "250 Message accepted for forwarding")
}

func (server *Server) forwardRoutes(ctx context.Context, routes []Route, message Message) error {
	for _, route := range routes {
		if forwardErr := server.config.Forwarder.Forward(ctx, route, message); forwardErr != nil {
			return forwardErr
		}
	}
	return nil
}

func (session *sessionState) routes() []Route {
	routeKeys := make([]string, 0, len(session.routesByAddress))
	for routeKey := range session.routesByAddress {
		routeKeys = append(routeKeys, routeKey)
	}
	sort.Strings(routeKeys)
	routes := make([]Route, 0, len(routeKeys))
	for _, routeKey := range routeKeys {
		routes = append(routes, session.routesByAddress[routeKey])
	}
	return routes
}

func (server *Server) readData(reader *bufio.Reader) ([]byte, bool, error) {
	var buffer bytes.Buffer
	tooLarge := false
	for {
		firstFragment := true
		for {
			fragment, readErr := reader.ReadSlice('\n')
			if readErr != nil && !errors.Is(readErr, bufio.ErrBufferFull) {
				if errors.Is(readErr, io.EOF) {
					return nil, false, readErr
				}
				return nil, false, readErr
			}
			lineComplete := !errors.Is(readErr, bufio.ErrBufferFull)
			if firstFragment && lineComplete && isSMTPDataTerminator(fragment) {
				return buffer.Bytes(), tooLarge, nil
			}
			if firstFragment && len(fragment) >= 2 && fragment[0] == '.' && fragment[1] == '.' {
				fragment = fragment[1:]
			}
			firstFragment = false
			if !tooLarge {
				if int64(buffer.Len()+len(fragment)) > server.config.MaxMessageBytes {
					tooLarge = true
				} else {
					buffer.Write(fragment)
				}
			}
			if lineComplete {
				break
			}
		}
	}
}

func isSMTPDataTerminator(line []byte) bool {
	return bytes.Equal(line, []byte(".\r\n")) || bytes.Equal(line, []byte(".\n"))
}

func splitCommand(line string) (string, string) {
	trimmedLine := strings.TrimRight(line, "\r\n")
	command, argument, hasArgument := strings.Cut(trimmedLine, " ")
	if !hasArgument {
		return strings.ToUpper(strings.TrimSpace(command)), ""
	}
	return strings.ToUpper(strings.TrimSpace(command)), strings.TrimSpace(argument)
}

func extractSMTPPath(argument string, expectedPrefix string) (smtpPath, error) {
	trimmedArgument := strings.TrimSpace(argument)
	if !strings.HasPrefix(strings.ToUpper(trimmedArgument), expectedPrefix) {
		return smtpPath{}, errors.New("missing smtp path prefix")
	}
	path := strings.TrimSpace(trimmedArgument[len(expectedPrefix):])
	if strings.HasPrefix(path, "<") {
		endIndex := strings.Index(path, ">")
		if endIndex < 0 {
			return smtpPath{}, errors.New("unterminated smtp path")
		}
		return smtpPath{value: path[1:endIndex], bracketed: true}, nil
	} else if fieldValues := strings.Fields(path); len(fieldValues) > 0 {
		path = fieldValues[0]
	}
	return smtpPath{value: path}, nil
}

func parseSMTPPath(argument string, expectedPrefix string) (smtpidentity.Address, error) {
	path, pathErr := extractSMTPPath(argument, expectedPrefix)
	if pathErr != nil {
		return smtpidentity.Address{}, pathErr
	}
	return smtpidentity.NewAddress(path.value)
}

func parseSMTPReversePath(argument string, expectedPrefix string) (ReversePath, error) {
	path, pathErr := extractSMTPPath(argument, expectedPrefix)
	if pathErr != nil {
		return ReversePath{}, pathErr
	}
	if path.value == "" && !path.bracketed {
		return ReversePath{}, errors.New("missing smtp reverse path")
	}
	return NewReversePath(path.value)
}

func writeSMTPLine(writer *bufio.Writer, line string) error {
	if _, writeErr := writer.WriteString(line + "\r\n"); writeErr != nil {
		return writeErr
	}
	return writer.Flush()
}

func rejectSMTPConnection(connection net.Conn, line string) {
	defer connection.Close()
	writer := bufio.NewWriter(connection)
	_ = writeSMTPLine(writer, line)
}

func setSMTPReadDeadline(connection net.Conn, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}
	return connection.SetReadDeadline(time.Now().Add(timeout))
}

func remoteHostForConnection(connection net.Conn) string {
	host, _, splitErr := net.SplitHostPort(connection.RemoteAddr().String())
	if splitErr != nil {
		return connection.RemoteAddr().String()
	}
	return host
}

type sessionLimiter struct {
	mutex                    sync.Mutex
	activeSessions           int
	activeSessionsByHost     map[string]int
	maxConcurrentSessions    int
	maxSessionsPerRemoteHost int
}

func newSessionLimiter(maxConcurrentSessions int, maxSessionsPerRemoteHost int) *sessionLimiter {
	return &sessionLimiter{
		activeSessionsByHost:     make(map[string]int),
		maxConcurrentSessions:    maxConcurrentSessions,
		maxSessionsPerRemoteHost: maxSessionsPerRemoteHost,
	}
}

func (limiter *sessionLimiter) acquire(remoteHost string) (func(), error) {
	normalizedRemoteHost := strings.TrimSpace(remoteHost)
	if normalizedRemoteHost == "" {
		normalizedRemoteHost = "unknown"
	}
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	if limiter.activeSessions >= limiter.maxConcurrentSessions {
		return nil, errors.New("smtp_forwarding.concurrent_sessions_exceeded")
	}
	if limiter.activeSessionsByHost[normalizedRemoteHost] >= limiter.maxSessionsPerRemoteHost {
		return nil, errors.New("smtp_forwarding.remote_sessions_exceeded")
	}
	limiter.activeSessions++
	limiter.activeSessionsByHost[normalizedRemoteHost]++
	return func() {
		limiter.release(normalizedRemoteHost)
	}, nil
}

func (limiter *sessionLimiter) release(remoteHost string) {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	if limiter.activeSessions > 0 {
		limiter.activeSessions--
	}
	if limiter.activeSessionsByHost[remoteHost] <= 1 {
		delete(limiter.activeSessionsByHost, remoteHost)
		return
	}
	limiter.activeSessionsByHost[remoteHost]--
}
