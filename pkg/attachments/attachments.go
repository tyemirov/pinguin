package attachments

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tyemirov/pinguin/pkg/grpcapi"
)

const defaultContentType = "application/octet-stream"

// Load reads the provided attachment specifiers into gRPC attachment messages.
// Each specifier has the form "path" or "path::content-type".
func Load(inputs []string) ([]*grpcapi.EmailAttachment, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	result := make([]*grpcapi.EmailAttachment, 0, len(inputs))
	for _, raw := range inputs {
		path, explicitType := splitInput(raw)
		if path == "" {
			return nil, fmt.Errorf("attachment path is required")
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read attachment %q: %w", path, readErr)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("attachment %q is empty", path)
		}

		contentType := explicitType
		if contentType == "" {
			contentType = inferContentType(path, data)
		}

		result = append(result, &grpcapi.EmailAttachment{
			Filename:    filepath.Base(path),
			ContentType: contentType,
			Data:        data,
		})
	}
	return result, nil
}

func splitInput(input string) (string, string) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", ""
	}
	parts := strings.SplitN(trimmed, "::", 2)
	path := strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		return path, strings.TrimSpace(parts[1])
	}
	return path, ""
}

func inferContentType(path string, data []byte) string {
	if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
		if detected := mime.TypeByExtension(ext); detected != "" {
			return detected
		}
	}
	if len(data) > 0 {
		if sniffed := http.DetectContentType(data); sniffed != "" {
			return sniffed
		}
	}
	return defaultContentType
}
