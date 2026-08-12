package tests

import (
	"strings"
	"testing"
)

func TestManagedTenantContractDeletesRuntimeBootstrapInputs(t *testing.T) {
	t.Helper()

	configPaths := []string{
		"configs/config.pinguin.yml",
		"configs/config.production.yml",
	}
	for _, configPath := range configPaths {
		contents := string(readRepoFile(t, configPath))
		for _, obsoleteInput := range []string{"grpcAuthToken:", "tenants:"} {
			if strings.Contains(contents, obsoleteInput) {
				t.Fatalf("%s contains obsolete runtime input %q", configPath, obsoleteInput)
			}
		}
	}

	manifest := string(readRepoFile(t, ".mprlab/deploy/resources.yml"))
	for _, obsoleteInput := range []string{"GRPC_AUTH_TOKEN", "PINGUIN_TENANT_ID_", "PINGUIN_PS_SMTP_"} {
		if strings.Contains(manifest, obsoleteInput) {
			t.Fatalf("deployment manifest contains obsolete private input %q", obsoleteInput)
		}
	}
}

func TestManagedTenantProtoReservesCallerTenantFields(t *testing.T) {
	t.Helper()

	proto := string(readRepoFile(t, "pkg/proto/pinguin.proto"))
	for _, reservation := range []string{
		"reserved 7;",
		`reserved "tenant_id";`,
		"reserved 13;",
		"reserved 2;",
		"reserved 3;",
	} {
		if !strings.Contains(proto, reservation) {
			t.Fatalf("protobuf contract is missing %q", reservation)
		}
	}
}
