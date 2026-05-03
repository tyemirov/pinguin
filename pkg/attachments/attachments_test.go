package attachments

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitInput(t *testing.T) {
	t.Parallel()

	path, contentType := splitInput(" /tmp/file.txt :: text/plain ")
	if path != "/tmp/file.txt" {
		t.Fatalf("unexpected path %q", path)
	}
	if contentType != "text/plain" {
		t.Fatalf("unexpected content type %q", contentType)
	}

	path, contentType = splitInput("file.bin")
	if path != "file.bin" || contentType != "" {
		t.Fatalf("unexpected result %q %q", path, contentType)
	}
}

func TestLoadInfersContentType(t *testing.T) {
	t.Parallel()

	tempFile := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(tempFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	attachments, err := Load([]string{tempFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected one attachment")
	}
	if attachments[0].ContentType == "" {
		t.Fatalf("expected inferred content type")
	}
}

func TestLoadHandlesExplicitTypeAndErrors(t *testing.T) {
	t.Parallel()

	if attachments, err := Load(nil); err != nil || attachments != nil {
		t.Fatalf("expected nil attachments without input, got attachments=%v err=%v", attachments, err)
	}

	tempDir := t.TempDir()
	payloadPath := filepath.Join(tempDir, "payload.bin")
	if err := os.WriteFile(payloadPath, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	attachments, err := Load([]string{payloadPath + "::application/x-test"})
	if err != nil {
		t.Fatalf("load explicit content type: %v", err)
	}
	if len(attachments) != 1 || attachments[0].ContentType != "application/x-test" || attachments[0].Filename != "payload.bin" {
		t.Fatalf("unexpected attachment %+v", attachments)
	}

	_, err = Load([]string{filepath.Join(tempDir, "missing.txt")})
	if err == nil {
		t.Fatalf("expected missing file error")
	}

	emptyPath := filepath.Join(tempDir, "empty.txt")
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	_, err = Load([]string{emptyPath})
	if err == nil {
		t.Fatalf("expected empty attachment error")
	}
}

func TestLoadRequiresPath(t *testing.T) {
	t.Parallel()

	_, err := Load([]string{"   "})
	if err == nil {
		t.Fatalf("expected error for missing path")
	}
}

func TestInferContentTypeFallbacks(t *testing.T) {
	t.Parallel()

	if inferred := inferContentType("payload.unknown", []byte("hello")); inferred == defaultContentType {
		t.Fatalf("expected sniffed content type, got default")
	}
	if inferred := inferContentType("payload.unknown", nil); inferred != defaultContentType {
		t.Fatalf("expected default content type, got %q", inferred)
	}
}
