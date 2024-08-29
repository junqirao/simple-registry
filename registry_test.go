package simple_registry

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func getConfig() Config {
	return Config{
		Type: TypeEtcd,
		Database: DatabaseConfig{
			Endpoints: []string{"127.0.0.1:2379", "127.0.0.1:2380", "127.0.0.1:2381"},
			Username:  "",
			Password:  "",
			Tls:       nil,
		},
		Prefix: "/test-registry/",
	}
}

func TestInitWithoutInstance(t *testing.T) {
	err := Init(context.Background(), getConfig())
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestInit(t *testing.T) {
	err := Init(context.Background(), getConfig(),
		NewInstance("test-service").
			WithAddress("127.0.0.1", 8080).
			WithMetaData(map[string]interface{}{"key": "value"}))
	if err != nil {
		t.Fatal(err)
		return
	}
	r := Registry.(*registry)
	r.cache.Range(func(serviceName, s interface{}) bool {
		service := s.(*Service)
		service.Range(func(instance *Instance) bool {
			t.Log(instance)
			return true
		})
		return true
	})
	t.Log("wait 20 s")
	time.Sleep(time.Second * 20)
}

func TestRegistry(t *testing.T) {
	err := Init(context.Background(), getConfig(),
		NewInstance("test-service").
			WithAddress("127.0.0.1", 8080).
			WithMetaData(map[string]interface{}{"key": "value"}))
	if err != nil {
		t.Fatal(err)
		return
	}

	service, err := Registry.GetService(context.Background(), "test-service")
	if err != nil {
		t.Fatal(err)
		return
	}
	fmt.Printf("service: %+v\n", service.Instances())
	instance := service.Instances()[0]
	if instance.Id != currentInstance.Id {
		t.Fatal("instance id not equal")
	}

	services, err := Registry.GetServices(context.Background())
	if err != nil {
		t.Fatal(err)
		return
	}
	for serviceName, s := range services {
		fmt.Printf("services[%s]: %+v\n", serviceName, s.Instances())
	}

	Registry.RegisterEventHandler(func(instance *Instance, e EventType) {
		fmt.Printf("event: %s, instance: %+v\n", e, instance)
	})

	err = Registry.Deregister(context.Background())
	if err != nil {
		t.Fatal(err)
		return
	}
}
