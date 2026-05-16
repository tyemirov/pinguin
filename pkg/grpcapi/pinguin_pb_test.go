package grpcapi

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEnumHelpers(t *testing.T) {
	t.Helper()
	if NotificationType_EMAIL.String() != "EMAIL" {
		t.Fatalf("unexpected notification type string")
	}
	if NotificationType_SMS.Enum() == nil || *NotificationType_SMS.Enum() != NotificationType_SMS {
		t.Fatalf("Enum helper did not round-trip value")
	}
	if NotificationType_EMAIL.Descriptor() == nil || NotificationType_EMAIL.Type() == nil {
		t.Fatalf("Descriptor or Type missing for notification type")
	}
	if NotificationType_SMS.Number() != 1 {
		t.Fatalf("unexpected number for SMS notification type")
	}
	if _, idx := NotificationType_EMAIL.EnumDescriptor(); idx[0] != 0 {
		t.Fatalf("unexpected EnumDescriptor index for notification type")
	}

	if Status_SENT.String() != "SENT" {
		t.Fatalf("unexpected status string for SENT")
	}
	if Status_SENT.Enum() == nil || *Status_SENT.Enum() != Status_SENT {
		t.Fatalf("Enum helper did not round-trip status")
	}
	if Status_CANCELLED.Descriptor() == nil || Status_CANCELLED.Type() == nil {
		t.Fatalf("Descriptor or Type missing for status")
	}
	if Status_ERRORED.Number() != 5 {
		t.Fatalf("unexpected number for ERRORED status")
	}
	if _, idx := Status_QUEUED.EnumDescriptor(); idx[0] != 1 {
		t.Fatalf("unexpected EnumDescriptor index for status")
	}
}

func TestEmailAttachmentAccessors(t *testing.T) {
	t.Helper()
	attachment := &EmailAttachment{
		Filename:    "report.pdf",
		ContentType: "application/pdf",
		Data:        []byte("data"),
	}
	if attachment.String() == "" {
		t.Fatalf("String should return content")
	}
	if attachment.GetFilename() != "report.pdf" {
		t.Fatalf("GetFilename returned %q", attachment.GetFilename())
	}
	if attachment.GetContentType() != "application/pdf" {
		t.Fatalf("GetContentType returned %q", attachment.GetContentType())
	}
	if string(attachment.GetData()) != "data" {
		t.Fatalf("GetData returned %q", string(attachment.GetData()))
	}
	if attachment.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("ProtoReflect descriptor missing")
	}
	attachment.ProtoMessage()
	if desc, _ := attachment.Descriptor(); len(desc) == 0 {
		t.Fatalf("descriptor bytes should not be empty")
	}
	attachment.Reset()
	if attachment.GetFilename() != "" {
		t.Fatalf("Reset did not clear filename")
	}
}

func TestNotificationRequestMessage(t *testing.T) {
	t.Helper()
	req := &NotificationRequest{
		NotificationType: NotificationType_EMAIL,
		Recipient:        "user@example.com",
		Subject:          "Hello",
		Message:          "Body",
		ScheduledTime:    timestamppb.New(time.Unix(10, 0)),
		Attachments: []*EmailAttachment{
			{Filename: "a.txt"},
		},
		TenantId: "tenant",
	}
	if req.String() == "" {
		t.Fatalf("String should not be empty")
	}
	if req.GetRecipient() != "user@example.com" {
		t.Fatalf("unexpected recipient %q", req.GetRecipient())
	}
	if req.GetSubject() != "Hello" {
		t.Fatalf("unexpected subject %q", req.GetSubject())
	}
	if req.GetMessage() != "Body" {
		t.Fatalf("unexpected message %q", req.GetMessage())
	}
	if req.GetScheduledTime().GetSeconds() != 10 {
		t.Fatalf("unexpected scheduled time")
	}
	if len(req.GetAttachments()) != 1 {
		t.Fatalf("expected one attachment")
	}
	if req.GetTenantId() != "tenant" {
		t.Fatalf("unexpected tenant id")
	}
	if req.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("missing descriptor")
	}
	req.ProtoMessage()
	if desc, _ := req.Descriptor(); len(desc) == 0 {
		t.Fatalf("descriptor should be populated")
	}
	if req.GetNotificationType() != NotificationType_EMAIL {
		t.Fatalf("unexpected notification type default")
	}
	req.Reset()
	if req.GetRecipient() != "" || req.GetAttachments() != nil {
		t.Fatalf("Reset did not clear fields")
	}
}

func TestNotificationResponseMessage(t *testing.T) {
	t.Helper()
	resp := &NotificationResponse{
		NotificationId:    "notif-1",
		NotificationType:  NotificationType_SMS,
		Recipient:         "+15551234",
		Subject:           "ignored",
		Message:           "msg",
		Status:            Status_SENT,
		ProviderMessageId: "provider",
		RetryCount:        3,
		CreatedAt:         "created",
		UpdatedAt:         "updated",
		ScheduledTime:     timestamppb.New(time.Unix(100, 0)),
		Attachments: []*EmailAttachment{
			{Filename: "b.txt"},
		},
		TenantId: "tenant",
	}
	if resp.String() == "" {
		t.Fatalf("String should not be empty")
	}
	if resp.GetNotificationId() != "notif-1" {
		t.Fatalf("unexpected id %q", resp.GetNotificationId())
	}
	if resp.GetRecipient() != "+15551234" {
		t.Fatalf("unexpected recipient getter")
	}
	if resp.GetSubject() != "ignored" {
		t.Fatalf("unexpected subject getter")
	}
	if resp.GetMessage() != "msg" {
		t.Fatalf("unexpected message getter")
	}
	if resp.GetNotificationType() != NotificationType_SMS {
		t.Fatalf("unexpected type %v", resp.GetNotificationType())
	}
	if resp.GetStatus() != Status_SENT {
		t.Fatalf("unexpected status")
	}
	if resp.GetProviderMessageId() != "provider" {
		t.Fatalf("unexpected provider id")
	}
	if resp.GetRetryCount() != 3 {
		t.Fatalf("unexpected retry count")
	}
	if resp.GetCreatedAt() != "created" || resp.GetUpdatedAt() != "updated" {
		t.Fatalf("unexpected timestamp strings")
	}
	if resp.GetScheduledTime().GetSeconds() != 100 {
		t.Fatalf("unexpected scheduled time")
	}
	if len(resp.GetAttachments()) != 1 {
		t.Fatalf("unexpected attachments")
	}
	if resp.GetTenantId() != "tenant" {
		t.Fatalf("unexpected tenant id")
	}
	resp.ProtoMessage()
	if desc, _ := resp.Descriptor(); len(desc) == 0 {
		t.Fatalf("descriptor should not be empty")
	}
	resp.Reset()
	if resp.GetNotificationId() != "" {
		t.Fatalf("Reset did not clear notification id")
	}
}

func TestUtilityMessages(t *testing.T) {
	t.Helper()
	getReq := &GetNotificationStatusRequest{NotificationId: "id"}
	if getReq.GetNotificationId() != "id" {
		t.Fatalf("unexpected GetNotificationStatusRequest ID")
	}
	getReq.ProtoMessage()
	if desc, _ := getReq.Descriptor(); len(desc) == 0 {
		t.Fatalf("descriptor missing")
	}
	getReq.Reset()
	if getReq.GetNotificationId() != "" {
		t.Fatalf("Reset should clear id")
	}

	listReq := &ListNotificationsRequest{Statuses: []Status{Status_QUEUED}}
	if len(listReq.GetStatuses()) != 1 {
		t.Fatalf("unexpected statuses length")
	}
	listReq.ProtoMessage()
	if desc, _ := listReq.Descriptor(); len(desc) == 0 {
		t.Fatalf("descriptor missing")
	}
	listResp := &ListNotificationsResponse{
		Notifications: []*NotificationResponse{{NotificationId: "id"}},
	}
	if len(listResp.GetNotifications()) != 1 {
		t.Fatalf("unexpected notifications length")
	}
	listResp.ProtoMessage()
	if desc, _ := listResp.Descriptor(); len(desc) == 0 {
		t.Fatalf("descriptor missing")
	}
	re := &RescheduleNotificationRequest{
		NotificationId: "nid",
		ScheduledTime:  timestamppb.New(time.Unix(5, 0)),
	}
	if re.GetNotificationId() != "nid" || re.GetScheduledTime().GetSeconds() != 5 {
		t.Fatalf("unexpected reschedule request fields")
	}
	re.ProtoMessage()
	if desc, _ := re.Descriptor(); len(desc) == 0 {
		t.Fatalf("descriptor missing")
	}
	cancel := &CancelNotificationRequest{NotificationId: "nid", TenantId: "tenant"}
	if cancel.GetNotificationId() != "nid" || cancel.GetTenantId() != "tenant" {
		t.Fatalf("unexpected cancel id")
	}
	if cancel.String() == "" {
		t.Fatalf("expected cancel string")
	}
	cancel.ProtoMessage()
	if desc, _ := cancel.Descriptor(); len(desc) == 0 {
		t.Fatalf("descriptor missing")
	}
	cancel.Reset()
	if cancel.GetNotificationId() != "" {
		t.Fatalf("cancel reset should clear id")
	}
}

func TestRawDescriptorFunctions(t *testing.T) {
	t.Helper()
	if len(file_pkg_proto_pinguin_proto_rawDescGZIP()) == 0 {
		t.Fatalf("raw descriptor should not be empty")
	}
	file_pkg_proto_pinguin_proto_init()
}

func TestNilGeneratedMessageAccessors(t *testing.T) {
	var attachment *EmailAttachment
	if attachment.GetFilename() != "" || attachment.GetContentType() != "" || attachment.GetData() != nil {
		t.Fatalf("unexpected nil attachment getters")
	}
	if attachment.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("nil attachment descriptor missing")
	}

	var request *NotificationRequest
	if request.GetNotificationType() != NotificationType_EMAIL || request.GetRecipient() != "" || request.GetSubject() != "" || request.GetMessage() != "" {
		t.Fatalf("unexpected nil request scalar getters")
	}
	if request.GetScheduledTime() != nil || request.GetAttachments() != nil || request.GetTenantId() != "" {
		t.Fatalf("unexpected nil request reference getters")
	}
	if request.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("nil request descriptor missing")
	}

	var response *NotificationResponse
	if response.GetNotificationId() != "" || response.GetNotificationType() != NotificationType_EMAIL || response.GetStatus() != Status_QUEUED {
		t.Fatalf("unexpected nil response primary getters")
	}
	if response.GetRecipient() != "" || response.GetSubject() != "" || response.GetMessage() != "" || response.GetProviderMessageId() != "" {
		t.Fatalf("unexpected nil response string getters")
	}
	if response.GetRetryCount() != 0 || response.GetCreatedAt() != "" || response.GetUpdatedAt() != "" {
		t.Fatalf("unexpected nil response count/time getters")
	}
	if response.GetScheduledTime() != nil || response.GetAttachments() != nil || response.GetTenantId() != "" {
		t.Fatalf("unexpected nil response reference getters")
	}
	if response.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("nil response descriptor missing")
	}

	var statusRequest *GetNotificationStatusRequest
	if statusRequest.GetNotificationId() != "" || statusRequest.GetTenantId() != "" {
		t.Fatalf("unexpected nil status request getters")
	}
	if statusRequest.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("nil status request descriptor missing")
	}

	var listRequest *ListNotificationsRequest
	if listRequest.GetStatuses() != nil || listRequest.GetTenantId() != "" {
		t.Fatalf("unexpected nil list request getters")
	}
	if listRequest.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("nil list request descriptor missing")
	}

	var listResponse *ListNotificationsResponse
	if listResponse.GetNotifications() != nil {
		t.Fatalf("unexpected nil list response getters")
	}
	if listResponse.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("nil list response descriptor missing")
	}

	var reschedule *RescheduleNotificationRequest
	if reschedule.GetNotificationId() != "" || reschedule.GetScheduledTime() != nil || reschedule.GetTenantId() != "" {
		t.Fatalf("unexpected nil reschedule getters")
	}
	if reschedule.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("nil reschedule descriptor missing")
	}

	var cancel *CancelNotificationRequest
	if cancel.GetNotificationId() != "" || cancel.GetTenantId() != "" {
		t.Fatalf("unexpected nil cancel getters")
	}
	if cancel.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("nil cancel descriptor missing")
	}
}

func TestUtilityMessagesFullMethods(t *testing.T) {
	statusRequest := &GetNotificationStatusRequest{NotificationId: "id", TenantId: "tenant"}
	if statusRequest.String() == "" || statusRequest.GetTenantId() != "tenant" {
		t.Fatalf("unexpected status request")
	}
	if statusRequest.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("missing status request descriptor")
	}

	listRequest := &ListNotificationsRequest{Statuses: []Status{Status_SENT}, TenantId: "tenant"}
	if listRequest.String() == "" || listRequest.GetTenantId() != "tenant" {
		t.Fatalf("unexpected list request")
	}
	if listRequest.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("missing list request descriptor")
	}
	listRequest.Reset()
	if listRequest.GetStatuses() != nil {
		t.Fatalf("list reset should clear statuses")
	}

	listResponse := &ListNotificationsResponse{Notifications: []*NotificationResponse{{NotificationId: "id"}}}
	if listResponse.String() == "" {
		t.Fatalf("unexpected list response string")
	}
	if listResponse.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("missing list response descriptor")
	}
	listResponse.Reset()
	if listResponse.GetNotifications() != nil {
		t.Fatalf("list response reset should clear notifications")
	}

	reschedule := &RescheduleNotificationRequest{
		NotificationId: "id",
		ScheduledTime:  timestamppb.New(time.Unix(10, 0)),
		TenantId:       "tenant",
	}
	if reschedule.String() == "" || reschedule.GetTenantId() != "tenant" {
		t.Fatalf("unexpected reschedule request")
	}
	if reschedule.ProtoReflect().Descriptor().FullName() == "" {
		t.Fatalf("missing reschedule descriptor")
	}
	reschedule.Reset()
	if reschedule.GetNotificationId() != "" {
		t.Fatalf("reschedule reset should clear id")
	}
}
