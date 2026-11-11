package grpcapi

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeConn struct {
	lastMethod string
	err        error
}

func (c *fakeConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	c.lastMethod = method
	if c.err != nil {
		return c.err
	}
	switch out := reply.(type) {
	case *NotificationResponse:
		out.NotificationId = "notif"
		out.Status = Status_SENT
	case *ListNotificationsResponse:
		out.Notifications = []*NotificationResponse{{NotificationId: "notif"}}
	default:
	}
	return nil
}

func (c *fakeConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, fmt.Errorf("streaming not supported")
}

func TestNotificationServiceClientCoverage(t *testing.T) {
	t.Helper()
	client := NewNotificationServiceClient(&fakeConn{})
	ctx := context.Background()
	if _, err := client.SendNotification(ctx, &NotificationRequest{}); err != nil {
		t.Fatalf("SendNotification error: %v", err)
	}
	if _, err := client.GetNotificationStatus(ctx, &GetNotificationStatusRequest{}); err != nil {
		t.Fatalf("GetNotificationStatus error: %v", err)
	}
	if _, err := client.ListNotifications(ctx, &ListNotificationsRequest{}); err != nil {
		t.Fatalf("ListNotifications error: %v", err)
	}
	if _, err := client.RescheduleNotification(ctx, &RescheduleNotificationRequest{}); err != nil {
		t.Fatalf("RescheduleNotification error: %v", err)
	}
	if _, err := client.CancelNotification(ctx, &CancelNotificationRequest{}); err != nil {
		t.Fatalf("CancelNotification error: %v", err)
	}
}

func TestNotificationServiceClientErrors(t *testing.T) {
	t.Helper()
	expectedErr := errors.New("rpc failed")
	client := NewNotificationServiceClient(&fakeConn{err: expectedErr})
	ctx := context.Background()
	calls := []func() error{
		func() error {
			_, err := client.SendNotification(ctx, &NotificationRequest{})
			return err
		},
		func() error {
			_, err := client.GetNotificationStatus(ctx, &GetNotificationStatusRequest{})
			return err
		},
		func() error {
			_, err := client.ListNotifications(ctx, &ListNotificationsRequest{})
			return err
		},
		func() error {
			_, err := client.RescheduleNotification(ctx, &RescheduleNotificationRequest{})
			return err
		},
		func() error {
			_, err := client.CancelNotification(ctx, &CancelNotificationRequest{})
			return err
		},
	}
	for _, call := range calls {
		if err := call(); !errors.Is(err, expectedErr) {
			t.Fatalf("expected rpc error, got %v", err)
		}
	}
}

type coverageServer struct {
	UnimplementedNotificationServiceServer
	sendCalled       bool
	statusCalled     bool
	listCalled       bool
	rescheduleCalled bool
	cancelCalled     bool
}

func (s *coverageServer) SendNotification(context.Context, *NotificationRequest) (*NotificationResponse, error) {
	s.sendCalled = true
	return &NotificationResponse{NotificationId: "id"}, nil
}

func (s *coverageServer) GetNotificationStatus(context.Context, *GetNotificationStatusRequest) (*NotificationResponse, error) {
	s.statusCalled = true
	return &NotificationResponse{NotificationId: "id"}, nil
}

func (s *coverageServer) ListNotifications(context.Context, *ListNotificationsRequest) (*ListNotificationsResponse, error) {
	s.listCalled = true
	return &ListNotificationsResponse{}, nil
}

func (s *coverageServer) RescheduleNotification(context.Context, *RescheduleNotificationRequest) (*NotificationResponse, error) {
	s.rescheduleCalled = true
	return &NotificationResponse{NotificationId: "id"}, nil
}

func (s *coverageServer) CancelNotification(context.Context, *CancelNotificationRequest) (*NotificationResponse, error) {
	s.cancelCalled = true
	return &NotificationResponse{NotificationId: "id"}, nil
}

func TestNotificationServiceServerHandlers(t *testing.T) {
	t.Helper()
	server := &coverageServer{}
	ctx := context.Background()

	decoder := func(interface{}) error { return nil }

	if _, err := _NotificationService_SendNotification_Handler(server, ctx, decoder, nil); err != nil {
		t.Fatalf("Send handler error: %v", err)
	}
	if _, err := _NotificationService_GetNotificationStatus_Handler(server, ctx, decoder, nil); err != nil {
		t.Fatalf("Status handler error: %v", err)
	}
	if _, err := _NotificationService_ListNotifications_Handler(server, ctx, decoder, nil); err != nil {
		t.Fatalf("List handler error: %v", err)
	}
	if _, err := _NotificationService_RescheduleNotification_Handler(server, ctx, decoder, nil); err != nil {
		t.Fatalf("Reschedule handler error: %v", err)
	}
	if _, err := _NotificationService_CancelNotification_Handler(server, ctx, decoder, nil); err != nil {
		t.Fatalf("Cancel handler error: %v", err)
	}

	if !(server.sendCalled && server.statusCalled && server.listCalled && server.rescheduleCalled && server.cancelCalled) {
		t.Fatalf("expected all server methods to be called")
	}
}

func TestNotificationServiceServerHandlersWithInterceptorsAndDecoderErrors(t *testing.T) {
	server := &coverageServer{}
	ctx := context.Background()
	decoderErr := errors.New("decode failed")
	failDecoder := func(interface{}) error { return decoderErr }
	passDecoder := func(interface{}) error { return nil }
	interceptorCalled := 0
	interceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		interceptorCalled++
		if info.FullMethod == "" {
			t.Fatalf("expected full method")
		}
		return handler(ctx, req)
	}

	handlerCalls := []struct {
		name string
		call func(func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error)
	}{
		{name: "send", call: func(dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
			return _NotificationService_SendNotification_Handler(server, ctx, dec, interceptor)
		}},
		{name: "status", call: func(dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
			return _NotificationService_GetNotificationStatus_Handler(server, ctx, dec, interceptor)
		}},
		{name: "list", call: func(dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
			return _NotificationService_ListNotifications_Handler(server, ctx, dec, interceptor)
		}},
		{name: "reschedule", call: func(dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
			return _NotificationService_RescheduleNotification_Handler(server, ctx, dec, interceptor)
		}},
		{name: "cancel", call: func(dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
			return _NotificationService_CancelNotification_Handler(server, ctx, dec, interceptor)
		}},
	}
	for _, handlerCall := range handlerCalls {
		handlerCall := handlerCall
		t.Run(handlerCall.name+" decoder error", func(t *testing.T) {
			_, err := handlerCall.call(failDecoder, nil)
			if !errors.Is(err, decoderErr) {
				t.Fatalf("expected decoder error, got %v", err)
			}
		})
		t.Run(handlerCall.name+" interceptor", func(t *testing.T) {
			if _, err := handlerCall.call(passDecoder, interceptor); err != nil {
				t.Fatalf("handler with interceptor: %v", err)
			}
		})
	}
	if interceptorCalled != len(handlerCalls) {
		t.Fatalf("expected %d interceptor calls, got %d", len(handlerCalls), interceptorCalled)
	}
}

func TestRegisterNotificationServiceServer(t *testing.T) {
	t.Helper()
	grpcServer := grpc.NewServer()
	RegisterNotificationServiceServer(grpcServer, &coverageServer{})
	grpcServer.Stop()
}

func TestUnimplementedServerResponses(t *testing.T) {
	t.Helper()
	server := UnimplementedNotificationServiceServer{}
	ctx := context.Background()
	assertUnimplemented := func(err error) {
		if err == nil || status.Code(err) != codes.Unimplemented {
			t.Fatalf("expected unimplemented error, got %v", err)
		}
	}
	_, err := server.SendNotification(ctx, &NotificationRequest{})
	assertUnimplemented(err)
	_, err = server.GetNotificationStatus(ctx, &GetNotificationStatusRequest{})
	assertUnimplemented(err)
	_, err = server.ListNotifications(ctx, &ListNotificationsRequest{})
	assertUnimplemented(err)
	_, err = server.RescheduleNotification(ctx, &RescheduleNotificationRequest{})
	assertUnimplemented(err)
	_, err = server.CancelNotification(ctx, &CancelNotificationRequest{})
	assertUnimplemented(err)
	server.mustEmbedUnimplementedNotificationServiceServer()
	server.testEmbeddedByValue()
}
