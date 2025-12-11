package tenant

import "context"

type contextKey string

const tenantContextKey contextKey = "tenant.runtime"

// WithRuntime stores runtime config in context.
func WithRuntime(ctx context.Context, cfg RuntimeConfig) context.Context {
	return context.WithValue(ctx, tenantContextKey, cfg)
}

// RuntimeFromContext retrieves runtime config.
func RuntimeFromContext(ctx context.Context) (RuntimeConfig, bool) {
	cfg, ok := ctx.Value(tenantContextKey).(RuntimeConfig)
	return cfg, ok
}
