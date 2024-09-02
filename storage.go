package simple_registry

import (
	"context"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
)

type (
	// Storage interface
	Storage interface {
		Get(ctx context.Context, key ...string) (v []*KV, err error)
		Set(ctx context.Context, key string, value interface{}) (err error)
		Delete(ctx context.Context, key string) (err error)
	}
	// StorageEventHandler process storage event
	StorageEventHandler func(t EventType, key string, value interface{})
	storages            struct {
		ctx context.Context
		cfg Config
		db  Database
		m   sync.Map // key: (name)string, value: Storage
		evs sync.Map // key: (name)string, value: StorageEventHandler
	}
)

func newStorages(ctx context.Context, cfg Config, db Database) *storages {
	sto := &storages{ctx: ctx, cfg: cfg, db: db}
	// watch and update caches event bus
	sto.watchAndUpdateCaches(ctx)
	return sto
}

// GetStorage or create Storage instance
func (s *storages) GetStorage(name string, uncached ...bool) Storage {
	var cs *cachedStorage
	v, ok := s.m.Load(name)
	if ok {
		cs = v.(*cachedStorage)
	}

	if cs == nil {
		cs = newCachedStorage(s.ctx, newStorage(s.cfg.getStoragePrefix(), name, s.db, s.cfg.Storage))
		s.m.Store(name, cs)
	}

	if len(uncached) > 0 && uncached[0] {
		return cs.db
	}
	return cs
}

func (s *storages) watchAndUpdateCaches(ctx context.Context) {
	pfx := s.cfg.getStoragePrefix()
	err := s.db.Watch(ctx, pfx, func(ctx context.Context, e Event) {
		pos := strings.Split(strings.TrimPrefix(e.Key, pfx), s.cfg.Storage.Separator)
		if len(pos) == 0 {
			return
		}
		name := pos[0]
		key := strings.Join(pos[1:], s.cfg.Storage.Separator)

		// internal event
		if sto, ok := s.m.Load(name); ok {
			sto.(*cachedStorage).handleEvent(e.Type, key, e.Value)
		}

		// push to event handler
		if ev, ok := s.evs.Load(name); ok {
			ev.(StorageEventHandler)(e.Type, key, e.Value)
		}
	})
	if err != nil {
		g.Log().Errorf(ctx, "failed to watch and update caches at storage: %s", err.Error())
	}
}

func (s *storages) SetEventHandler(name string, handler StorageEventHandler) {
	s.evs.Store(name, handler)
}
