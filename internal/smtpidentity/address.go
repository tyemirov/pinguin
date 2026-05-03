package smtpidentity

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
)

var (
	// ErrInvalidAddress indicates an address cannot be used as an SMTP identity.
	ErrInvalidAddress = errors.New("smtp_identity.address.invalid")
)

// Address is a normalized exact sender address.
type Address struct {
	value  string
	domain string
}

// NewAddress validates and normalizes a sender address.
func NewAddress(rawAddress string) (Address, error) {
	trimmedAddress := strings.TrimSpace(rawAddress)
	if trimmedAddress == "" || strings.ContainsAny(trimmedAddress, "\r\n") {
		return Address{}, ErrInvalidAddress
	}
	parsedAddress, parseErr := mail.ParseAddress(trimmedAddress)
	if parseErr != nil {
		return Address{}, fmt.Errorf("%w: %s", ErrInvalidAddress, trimmedAddress)
	}
	if parsedAddress.Name != "" || parsedAddress.Address != trimmedAddress {
		return Address{}, fmt.Errorf("%w: %s", ErrInvalidAddress, trimmedAddress)
	}
	localPart, domainPart, _ := strings.Cut(parsedAddress.Address, "@")
	normalizedDomain := strings.ToLower(strings.TrimSpace(domainPart))
	normalizedLocal := strings.ToLower(strings.TrimSpace(localPart))
	return Address{
		value:  normalizedLocal + "@" + normalizedDomain,
		domain: normalizedDomain,
	}, nil
}

// ParseHeaderFromAddress extracts the single From mailbox from an RFC 5322 header.
func ParseHeaderFromAddress(fromHeader string) (Address, error) {
	parsedAddresses, parseErr := mail.ParseAddressList(strings.TrimSpace(fromHeader))
	if parseErr != nil || len(parsedAddresses) != 1 {
		return Address{}, fmt.Errorf("%w: from header", ErrInvalidAddress)
	}
	return NewAddress(parsedAddresses[0].Address)
}

// String returns the normalized address.
func (address Address) String() string {
	return address.value
}

// Domain returns the normalized domain part.
func (address Address) Domain() string {
	return address.domain
}

// Equals reports whether two addresses identify the same normalized mailbox.
func (address Address) Equals(candidate Address) bool {
	return address.value == candidate.value
}
