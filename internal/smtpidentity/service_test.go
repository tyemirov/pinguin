package smtpidentity

import (
	"context"
	"testing"
)

func TestServiceWrapsRepositoryWorkflows(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	service := NewService(repository, PublicSettings{
		Host:         "smtp.example.com",
		Port:         587,
		SecurityMode: "starttls",
	})

	created, createErr := service.Create(context.Background(), mustAddress(t, "alice@example.com"))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if created.Username == "" || created.Password == "" {
		t.Fatalf("expected one-time credentials")
	}
	if created.SMTPSettings.Host != "smtp.example.com" {
		t.Fatalf("unexpected public settings %+v", created.SMTPSettings)
	}

	identities, listErr := service.List(context.Background())
	if listErr != nil {
		t.Fatalf("list identities: %v", listErr)
	}
	if len(identities) != 1 {
		t.Fatalf("expected one identity, got %d", len(identities))
	}

	rotated, rotateErr := service.Rotate(context.Background(), " "+created.Identity.ID+" ")
	if rotateErr != nil {
		t.Fatalf("rotate identity: %v", rotateErr)
	}
	if rotated.Password == created.Password {
		t.Fatalf("expected rotated password to change")
	}

	if deleteErr := service.Delete(context.Background(), " "+created.Identity.ID+" "); deleteErr != nil {
		t.Fatalf("delete identity: %v", deleteErr)
	}
}

func TestServicePropagatesRepositoryErrors(t *testing.T) {
	repository, _ := newIdentityRepository(t)
	service := NewService(repository, PublicSettings{})

	if _, createErr := service.Create(context.Background(), mustAddress(t, "alice@example.com")); createErr == nil {
		t.Fatalf("expected create error without sender domain")
	}
	if _, rotateErr := service.Rotate(context.Background(), "missing"); rotateErr == nil {
		t.Fatalf("expected rotate error")
	}
	if deleteErr := service.Delete(context.Background(), "missing"); deleteErr == nil {
		t.Fatalf("expected delete error")
	}
}
