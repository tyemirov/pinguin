package smtpforwarding

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/tyemirov/pinguin/internal/smtpidentity"
)

func TestRouteSetValidationAndAccessors(testHandle *testing.T) {
	routeAddress := mustAddress(testHandle, "support@help.example.com")
	firstRecipient := mustAddress(testHandle, "alice@example.com")
	secondRecipient := mustAddress(testHandle, "maria@example.com")
	route, routeErr := NewRoute(routeAddress, []smtpidentity.Address{firstRecipient, secondRecipient})
	if routeErr != nil {
		testHandle.Fatalf("new route: %v", routeErr)
	}
	if route.Address().String() != "support@help.example.com" {
		testHandle.Fatalf("unexpected route address %s", route.Address().String())
	}
	forwardTo := route.ForwardTo()
	forwardTo[0] = secondRecipient
	if route.ForwardTo()[0].String() != "alice@example.com" {
		testHandle.Fatalf("expected ForwardTo to return a copy")
	}
	recipientStrings := route.ForwardRecipientStrings()
	if strings.Join(recipientStrings, ",") != "alice@example.com,maria@example.com" {
		testHandle.Fatalf("unexpected recipient strings %v", recipientStrings)
	}

	secondRoute, secondRouteErr := NewRoute(
		mustAddress(testHandle, "sales@help.example.com"),
		[]smtpidentity.Address{mustAddress(testHandle, "sales@example.com")},
	)
	if secondRouteErr != nil {
		testHandle.Fatalf("new second route: %v", secondRouteErr)
	}
	routeSet, routeSetErr := NewRouteSet([]Route{secondRoute, route})
	if routeSetErr != nil {
		testHandle.Fatalf("new route set: %v", routeSetErr)
	}
	if resolved, exists, resolveErr := routeSet.Resolve(context.Background(), routeAddress); resolveErr != nil || !exists || resolved.Address().String() != routeAddress.String() {
		testHandle.Fatalf("expected route to resolve")
	}
	if _, exists, resolveErr := routeSet.Resolve(context.Background(), mustAddress(testHandle, "missing@help.example.com")); resolveErr != nil || exists {
		testHandle.Fatalf("unexpected missing route")
	}
	routes := routeSet.Routes()
	if len(routes) != 2 || routes[0].Address().String() != "sales@help.example.com" || routes[1].Address().String() != "support@help.example.com" {
		testHandle.Fatalf("expected sorted routes, got %+v", routes)
	}
}

func TestRouteValidationRejectsInvalidRoutes(testHandle *testing.T) {
	validAddress := mustAddress(testHandle, "support@help.example.com")
	validRecipient := mustAddress(testHandle, "owner@example.com")
	for _, testCase := range []struct {
		name       string
		route      Route
		routeSlice []Route
	}{
		{name: "empty address", route: Route{forwardTo: []smtpidentity.Address{validRecipient}}},
		{name: "empty recipient", route: Route{address: validAddress, forwardTo: []smtpidentity.Address{{}}}},
		{name: "no recipients", route: Route{address: validAddress}},
		{
			name:  "duplicate recipient",
			route: Route{address: validAddress, forwardTo: []smtpidentity.Address{validRecipient, validRecipient}},
		},
		{name: "empty route set address", routeSlice: []Route{{}}},
		{name: "duplicate route set", routeSlice: []Route{
			{address: validAddress, forwardTo: []smtpidentity.Address{validRecipient}},
			{address: validAddress, forwardTo: []smtpidentity.Address{validRecipient}},
		}},
		{name: "empty route set", routeSlice: []Route{}},
	} {
		testCase := testCase
		testHandle.Run(testCase.name, func(t *testing.T) {
			if testCase.routeSlice != nil {
				_, routeSetErr := NewRouteSet(testCase.routeSlice)
				if !errors.Is(routeSetErr, ErrInvalidRoute) {
					t.Fatalf("expected invalid route set error, got %v", routeSetErr)
				}
				return
			}
			_, routeErr := NewRoute(testCase.route.address, testCase.route.forwardTo)
			if !errors.Is(routeErr, ErrInvalidRoute) {
				t.Fatalf("expected invalid route error, got %v", routeErr)
			}
		})
	}
}

func TestRelayForwarder(testHandle *testing.T) {
	route := mustRoute(testHandle)
	message := Message{Data: []byte("From: sender@example.net\r\n\r\nHello")}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	if _, err := NewRelayForwarder(nil, logger); err == nil {
		testHandle.Fatalf("expected nil sender error")
	}
	if _, err := NewRelayForwarder(&recordingRawEmailSender{}, nil); err == nil {
		testHandle.Fatalf("expected nil logger error")
	}

	sender := &recordingRawEmailSender{}
	forwarder, forwarderErr := NewRelayForwarder(sender, logger)
	if forwarderErr != nil {
		testHandle.Fatalf("new forwarder: %v", forwarderErr)
	}
	if err := forwarder.Forward(context.Background(), route, message); err != nil {
		testHandle.Fatalf("forward: %v", err)
	}
	if sender.fromAddress != route.Address().String() {
		testHandle.Fatalf("unexpected sender %q", sender.fromAddress)
	}
	if strings.Join(sender.recipients, ",") != "owner@example.com" {
		testHandle.Fatalf("unexpected recipients %v", sender.recipients)
	}
	if string(sender.rawMessage) != string(message.Data) {
		testHandle.Fatalf("expected raw message to be preserved")
	}

	sender.err = errors.New("relay down")
	if err := forwarder.Forward(context.Background(), route, message); !errors.Is(err, ErrForwardTemporary) {
		testHandle.Fatalf("expected temporary forward error, got %v", err)
	}
}

type recordingRawEmailSender struct {
	fromAddress string
	recipients  []string
	rawMessage  []byte
	err         error
}

func (sender *recordingRawEmailSender) SendRawEmail(_ context.Context, fromAddress string, recipients []string, rawMessage []byte) error {
	sender.fromAddress = fromAddress
	sender.recipients = append([]string(nil), recipients...)
	sender.rawMessage = append([]byte(nil), rawMessage...)
	return sender.err
}

func mustAddress(testHandle *testing.T, rawAddress string) smtpidentity.Address {
	testHandle.Helper()
	address, addressErr := smtpidentity.NewAddress(rawAddress)
	if addressErr != nil {
		testHandle.Fatalf("address %s: %v", rawAddress, addressErr)
	}
	return address
}

func mustRoute(testHandle *testing.T) Route {
	testHandle.Helper()
	route, routeErr := NewRoute(
		mustAddress(testHandle, "support@help.example.com"),
		[]smtpidentity.Address{mustAddress(testHandle, "owner@example.com")},
	)
	if routeErr != nil {
		testHandle.Fatalf("route: %v", routeErr)
	}
	return route
}
