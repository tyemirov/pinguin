package smtpsubmission

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/mail"
	"strings"
	"sync"

	"github.com/tyemirov/pinguin/internal/smtpidentity"
)

const (
	defaultMaxMessageBytes = int64(25 * 1024 * 1024)
	defaultMaxRecipients   = 100
)

// Config defines the SMTP submission server dependencies.
type Config struct {
	Hostname          string
	ListenAddr        string
	TLSListenAddr     string
	TLSConfig         *tls.Config
	MaxMessageBytes   int64
	MaxRecipients     int
	AllowInsecureAuth bool
	Authenticator     Authenticator
	Relay             RawRelay
	Logger            *slog.Logger
}

// Server accepts authenticated SMTP submissions.
type Server struct {
	config Config
	logger *slog.Logger
}

type sessionState struct {
	secure        bool
	authenticated *smtpidentity.AuthenticatedIdentity
	mailFrom      *smtpidentity.Address
	recipients    []smtpidentity.Address
}

type smtpListener struct {
	listener    net.Listener
	implicitTLS bool
}

// NewServer constructs an SMTP submission server.
func NewServer(cfg Config) (*Server, error) {
	if strings.TrimSpace(cfg.Hostname) == "" {
		return nil, errors.New("smtp submission: hostname is required")
	}
	if cfg.Authenticator == nil {
		return nil, errors.New("smtp submission: authenticator is required")
	}
	if cfg.Relay == nil {
		return nil, errors.New("smtp submission: relay is required")
	}
	if cfg.Logger == nil {
		return nil, errors.New("smtp submission: logger is required")
	}
	if strings.TrimSpace(cfg.TLSListenAddr) != "" && cfg.TLSConfig == nil {
		return nil, errors.New("smtp submission: tls listener requires tls config")
	}
	if strings.TrimSpace(cfg.ListenAddr) != "" && !cfg.AllowInsecureAuth && cfg.TLSConfig == nil {
		return nil, errors.New("smtp submission: starttls listener requires tls config")
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = defaultMaxMessageBytes
	}
	if cfg.MaxRecipients <= 0 {
		cfg.MaxRecipients = defaultMaxRecipients
	}
	return &Server{config: cfg, logger: cfg.Logger}, nil
}

// LoadTLSConfig loads certificate files for STARTTLS and implicit TLS.
func LoadTLSConfig(certPath string, keyPath string) (*tls.Config, error) {
	certificate, certErr := tls.LoadX509KeyPair(certPath, keyPath)
	if certErr != nil {
		return nil, fmt.Errorf("smtp submission: load tls certificate: %w", certErr)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// Start listens on configured SMTP submission addresses until the context ends.
func (server *Server) Start(ctx context.Context) error {
	var listeners []smtpListener
	if strings.TrimSpace(server.config.ListenAddr) != "" {
		listener, listenErr := net.Listen("tcp", server.config.ListenAddr)
		if listenErr != nil {
			return fmt.Errorf("smtp submission: listen %s: %w", server.config.ListenAddr, listenErr)
		}
		listeners = append(listeners, smtpListener{listener: listener})
	}
	if strings.TrimSpace(server.config.TLSListenAddr) != "" {
		listener, listenErr := tls.Listen("tcp", server.config.TLSListenAddr, server.config.TLSConfig)
		if listenErr != nil {
			closeListeners(listeners)
			return fmt.Errorf("smtp submission: tls listen %s: %w", server.config.TLSListenAddr, listenErr)
		}
		listeners = append(listeners, smtpListener{listener: listener, implicitTLS: true})
	}
	if len(listeners) == 0 {
		return errors.New("smtp submission: no listeners configured")
	}
	return server.serveListeners(ctx, listeners)
}

func (server *Server) serveListeners(ctx context.Context, listeners []smtpListener) error {
	errChan := make(chan error, len(listeners))
	var closeOnce sync.Once
	done := make(chan struct{})
	defer close(done)
	for _, listenerConfig := range listeners {
		go func(listener net.Listener, implicitTLS bool) {
			errChan <- server.Serve(ctx, listener, implicitTLS)
		}(listenerConfig.listener, listenerConfig.implicitTLS)
	}
	go func() {
		select {
		case <-ctx.Done():
			closeOnce.Do(func() { closeListeners(listeners) })
		case <-done:
		}
	}()
	for completedListeners := 0; completedListeners < len(listeners); completedListeners++ {
		serveErr := <-errChan
		if serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
			closeOnce.Do(func() { closeListeners(listeners) })
			for completedListeners++; completedListeners < len(listeners); completedListeners++ {
				<-errChan
			}
			return serveErr
		}
	}
	return nil
}

// Serve accepts connections from an existing listener.
func (server *Server) Serve(ctx context.Context, listener net.Listener, implicitTLS bool) error {
	for {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			return acceptErr
		}
		go server.handleConnection(ctx, connection, implicitTLS)
	}
}

func (server *Server) handleConnection(ctx context.Context, connection net.Conn, implicitTLS bool) {
	defer connection.Close()
	session := &sessionState{secure: implicitTLS}
	reader := bufio.NewReader(connection)
	writer := bufio.NewWriter(connection)
	if writeErr := writeSMTPLine(writer, "220 "+server.config.Hostname+" Pinguin SMTP submission ready"); writeErr != nil {
		return
	}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return
		}
		command, argument := splitCommand(line)
		switch command {
		case "EHLO", "HELO":
			server.handleHello(writer, session)
		case "STARTTLS":
			if session.secure {
				writeSMTPLine(writer, "503 TLS already active")
				continue
			}
			if server.config.TLSConfig == nil {
				writeSMTPLine(writer, "454 TLS not available")
				continue
			}
			if writeSMTPLine(writer, "220 Ready to start TLS") != nil {
				return
			}
			tlsConnection := tls.Server(connection, server.config.TLSConfig)
			if handshakeErr := tlsConnection.Handshake(); handshakeErr != nil {
				return
			}
			connection = tlsConnection
			reader = bufio.NewReader(connection)
			writer = bufio.NewWriter(connection)
			*session = sessionState{secure: true}
		case "AUTH":
			server.handleAuth(ctx, reader, writer, session, argument)
		case "MAIL":
			server.handleMail(writer, session, argument)
		case "RCPT":
			server.handleRecipient(writer, session, argument)
		case "DATA":
			server.handleData(ctx, reader, writer, session)
		case "RSET":
			session.mailFrom = nil
			session.recipients = nil
			writeSMTPLine(writer, "250 OK")
		case "NOOP":
			writeSMTPLine(writer, "250 OK")
		case "QUIT":
			writeSMTPLine(writer, "221 Bye")
			return
		default:
			writeSMTPLine(writer, "502 Command not implemented")
		}
	}
}

func (server *Server) handleHello(writer *bufio.Writer, session *sessionState) {
	lines := []string{
		"250-" + server.config.Hostname,
		fmt.Sprintf("250-SIZE %d", server.config.MaxMessageBytes),
	}
	if !session.secure && server.config.TLSConfig != nil {
		lines = append(lines, "250-STARTTLS")
	}
	if session.secure || server.config.AllowInsecureAuth {
		lines = append(lines, "250-AUTH PLAIN LOGIN")
	}
	lines = append(lines, "250 OK")
	for _, line := range lines {
		writeSMTPLine(writer, line)
	}
}

func (server *Server) handleAuth(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, session *sessionState, argument string) {
	if session.authenticated != nil {
		writeSMTPLine(writer, "503 Already authenticated")
		return
	}
	if !session.secure && !server.config.AllowInsecureAuth {
		writeSMTPLine(writer, "530 Must issue STARTTLS first")
		return
	}
	mechanism, payload := splitAuthArgument(argument)
	var username string
	var password string
	var parseErr error
	switch mechanism {
	case "PLAIN":
		username, password, parseErr = server.readPlainAuth(reader, writer, payload)
	case "LOGIN":
		username, password, parseErr = server.readLoginAuth(reader, writer, payload)
	default:
		writeSMTPLine(writer, "504 Authentication mechanism unsupported")
		return
	}
	if parseErr != nil {
		writeSMTPLine(writer, "535 Authentication failed")
		return
	}
	identity, authErr := server.config.Authenticator.Authenticate(ctx, username, password)
	if authErr != nil {
		writeSMTPLine(writer, "535 Authentication failed")
		return
	}
	session.authenticated = &identity
	writeSMTPLine(writer, "235 Authentication successful")
}

func (server *Server) readPlainAuth(reader *bufio.Reader, writer *bufio.Writer, payload string) (string, string, error) {
	if strings.TrimSpace(payload) == "" {
		if writeErr := writeSMTPLine(writer, "334 "); writeErr != nil {
			return "", "", writeErr
		}
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return "", "", readErr
		}
		payload = strings.TrimSpace(line)
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
	if decodeErr != nil {
		return "", "", decodeErr
	}
	parts := bytes.Split(decoded, []byte{0})
	if len(parts) != 3 {
		return "", "", errors.New("invalid plain auth")
	}
	return string(parts[1]), string(parts[2]), nil
}

func (server *Server) readLoginAuth(reader *bufio.Reader, writer *bufio.Writer, payload string) (string, string, error) {
	usernamePayload := strings.TrimSpace(payload)
	if usernamePayload == "" {
		if writeErr := writeSMTPLine(writer, "334 "+base64.StdEncoding.EncodeToString([]byte("Username:"))); writeErr != nil {
			return "", "", writeErr
		}
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return "", "", readErr
		}
		usernamePayload = strings.TrimSpace(line)
	}
	usernameBytes, usernameErr := base64.StdEncoding.DecodeString(usernamePayload)
	if usernameErr != nil {
		return "", "", usernameErr
	}
	if writeErr := writeSMTPLine(writer, "334 "+base64.StdEncoding.EncodeToString([]byte("Password:"))); writeErr != nil {
		return "", "", writeErr
	}
	passwordLine, passwordErr := reader.ReadString('\n')
	if passwordErr != nil {
		return "", "", passwordErr
	}
	passwordBytes, passwordDecodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(passwordLine))
	if passwordDecodeErr != nil {
		return "", "", passwordDecodeErr
	}
	return string(usernameBytes), string(passwordBytes), nil
}

func (server *Server) handleMail(writer *bufio.Writer, session *sessionState, argument string) {
	if session.authenticated == nil {
		writeSMTPLine(writer, "530 Authentication required")
		return
	}
	address, parseErr := parseSMTPPath(argument, "FROM:")
	if parseErr != nil {
		writeSMTPLine(writer, "501 Invalid sender")
		return
	}
	if !address.Equals(session.authenticated.EmailAddress) {
		writeSMTPLine(writer, "553 Sender not authorized")
		return
	}
	session.mailFrom = &address
	session.recipients = nil
	writeSMTPLine(writer, "250 OK")
}

func (server *Server) handleRecipient(writer *bufio.Writer, session *sessionState, argument string) {
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
	session.recipients = append(session.recipients, address)
	writeSMTPLine(writer, "250 OK")
}

func (server *Server) handleData(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, session *sessionState) {
	if session.authenticated == nil || session.mailFrom == nil || len(session.recipients) == 0 {
		writeSMTPLine(writer, "503 Need MAIL FROM and RCPT TO first")
		return
	}
	if writeSMTPLine(writer, "354 End data with <CR><LF>.<CR><LF>") != nil {
		return
	}
	data, tooLarge, readErr := server.readData(reader)
	if readErr != nil {
		return
	}
	if tooLarge {
		session.mailFrom = nil
		session.recipients = nil
		writeSMTPLine(writer, "552 Message too large")
		return
	}
	headerAddress, headerErr := parseMessageFrom(data)
	if headerErr != nil || !headerAddress.Equals(session.authenticated.EmailAddress) {
		session.mailFrom = nil
		session.recipients = nil
		writeSMTPLine(writer, "553 Sender not authorized")
		return
	}
	relayErr := server.config.Relay.Relay(ctx, RawMessage{
		IdentityID: session.authenticated.ID,
		From:       session.authenticated.EmailAddress,
		Recipients: append([]smtpidentity.Address(nil), session.recipients...),
		Data:       append([]byte(nil), data...),
	})
	session.mailFrom = nil
	session.recipients = nil
	if relayErr != nil {
		if errors.Is(relayErr, ErrRelayPermanent) {
			writeSMTPLine(writer, "554 Message rejected")
			return
		}
		writeSMTPLine(writer, "451 Requested action aborted")
		return
	}
	writeSMTPLine(writer, "250 Message accepted")
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

func parseMessageFrom(data []byte) (smtpidentity.Address, error) {
	message, readErr := mail.ReadMessage(bytes.NewReader(data))
	if readErr != nil {
		return smtpidentity.Address{}, readErr
	}
	return smtpidentity.ParseHeaderFromAddress(message.Header.Get("From"))
}

func splitCommand(line string) (string, string) {
	trimmedLine := strings.TrimRight(line, "\r\n")
	command, argument, hasArgument := strings.Cut(trimmedLine, " ")
	if !hasArgument {
		return strings.ToUpper(strings.TrimSpace(command)), ""
	}
	return strings.ToUpper(strings.TrimSpace(command)), strings.TrimSpace(argument)
}

func splitAuthArgument(argument string) (string, string) {
	mechanism, payload, hasPayload := strings.Cut(strings.TrimSpace(argument), " ")
	if !hasPayload {
		return strings.ToUpper(mechanism), ""
	}
	return strings.ToUpper(strings.TrimSpace(mechanism)), strings.TrimSpace(payload)
}

func parseSMTPPath(argument string, expectedPrefix string) (smtpidentity.Address, error) {
	trimmedArgument := strings.TrimSpace(argument)
	if !strings.HasPrefix(strings.ToUpper(trimmedArgument), expectedPrefix) {
		return smtpidentity.Address{}, errors.New("missing smtp path prefix")
	}
	path := strings.TrimSpace(trimmedArgument[len(expectedPrefix):])
	if strings.HasPrefix(path, "<") {
		endIndex := strings.Index(path, ">")
		if endIndex < 0 {
			return smtpidentity.Address{}, errors.New("unterminated smtp path")
		}
		path = path[1:endIndex]
	} else if fieldValues := strings.Fields(path); len(fieldValues) > 0 {
		path = fieldValues[0]
	}
	return smtpidentity.NewAddress(path)
}

func writeSMTPLine(writer *bufio.Writer, line string) error {
	if _, writeErr := writer.WriteString(line + "\r\n"); writeErr != nil {
		return writeErr
	}
	return writer.Flush()
}

func closeListeners(listeners []smtpListener) {
	for _, listenerConfig := range listeners {
		listenerConfig.listener.Close()
	}
}
