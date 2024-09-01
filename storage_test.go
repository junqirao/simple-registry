package simple_registry

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func dfs(node *storageNode) {
	if node == nil {
		return
	}
	fmt.Printf("%s: %+v\n", node.name, node.values)
	node.next.Range(func(_, value any) bool {
		dfs(value.(*storageNode))
		return true
	})
}
func TestCachedStorage(t *testing.T) {
	err := Init(context.Background(), getConfig())
	if err != nil {
		t.Fatal(err)
		return
	}
	sto := Storages.Get("test")
	cs := sto.(*cachedStorage)
	// print tree
	dfs(cs.root)

	fmt.Println("build cache ---------------")
	err = sto.Set(context.Background(), "key", "value")
	if err != nil {
		t.Fatal(err)
		return
	}
	err = sto.Set(context.Background(), "key1", "value1")
	if err != nil {
		t.Fatal(err)
		return
	}
	err = sto.Set(context.Background(), "key2", "value2")
	if err != nil {
		t.Fatal(err)
		return
	}
	err = sto.Set(context.Background(), "key3/1", "value3-1")
	if err != nil {
		t.Fatal(err)
		return
	}
	err = sto.Set(context.Background(), "key3/2", "value3-2")
	if err != nil {
		t.Fatal(err)
		return
	}
	dfs(cs.root)
	fmt.Println("rebuild cache ---------------")
	cs.buildCache(context.Background())
	dfs(cs.root)

	fmt.Println("get all ---------------")
	kvs, err := sto.Get(context.Background())
	if err != nil {
		t.Fatal(err)
		return
	}
	for _, kv := range kvs {
		t.Logf("%+v", kv)
	}
	fmt.Println("get one ---------------")
	vs, err := sto.Get(context.Background(), "key")
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Logf("key: %+v", vs[0])

	fmt.Println("delete ---------------")
	err = sto.Delete(context.Background(), "key")
	if err != nil {
		t.Fatal(err)
		return
	}
	err = sto.Delete(context.Background(), "key3")
	if err != nil {
		t.Fatal(err)
		return
	}
	dfs(cs.root)
}

func TestEvent(t *testing.T) {
	err := Init(context.Background(), getConfig())
	if err != nil {
		t.Fatal(err)
		return
	}

	go func() {
		for {
			dfs(Storages.Get("test").(*cachedStorage).root)
			fmt.Println("---------------------------")
			time.Sleep(time.Second * 5)
		}
	}()

	time.Sleep(time.Minute)
}
