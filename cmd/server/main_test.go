package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/pkg/client"
	"github.com/tyemirov/pinguin/pkg/grpcapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log/slog"
)

func TestNotificationServerHandlesClientRequests(t *testing.T) {
	t.Helper()

	authToken := "unit-token"
	notificationService := &stubNotificationService{
		sendResponse: model.NotificationResponse{
			NotificationID:   "notif-123",
			NotificationType: model.NotificationEmail,
			Recipient:        "user@example.com",
			Message:          "Hello",
			Status:           model.StatusSent,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		},
		statusResponses: []model.NotificationResponse{
			{
				NotificationID:   "notif-123",
				NotificationType: model.NotificationEmail,
				Recipient:        "user@example.com",
				Message:          "Hello",
				Status:           model.StatusSent,
				CreatedAt:        time.Now().UTC(),
				UpdatedAt:        time.Now().UTC(),
			},
		},
	}

	serverAddress, shutdown := startTestNotificationServer(t, notificationService, authToken)
	defer shutdown()

	settings, settingsErr := client.NewSettings(serverAddress, authToken, 5, 5)
	if settingsErr != nil {
		t.Fatalf("settings error: %v", settingsErr)
	}
	clientLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	notificationClient, clientError := client.NewNotificationClient(clientLogger, settings)
	if clientError != nil {
		t.Fatalf("create client error: %v", clientError)
	}
	defer notificationClient.Close()

	grpcRequest := &grpcapi.NotificationRequest{
		NotificationType: grpcapi.NotificationType_EMAIL,
		Recipient:        "user@example.com",
		Subject:          "Unit",
		Message:          "Hello",
	}

	sendResponse, sendError := notificationClient.SendNotification(context.Background(), grpcRequest)
	if sendError != nil {
		t.Fatalf("send notification error: %v", sendError)
	}
	if sendResponse.NotificationId != "notif-123" {
		t.Fatalf("unexpected notification id %s", sendResponse.NotificationId)
	}

	statusResponse, statusError := notificationClient.GetNotificationStatus("notif-123")
	if statusError != nil {
		t.Fatalf("status retrieval error: %v", statusError)
	}
	if statusResponse.Status != grpcapi.Status_SENT {
		t.Fatalf("unexpected status %v", statusResponse.Status)
	}

	waitResponse, waitError := notificationClient.SendNotificationAndWait(grpcRequest)
	if waitError != nil {
		t.Fatalf("send and wait error: %v", waitError)
	}
	if waitResponse.Status != grpcapi.Status_SENT {
		t.Fatalf("unexpected wait status %v", waitResponse.Status)
	}

	if len(notificationService.sendCalls) != 2 {
		t.Fatalf("expected two send calls, got %d", len(notificationService.sendCalls))
	}
	if len(notificationService.statusCalls) == 0 {
		t.Fatalf("expected status calls")
	}
}

func TestSendNotificationRejectsInvalidScheduledTimestamp(t *testing.T) {
	t.Helper()

	invalidTimestamp := &timestamppb.Timestamp{Seconds: 1, Nanos: 1_000_000_000}

	notificationService := &stubNotificationService{}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	server := &notificationServiceServer{notificationService: notificationService, logger: logger}

	_, sendError := server.SendNotification(context.Background(), &grpcapi.NotificationRequest{
		NotificationType: grpcapi.NotificationType_EMAIL,
		Recipient:        "user@example.com",
		Message:          "Hello",
		ScheduledTime:    invalidTimestamp,
	})
	if sendError == nil {
		t.Fatalf("expected validation error")
	}
	if status.Code(sendError) != codes.InvalidArgument {
		t.Fatalf("unexpected status code %v", status.Code(sendError))
	}
	if len(notificationService.sendCalls) != 0 {
		t.Fatalf("unexpected service invocation")
	}
}

func TestListNotificationsTranslatesStatusesAndResponses(t *testing.T) {
	t.Helper()

	currentTime := time.Now().UTC()
	testCases := []struct {
		name             string
		requestStatuses  []grpcapi.Status
		expectedStatuses []model.NotificationStatus
	}{
		{
			name:             "NoStatusesForwardedAsEmpty",
			requestStatuses:  nil,
			expectedStatuses: nil,
		},
		{
			name: "MapsKnownStatuses",
			requestStatuses: []grpcapi.Status{
				grpcapi.Status_SENT,
				grpcapi.Status_FAILED,
				grpcapi.Status_CANCELLED,
			},
			expectedStatuses: []model.NotificationStatus{
				model.StatusSent,
				model.StatusFailed,
				model.StatusCancelled,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			stubService := &stubNotificationService{
				listResponses: []model.NotificationResponse{
					{
						NotificationID:   "notif-1",
						NotificationType: model.NotificationEmail,
						Recipient:        "alpha@example.com",
						Message:          "Hello",
						Status:           model.StatusSent,
						CreatedAt:        currentTime,
						UpdatedAt:        currentTime,
					},
					{
						NotificationID:   "notif-2",
						NotificationType: model.NotificationSMS,
						Recipient:        "beta@example.com",
						Message:          "Hi",
						Status:           model.StatusFailed,
						CreatedAt:        currentTime,
						UpdatedAt:        currentTime,
					},
				},
			}

			logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
			server := &notificationServiceServer{notificationService: stubService, logger: logger}

			response, err := server.ListNotifications(context.Background(), &grpcapi.ListNotificationsRequest{Statuses: testCase.requestStatuses})
			if err != nil {
				t.Fatalf("ListNotifications error: %v", err)
			}

			stubService.mutex.Lock()
			if len(stubService.listCalls) != 1 {
				stubService.mutex.Unlock()
				t.Fatalf("expected one list call, got %d", len(stubService.listCalls))
			}
			capturedStatuses := stubService.listCalls[0].Statuses
			stubService.mutex.Unlock()

			if len(testCase.expectedStatuses) == 0 {
				if len(capturedStatuses) != 0 {
					t.Fatalf("expected no statuses, got %v", capturedStatuses)
				}
			} else {
				if len(capturedStatuses) != len(testCase.expectedStatuses) {
					t.Fatalf("expected %d statuses, got %d", len(testCase.expectedStatuses), len(capturedStatuses))
				}
				for index, expectedStatus := range testCase.expectedStatuses {
					if capturedStatuses[index] != expectedStatus {
						t.Fatalf("status at index %d mismatch: expected %v, got %v", index, expectedStatus, capturedStatuses[index])
					}
				}
			}

			if len(response.Notifications) != 2 {
				t.Fatalf("expected two notifications, got %d", len(response.Notifications))
			}
			if response.Notifications[0].NotificationId != "notif-1" {
				t.Fatalf("unexpected first notification id %s", response.Notifications[0].NotificationId)
			}
			if response.Notifications[1].NotificationType != grpcapi.NotificationType_SMS {
				t.Fatalf("expected SMS notification type for second element")
			}
		})
	}
}

func TestRescheduleNotificationValidatesAndForwardsRequest(t *testing.T) {
	t.Helper()

	validTime := time.Date(2024, 1, 10, 15, 45, 0, 0, time.FixedZone("UTC+3", 3*60*60))
	validTimestamp := timestamppb.New(validTime)

	testCases := []struct {
		name              string
		request           *grpcapi.RescheduleNotificationRequest
		expectedCode      codes.Code
		expectServiceCall bool
	}{
		{
			name: "MissingNotificationID",
			request: &grpcapi.RescheduleNotificationRequest{
				ScheduledTime: validTimestamp,
			},
			expectedCode:      codes.InvalidArgument,
			expectServiceCall: false,
		},
		{
			name: "MissingScheduledTime",
			request: &grpcapi.RescheduleNotificationRequest{
				NotificationId: "notif-1",
			},
			expectedCode:      codes.InvalidArgument,
			expectServiceCall: false,
		},
		{
			name: "InvalidTimestamp",
			request: &grpcapi.RescheduleNotificationRequest{
				NotificationId: "notif-1",
				ScheduledTime:  &timestamppb.Timestamp{Seconds: 1, Nanos: 1_000_000_000},
			},
			expectedCode:      codes.InvalidArgument,
			expectServiceCall: false,
		},
		{
			name: "SuccessfulRequest",
			request: &grpcapi.RescheduleNotificationRequest{
				NotificationId: "notif-1",
				ScheduledTime:  validTimestamp,
			},
			expectedCode:      codes.OK,
			expectServiceCall: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			stubService := &stubNotificationService{
				rescheduleResponse: model.NotificationResponse{
					NotificationID:   "notif-1",
					NotificationType: model.NotificationEmail,
					Recipient:        "alpha@example.com",
					Message:          "Updated",
					Status:           model.StatusQueued,
					CreatedAt:        validTime.UTC(),
					UpdatedAt:        validTime.UTC(),
				},
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
			server := &notificationServiceServer{notificationService: stubService, logger: logger}

			response, err := server.RescheduleNotification(context.Background(), testCase.request)

			if testCase.expectedCode == codes.OK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if response.NotificationId != "notif-1" {
					t.Fatalf("unexpected notification id %s", response.NotificationId)
				}
				stubService.mutex.Lock()
				if len(stubService.rescheduleCalls) != 1 {
					stubService.mutex.Unlock()
					t.Fatalf("expected one reschedule call, got %d", len(stubService.rescheduleCalls))
				}
				capturedCall := stubService.rescheduleCalls[0]
				stubService.mutex.Unlock()

				if capturedCall.notificationID != "notif-1" {
					t.Fatalf("unexpected notification id %s", capturedCall.notificationID)
				}
				if !capturedCall.scheduledFor.Equal(validTime.UTC()) {
					t.Fatalf("scheduled time not normalized to UTC: %v", capturedCall.scheduledFor)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error but received nil")
			}
			if status.Code(err) != testCase.expectedCode {
				t.Fatalf("expected code %v, got %v", testCase.expectedCode, status.Code(err))
			}
			stubService.mutex.Lock()
			defer stubService.mutex.Unlock()
			if testCase.expectServiceCall && len(stubService.rescheduleCalls) == 0 {
				t.Fatalf("expected service call")
			}
			if !testCase.expectServiceCall && len(stubService.rescheduleCalls) != 0 {
				t.Fatalf("unexpected service call recorded")
			}
		})
	}
}

func TestCancelNotificationValidatesAndForwardsRequest(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name               string
		notificationID     string
		serviceError       error
		expectStatusError  bool
		expectedStatusCode codes.Code
	}{
		{
			name:               "MissingNotificationID",
			notificationID:     "",
			expectStatusError:  true,
			expectedStatusCode: codes.InvalidArgument,
		},
		{
			name:           "ServiceFailurePropagated",
			notificationID: "notif-failure",
			serviceError:   errors.New("failure"),
		},
		{
			name:           "SuccessfulCancellation",
			notificationID: "notif-1",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			stubService := &stubNotificationService{
				cancelResponse: model.NotificationResponse{
					NotificationID:   "notif-1",
					NotificationType: model.NotificationEmail,
					Recipient:        "alpha@example.com",
					Message:          "Cancelled",
					Status:           model.StatusCancelled,
					CreatedAt:        time.Now().UTC(),
					UpdatedAt:        time.Now().UTC(),
				},
				cancelError: testCase.serviceError,
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
			server := &notificationServiceServer{notificationService: stubService, logger: logger}

			response, err := server.CancelNotification(context.Background(), &grpcapi.CancelNotificationRequest{NotificationId: testCase.notificationID})

			if testCase.expectStatusError {
				if err == nil {
					t.Fatalf("expected error")
				}
				if status.Code(err) != testCase.expectedStatusCode {
					t.Fatalf("expected code %v, got %v", testCase.expectedStatusCode, status.Code(err))
				}
				stubService.mutex.Lock()
				if len(stubService.cancelCalls) != 0 {
					stubService.mutex.Unlock()
					t.Fatalf("unexpected service call recorded")
				}
				stubService.mutex.Unlock()
				return
			}

			if testCase.serviceError != nil {
				if !errors.Is(err, testCase.serviceError) {
					t.Fatalf("expected propagated error %v, got %v", testCase.serviceError, err)
				}
				stubService.mutex.Lock()
				if len(stubService.cancelCalls) != 1 {
					count := len(stubService.cancelCalls)
					stubService.mutex.Unlock()
					t.Fatalf("expected one cancel call, got %d", count)
				}
				if stubService.cancelCalls[0] != testCase.notificationID {
					recordedID := stubService.cancelCalls[0]
					stubService.mutex.Unlock()
					t.Fatalf("unexpected cancel id %s", recordedID)
				}
				stubService.mutex.Unlock()
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if response.NotificationId != "notif-1" {
				t.Fatalf("unexpected notification id %s", response.NotificationId)
			}
			stubService.mutex.Lock()
			defer stubService.mutex.Unlock()
			if len(stubService.cancelCalls) != 1 {
				t.Fatalf("expected one cancel call, got %d", len(stubService.cancelCalls))
			}
			if stubService.cancelCalls[0] != "notif-1" {
				t.Fatalf("unexpected cancel id %s", stubService.cancelCalls[0])
			}
		})
	}
}

func TestBuildAuthInterceptorRejectsUnauthorizedRequests(t *testing.T) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	interceptor := buildAuthInterceptor(logger, "expected-token")
	expectedResponse := "ok"
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return expectedResponse, nil
	}

	testCases := []struct {
		name            string
		ctx             context.Context
		expectedMessage string
	}{
		{
			name:            "MissingMetadata",
			ctx:             context.Background(),
			expectedMessage: "missing metadata",
		},
		{
			name:            "MissingAuthorizationHeader",
			ctx:             metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{})),
			expectedMessage: "missing authorization header",
		},
		{
			name: "InvalidAuthorizationFormat",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.New(map[string]string{"authorization": "Token value"}),
			),
			expectedMessage: "invalid authorization header",
		},
		{
			name: "InvalidToken",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.New(map[string]string{"authorization": "Bearer other-token"}),
			),
			expectedMessage: "invalid token",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			_, err := interceptor(testCase.ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/notifications.test"}, handler)
			if err == nil {
				t.Fatalf("expected error")
			}
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("expected unauthenticated, got %v", status.Code(err))
			}
			if status.Convert(err).Message() != testCase.expectedMessage {
				t.Fatalf("unexpected message %q", status.Convert(err).Message())
			}
		})
	}

	validCtx := metadata.NewIncomingContext(
		context.Background(),
		metadata.New(map[string]string{"authorization": "Bearer expected-token"}),
	)
	response, err := interceptor(validCtx, nil, &grpc.UnaryServerInfo{FullMethod: "/notifications.test"}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != expectedResponse {
		t.Fatalf("unexpected response %v", response)
	}
}

func TestBuildAuthInterceptorDoesNotLogTokenValue(t *testing.T) {
	t.Helper()

	var buffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{}))
	interceptor := buildAuthInterceptor(logger, "super-secret-token")

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.New(map[string]string{"authorization": "Bearer another-token"}),
	)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/notifications.test"}, func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated, got %v", status.Code(err))
	}

	logOutput := buffer.String()
	if !strings.Contains(logOutput, "Invalid token provided") {
		t.Fatalf("expected log message to mention invalid token, got %q", logOutput)
	}
	if strings.Contains(logOutput, "super-secret-token") {
		t.Fatalf("log output should not contain the expected token: %q", logOutput)
	}
	if strings.Contains(logOutput, "another-token") {
		t.Fatalf("log output should not contain the provided token value: %q", logOutput)
	}
}

func startTestNotificationServer(t *testing.T, svc service.NotificationService, token string) (string, func()) {
	t.Helper()

	listener, listenError := net.Listen("tcp", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("listen error: %v", listenError)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(buildAuthInterceptor(logger, token)))
	grpcapi.RegisterNotificationServiceServer(grpcServer, &notificationServiceServer{
		notificationService: svc,
		logger:              logger,
	})

	go func() {
		_ = grpcServer.Serve(listener)
	}()

	shutdown := func() {
		grpcServer.Stop()
		listener.Close()
	}

	return listener.Addr().String(), shutdown
}

type stubNotificationService struct {
	mutex              sync.Mutex
	sendCalls          []model.NotificationRequest
	statusCalls        []string
	sendResponse       model.NotificationResponse
	statusResponses    []model.NotificationResponse
	listCalls          []model.NotificationListFilters
	listResponses      []model.NotificationResponse
	rescheduleCalls    []rescheduleInvocation
	rescheduleResponse model.NotificationResponse
	rescheduleError    error
	cancelCalls        []string
	cancelResponse     model.NotificationResponse
	cancelError        error
}

func (stub *stubNotificationService) SendNotification(ctx context.Context, request model.NotificationRequest) (model.NotificationResponse, error) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.sendCalls = append(stub.sendCalls, request)
	return stub.sendResponse, nil
}

func (stub *stubNotificationService) GetNotificationStatus(ctx context.Context, notificationID string) (model.NotificationResponse, error) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.statusCalls = append(stub.statusCalls, notificationID)
	if len(stub.statusResponses) == 0 {
		return stub.sendResponse, nil
	}
	response := stub.statusResponses[0]
	stub.statusResponses = stub.statusResponses[1:]
	return response, nil
}

func (stub *stubNotificationService) ListNotifications(ctx context.Context, filters model.NotificationListFilters) ([]model.NotificationResponse, error) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.listCalls = append(stub.listCalls, filters)
	return stub.listResponses, nil
}

func (stub *stubNotificationService) RescheduleNotification(ctx context.Context, notificationID string, scheduledFor time.Time) (model.NotificationResponse, error) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.rescheduleCalls = append(stub.rescheduleCalls, rescheduleInvocation{notificationID: notificationID, scheduledFor: scheduledFor})
	if stub.rescheduleError != nil {
		return model.NotificationResponse{}, stub.rescheduleError
	}
	return stub.rescheduleResponse, nil
}

func (stub *stubNotificationService) CancelNotification(ctx context.Context, notificationID string) (model.NotificationResponse, error) {
	stub.mutex.Lock()
	defer stub.mutex.Unlock()
	stub.cancelCalls = append(stub.cancelCalls, notificationID)
	if stub.cancelError != nil {
		return model.NotificationResponse{}, stub.cancelError
	}
	return stub.cancelResponse, nil
}

func (stub *stubNotificationService) StartRetryWorker(ctx context.Context) {}

type rescheduleInvocation struct {
	notificationID string
	scheduledFor   time.Time
}
