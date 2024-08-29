package simple_registry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
)

var (
	// Registry Global instance of registry, use it after Init
	Registry Interface
	// currentInstance created at Init
	currentInstance      *Instance
	onceLoad             = sync.Once{}
	ErrAlreadyRegistered = errors.New("already registered")
)

// event type define
const (
	EventTypeCreate EventType = "create"
	EventTypeUpdate EventType = "update"
	EventTypeDelete EventType = "delete"
)

type (
	// Interface abstracts registry
	Interface interface {
		// register currentInstance
		register(ctx context.Context, ins *Instance) (err error)
		// Deregister deregister currentInstance
		Deregister(ctx context.Context) (err error)
		// GetService by service name
		GetService(ctx context.Context, serviceName string) (service *Service, err error)
		// GetServices of all
		GetServices(ctx context.Context) (services map[string]*Service, err error)
		// RegisterEventHandler register event handler
		RegisterEventHandler(handler EventHandler)
	}

	// EventType of instance change
	EventType string
	// EventHandler of instance change
	EventHandler func(i *Instance, e EventType)
	// eventWrapper ...
	eventWrapper struct {
		handler EventHandler
		next    *eventWrapper
	}
)

// Init registry module with config and sync services info from database and build local caches.
// if *Instance is provided will be register automatically.
// if context is done, watch loop will stop and local cache won't be updated anymore.
func Init(ctx context.Context, config Config, ins ...*Instance) (err error) {
	onceLoad.Do(func() {
		// create registry instance
		if Registry, err = newRegistry(ctx, config); err != nil {
			return
		}
		// collect instance info and register
		if len(ins) > 0 && ins[0] != nil {
			err = Registry.register(ctx, ins[0].fillInfo().clone())
		}
	})
	return
}

type registry struct {
	cli   Database
	cfg   *Config
	cache sync.Map // service_name : *Service
	evs   *eventWrapper
}

func newRegistry(ctx context.Context, cfg Config) (r Interface, err error) {
	cfg.check()
	reg := &registry{cfg: &cfg}
	switch cfg.Type {
	case TypeEtcd:
		reg.cli, err = newEtcd(ctx, cfg.Database)
	default:
		err = fmt.Errorf("unknown registry type \"%s\"", cfg.Type)
		return
	}

	// build local cache
	reg.buildCache(ctx)
	// watch changes and update local cache
	// ** notice if context.Done() watch loop will stop
	go reg.watch(ctx)

	return reg, nil
}

func (r *registry) register(ctx context.Context, ins *Instance) (err error) {
	// check is already registered
	if currentInstance != nil {
		return ErrAlreadyRegistered
	}
	currentInstance = ins
	service, err := r.GetService(ctx, currentInstance.ServiceName)
	if err != nil {
		return
	}
	for _, instance := range service.instances {
		if instance.Id == currentInstance.Id {
			return ErrAlreadyRegistered
		}
	}
	// register with heartbeat
	if err = r.cli.Set(context.Background(),
		currentInstance.registryIdentity(r.cfg.Prefix),
		currentInstance.String(),
		r.cfg.HeartBeatInterval, true); err != nil {
		return
	}
	g.Log().Infof(ctx, "registry success: %s", currentInstance.String())
	// rebuild local cache
	r.buildCache(ctx)
	return
}

func (r *registry) Deregister(ctx context.Context) (err error) {
	if currentInstance == nil {
		return
	}
	err = r.cli.Delete(ctx, currentInstance.registryIdentity())
	return
}

func (r *registry) GetService(_ context.Context, serviceName string) (service *Service, err error) {
	value, ok := r.cache.Load(serviceName)
	if ok {
		service = value.(*Service)
	} else {
		service = new(Service)
	}
	return
}

func (r *registry) GetServices(_ context.Context) (services map[string]*Service, err error) {
	services = make(map[string]*Service)
	r.cache.Range(func(key, value interface{}) bool {
		services[key.(string)] = value.(*Service)
		return true
	})
	return
}

func (r *registry) RegisterEventHandler(handler EventHandler) {
	if r.evs == nil {
		r.evs = &eventWrapper{handler: handler}
		return
	}
	p := r.evs
	for p != nil && p.next != nil {
		p = p.next
	}
	p.next = &eventWrapper{handler: handler}
}

func (r *registry) buildCache(ctx context.Context) {
	response, err := r.cli.Get(ctx, r.cfg.Prefix)
	if err != nil {
		g.Log().Errorf(ctx, "registry failed to build etcd cache: %v", err)
		return
	}
	size := 0
	for _, kv := range response {
		instance := new(Instance)
		if err = kv.Value.Struct(&instance); err != nil {
			return
		}

		serviceName := instance.ServiceName
		v, ok := r.cache.Load(serviceName)
		var service *Service
		if !ok || v == nil {
			service = new(Service)
			r.cache.Store(serviceName, service)
		} else {
			service = v.(*Service)
		}
		service.append(instance)
		size++
	}

	g.Log().Infof(ctx, "registry etcd cache builded, size=%v", size)
}

func (r *registry) watch(ctx context.Context) {
	err := r.cli.Watch(ctx, r.cfg.Prefix, func(ctx context.Context, e Event) {
		var instance *Instance
		switch e.Type {
		case EventTypeDelete:
			g.Log().Infof(ctx, "registry node delete event: %v", e.Key)
			// find and delete instance by e.key=instance.Identity()
			r.cache.Range(func(key, value interface{}) bool {
				var (
					deleted = false
					service = value.(*Service)
				)

				instance = service.remove(strings.TrimPrefix(e.Key, r.cfg.Prefix))
				deleted = instance != nil

				if deleted {
					if len(service.instances) == 0 {
						r.cache.Delete(key)
					} else {
						r.cache.Store(key, service)
					}
				}
				return !deleted
			})
		case EventTypeCreate, EventTypeUpdate:
			g.Log().Infof(ctx, "registry node register event: %v", e.Key)
			instance = new(Instance)
			if err := e.Value.Struct(&instance); err != nil {
				g.Log().Errorf(ctx, "registry failed to update on watch: %v", err)
				return
			}

			service, err := r.GetService(ctx, instance.ServiceName)
			if err != nil {
				g.Log().Errorf(ctx, "registry failed to update on watch: %v", err)
				return
			}
			if e.Type == EventTypeUpdate {
				service.append(instance)
			} else if e.Type == EventTypeUpdate {
				service.Range(func(i *Instance) bool {
					if i.Id == instance.Id {
						*i = *instance
						return false
					}
					return true
				})
			}

			r.cache.Store(instance.ServiceName, service)
			if currentInstance != nil && instance.Id == currentInstance.Id {
				currentInstance = instance.clone()
			}
		}

		r.pushEvent(instance, e.Type)
	})
	if err != nil {
		g.Log().Errorf(ctx, "registry failed to watch etcd: %v", err)
	}
}

func (r *registry) pushEvent(instance *Instance, e EventType) {
	ins := instance.clone()
	p := r.evs
	for p != nil {
		go p.handler(ins, e)
		p = p.next
	}
}
