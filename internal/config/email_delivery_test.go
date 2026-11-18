package config

import (
	"strings"
	"testing"
)

func TestEmailDeliveryConfig(t *testing.T) {
	base := `
server:
  databasePath: delivery.db
  logLevel: INFO
  maxRetries: 2
  retryIntervalSec: 1
  masterEncryptionKey: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  connectionTimeoutSec: 3
  operationTimeoutSec: 5
web:
  enabled: false
`
	section := `
emailDelivery:
  enabled: true
  region: us-east-1
  accountId: "299335570791"
  logGroup: /aws/events/pingwin-ses-delivery
  configurationSet: pingwin-delivery-events
  accessKeyId: ${DELIVERY_TEST_ACCESS_KEY}
  secretAccessKey: ${DELIVERY_TEST_SECRET_KEY}
  sessionToken: session-value
`
	t.Setenv("DELIVERY_TEST_ACCESS_KEY", "test-access-key")
	t.Setenv("DELIVERY_TEST_SECRET_KEY", "test-secret-key")
	cfg, err := loadConfigFromPath(writeConfigFile(t, base+section))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EmailDelivery.Enabled || cfg.EmailDelivery.Region != "us-east-1" || cfg.EmailDelivery.AccountID != "299335570791" || cfg.EmailDelivery.LogGroup != "/aws/events/pingwin-ses-delivery" || cfg.EmailDelivery.ConfigurationSet != "pingwin-delivery-events" || cfg.EmailDelivery.AccessKeyID != "test-access-key" || cfg.EmailDelivery.SecretAccessKey != "test-secret-key" || cfg.EmailDelivery.SessionToken != "session-value" {
		t.Fatal("email delivery provider configuration was not loaded")
	}
	_, err = loadConfigFromPath(writeConfigFile(t, base+"\nemailDelivery:\n  enabled: true\n"))
	if err == nil || !strings.Contains(err.Error(), "emailDelivery.region") || !strings.Contains(err.Error(), "emailDelivery.secretAccessKey") {
		t.Fatalf("incomplete enabled configuration must fail: %v", err)
	}
	cfg, err = loadConfigFromPath(writeConfigFile(t, base))
	if err != nil || cfg.EmailDelivery.Enabled {
		t.Fatalf("delivery history must remain explicitly unavailable without configuration: %v", err)
	}
}
