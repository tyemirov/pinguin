package integrationtest

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/httpapi"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/internal/tenant"
	"log/slog"
)

func TestHealthzBypassesTenantResolution(t *testing.T) {
	t.Helper()

	db, secretKeeper := setupTestDB(t)
	configFile := setupTenantConfig(t)
	if err := tenant.BootstrapFromFile(context.Background(), db, secretKeeper, configFile); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	repo := tenant.NewRepository(db, secretKeeper)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := service.NewNotificationService(db, logger, config.Config{}, repo)

	addr := allocateFreeAddr(t)
	server, err := httpapi.NewServer(httpapi.Config{
		ListenAddr:          addr,
		SessionValidator:    &mockSessionValidator{},
		NotificationService: svc,
		TenantRepository:    repo,
		TAuthBaseURL:        "https://tauth.example.com",
		TAuthTenantID:       "tauth-test",
		TAuthGoogleClientID: "client-id",
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}

	go func() { _ = server.Start() }()
	defer func() { _ = server.Shutdown(context.Background()) }()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	url := "http://" + addr + "/healthz"
	deadline := time.Now().Add(3 * time.Second)
	for {
		request, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("build request error: %v", err)
		}
		request.Host = "unknown.localhost"

		response, err := client.Do(request)
		if err == nil {
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 for healthz, got %d", response.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("healthz request error: %v", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func allocateFreeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	addr := listener.Addr().String()
	if closeErr := listener.Close(); closeErr != nil {
		t.Fatalf("close port listener: %v", closeErr)
	}
	return addr
}
