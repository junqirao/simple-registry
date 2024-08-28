package simple_registry

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

type (
	// WatchHandler watch registry
	WatchHandler func(ctx context.Context, e Event)
	// KV kv
	KV struct {
		Key   string
		Value *g.Var
	}
	// Event of database key value changes
	Event struct {
		KV
		Type EventType
	}
	// Database abstract key-value database ability
	Database interface {
		// Get values from database by key
		Get(ctx context.Context, key string) (v []*KV, err error)
		// GetPrefix values from database by prefixed key
		GetPrefix(ctx context.Context, key string) (v []*KV, err error)
		// Set value to database
		Set(ctx context.Context, key string, value interface{}, ttl int64, keepalive ...bool) (err error)
		// Delete value from database
		Delete(ctx context.Context, key string) (err error)
		// Watch database changes
		Watch(ctx context.Context, key string, handler WatchHandler) (err error)
	}
)
