package smtpidentity

import (
	"errors"
	"testing"
)

func TestAddressValidationAndHeaderParsing(t *testing.T) {
	address, err := NewAddress("Alice@Example.COM")
	if err != nil {
		t.Fatalf("new address: %v", err)
	}
	if address.String() != "alice@example.com" || address.Domain() != "example.com" {
		t.Fatalf("unexpected normalized address %s domain %s", address.String(), address.Domain())
	}

	headerAddress, headerErr := ParseHeaderFromAddress("<alice@example.com>")
	if headerErr != nil {
		t.Fatalf("parse header: %v", headerErr)
	}
	if !address.Equals(headerAddress) {
		t.Fatalf("expected addresses to match")
	}
}

func TestAddressRejectsInvalidInputs(t *testing.T) {
	for _, input := range []string{"", "alice@example.com\nbcc@example.com", "Alice <alice@example.com>", "missing-at"} {
		if _, err := NewAddress(input); !errors.Is(err, ErrInvalidAddress) {
			t.Fatalf("expected invalid address for %q, got %v", input, err)
		}
	}
	if _, err := ParseHeaderFromAddress("alice@example.com, bob@example.com"); !errors.Is(err, ErrInvalidAddress) {
		t.Fatalf("expected invalid multi-address header, got %v", err)
	}
}
