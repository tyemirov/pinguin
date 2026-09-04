package integrationtest

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
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
	repo := tenant.NewRepository(db, secretKeeper)
	var events bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&events, &slog.HandlerOptions{Level: slog.LevelInfo}))
	svc := service.NewNotificationService(db, logger, config.Config{}, repo)

	addr := allocateFreeAddr(t)
	server, err := httpapi.NewServer(httpapi.Config{
		ListenAddr:          addr,
		SessionValidator:    &mockSessionValidator{},
		NotificationService: svc,
		TenantRepository:    repo,
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
			if response.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("health cache header: %v", response.Header)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("healthz request error: %v", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	if events.Len() != 0 {
		t.Fatalf("successful probe emitted events: %s", events.String())
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusServiceUnavailable || string(body) != `{"status":"unavailable"}` || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("unavailable health: %d %v %s", response.StatusCode, response.Header, body)
	}
	if !strings.Contains(events.String(), "health check failed") {
		t.Fatalf("missing failure diagnostics: %s", events.String())
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
