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
	currentInstance = &Instance{}
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
		// Register register currentInstance
		Register(ctx context.Context) (err error)
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
	EventHandler func(ctx context.Context, i *Instance, e EventType)
	// eventWrapper ...
	eventWrapper struct {
		handler EventHandler
		next    *eventWrapper
	}
)

// Init registry module with config, if *Instance is provided will be register automatically
func Init(ctx context.Context, config Config, ins ...*Instance) (err error) {
	// create registry instance
	if Registry, err = NewRegistry(ctx, config); err != nil {
		return
	}
	// collect instance info and register
	if len(ins) > 0 && ins[0] != nil {
		i := ins[0].clone()
		i.fillInfo()
		currentInstance = i
		err = Registry.Register(ctx)
	}
	return
}

type registry struct {
	cli   Database
	cfg   *Config
	cache sync.Map // service_name : *Service
	evs   *eventWrapper
}

func NewRegistry(ctx context.Context, cfg Config) (r Interface, err error) {
	cfg.check()
	reg := &registry{cfg: &cfg}
	switch cfg.Type {
	case TypeEtcd:
		reg.cli, err = newEtcd(ctx, cfg.Database)
	default:
		err = fmt.Errorf("unknown registry type \"%s\"", cfg.Type)
		return
	}
	go reg.watch(context.Background())
	r = reg
	return
}

func (r *registry) Register(ctx context.Context) (err error) {
	// check is already registered
	service, err := r.GetService(ctx, currentInstance.ServiceName)
	if err != nil {
		return
	}
	for _, instance := range service.instances {
		if instance.Id == currentInstance.Id {
			err = errors.New("instance already registered")
			return
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
	// build local cache
	r.buildCache(ctx)
	return
}

func (r *registry) Deregister(ctx context.Context) (err error) {
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
		service.Append(instance)
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

				instance = service.Remove(strings.TrimPrefix(e.Key, r.cfg.Prefix))
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
				service.Append(instance)
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
			if instance.Id == currentInstance.Id {
				currentInstance = instance.clone()
			}
		}

		r.pushEvent(ctx, instance, e.Type)
	})
	if err != nil {
		g.Log().Errorf(ctx, "registry failed to watch etcd: %v", err)
	}
}

func (r *registry) pushEvent(_ context.Context, instance *Instance, e EventType) {
	p := r.evs
	for p != nil {
		go p.handler(context.Background(), instance, e)
		p = p.next
	}
}
