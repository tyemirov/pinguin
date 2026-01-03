package tenant

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidViewScope reports invalid tenant view scopes.
var ErrInvalidViewScope = errors.New("tenant: invalid view scope")

// DefaultViewScope returns the default scope when none is configured.
func DefaultViewScope() ViewScope {
	return ViewScopeGlobal
}

// ParseViewScope validates and normalizes a configured scope.
func ParseViewScope(rawValue string) (ViewScope, error) {
	normalized := strings.ToLower(strings.TrimSpace(rawValue))
	if normalized == "" {
		return "", fmt.Errorf("%w: empty view scope", ErrInvalidViewScope)
	}
	switch ViewScope(normalized) {
	case ViewScopeGlobal:
		return ViewScopeGlobal, nil
	case ViewScopeTenant:
		return ViewScopeTenant, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidViewScope, normalized)
	}
}
