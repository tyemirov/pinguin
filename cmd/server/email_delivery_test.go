package main

import (
	"testing"

	"github.com/tyemirov/pinguin/internal/config"
)

func TestRunServerEmailDelivery(t *testing.T) {
	cfg := serverTestConfig()
	cfg.WebInterfaceEnabled = true
	cfg.HTTPListenAddr = "127.0.0.1:8080"
	cfg.TAuthSigningKey = "signing-key"
	cfg.EmailDelivery = config.EmailDeliveryConfig{
		Enabled: true, Region: "us-east-1", AccountID: "299335570791",
		LogGroup: "/aws/events/pingwin-ses-delivery", ConfigurationSet: "pingwin-delivery-events",
		AccessKeyID: "test-access-key", SecretAccessKey: "test-secret-key",
	}
	state, dependencies := newServerTestDependencies(cfg)
	if code := runServer(nil, dependencies); code != 0 || state.httpConfig.DeliveryService == nil {
		t.Fatalf("enabled delivery reader was not wired into HTTP: exit=%d", code)
	}
	cfg.EmailDelivery.AccessKeyID = ""
	_, dependencies = newServerTestDependencies(cfg)
	if code := runServer(nil, dependencies); code != 1 {
		t.Fatalf("invalid provider configuration must stop startup: exit=%d", code)
	}
}
