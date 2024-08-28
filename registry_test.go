package simple_registry

import (
	"context"
	"testing"
	"time"
)

func getConfig() Config {
	return Config{
		Type: TypeEtcd,
		Database: DatabaseConfig{
			Endpoints: []string{"172.18.28.11:2379", "172.18.28.11:2380", "172.18.28.11:2381"},
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
