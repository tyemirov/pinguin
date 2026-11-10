package grpcutil

// MaxMessageSizeBytes defines the shared send/receive limit used by both the
// Pingwin server and clients to accommodate attachment-heavy payloads.
const MaxMessageSizeBytes = 32 * 1024 * 1024
