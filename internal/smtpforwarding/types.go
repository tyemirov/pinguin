package smtpforwarding

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/tyemirov/pinguin/internal/smtpidentity"
)

var (
	// ErrInvalidRoute indicates a forwarding route violates route invariants.
	ErrInvalidRoute = errors.New("smtp_forwarding.route.invalid")
	// ErrForwardTemporary indicates forwarding may succeed if the sender retries later.
	ErrForwardTemporary = errors.New("smtp_forwarding.forward_temporary")
)

// Forwarder immediately forwards an accepted inbound SMTP message.
type Forwarder interface {
	Forward(ctx context.Context, route Route, message Message) error
}

// RouteResolver resolves accepted inbound recipients to forwarding routes.
type RouteResolver interface {
	Resolve(ctx context.Context, address smtpidentity.Address) (Route, bool, error)
}

// Message carries an accepted inbound SMTP payload.
type Message struct {
	From       smtpidentity.Address
	Recipients []smtpidentity.Address
	Data       []byte
}

// Route maps one inbound address to its forwarding recipients.
type Route struct {
	address   smtpidentity.Address
	forwardTo []smtpidentity.Address
}

// NewRoute constructs a forwarding route.
func NewRoute(address smtpidentity.Address, forwardTo []smtpidentity.Address) (Route, error) {
	if address.String() == "" {
		return Route{}, fmt.Errorf("%w: address is required", ErrInvalidRoute)
	}
	seenRecipients := make(map[string]struct{}, len(forwardTo))
	recipients := make([]smtpidentity.Address, 0, len(forwardTo))
	for _, recipient := range forwardTo {
		if recipient.String() == "" {
			return Route{}, fmt.Errorf("%w: forward recipient is required", ErrInvalidRoute)
		}
		recipientKey := recipient.String()
		if _, exists := seenRecipients[recipientKey]; exists {
			return Route{}, fmt.Errorf("%w: duplicate forward recipient %s", ErrInvalidRoute, recipientKey)
		}
		seenRecipients[recipientKey] = struct{}{}
		recipients = append(recipients, recipient)
	}
	if len(recipients) == 0 {
		return Route{}, fmt.Errorf("%w: forward recipients are required", ErrInvalidRoute)
	}
	return Route{address: address, forwardTo: recipients}, nil
}

// Address returns the configured inbound route address.
func (route Route) Address() smtpidentity.Address {
	return route.address
}

// ForwardTo returns a copy of route forwarding recipients.
func (route Route) ForwardTo() []smtpidentity.Address {
	return append([]smtpidentity.Address(nil), route.forwardTo...)
}

// ForwardRecipientStrings returns normalized forwarding recipient addresses.
func (route Route) ForwardRecipientStrings() []string {
	recipients := make([]string, 0, len(route.forwardTo))
	for _, recipient := range route.forwardTo {
		recipients = append(recipients, recipient.String())
	}
	return recipients
}

// RouteSet resolves accepted inbound recipients to forwarding routes.
type RouteSet struct {
	routesByAddress map[string]Route
}

// NewRouteSet constructs a route set and rejects duplicate route addresses.
func NewRouteSet(routes []Route) (RouteSet, error) {
	routesByAddress := make(map[string]Route, len(routes))
	for _, route := range routes {
		routeKey := route.Address().String()
		if routeKey == "" {
			return RouteSet{}, fmt.Errorf("%w: address is required", ErrInvalidRoute)
		}
		if _, exists := routesByAddress[routeKey]; exists {
			return RouteSet{}, fmt.Errorf("%w: duplicate route %s", ErrInvalidRoute, routeKey)
		}
		routesByAddress[routeKey] = route
	}
	if len(routesByAddress) == 0 {
		return RouteSet{}, fmt.Errorf("%w: routes are required", ErrInvalidRoute)
	}
	return RouteSet{routesByAddress: routesByAddress}, nil
}

// Resolve returns the route for an accepted inbound recipient.
func (set RouteSet) Resolve(_ context.Context, address smtpidentity.Address) (Route, bool, error) {
	route, exists := set.routesByAddress[address.String()]
	return route, exists, nil
}

// Routes returns configured routes sorted by route address.
func (set RouteSet) Routes() []Route {
	keys := make([]string, 0, len(set.routesByAddress))
	for routeKey := range set.routesByAddress {
		keys = append(keys, routeKey)
	}
	sort.Strings(keys)
	routes := make([]Route, 0, len(keys))
	for _, routeKey := range keys {
		routes = append(routes, set.routesByAddress[routeKey])
	}
	return routes
}
