package simple_registry

import (
	"crypto/tls"
	"fmt"
)

var (
	TypeEtcd = "etcd"
)

type (
	// Config for registry
	Config struct {
		Type              string         `json:"type"`
		Database          DatabaseConfig `json:"database"`
		Storage           StorageConfig  `json:"storage"`
		Prefix            string         `json:"prefix"`              // start with "/",and end with "/" in etcd
		HeartBeatInterval int64          `json:"heart_beat_interval"` // default 3s
	}
	// StorageConfig for storage module
	StorageConfig struct {
		Separator string `json:"separator"`
	}
	// DatabaseConfig for etcd,consul,nacos...
	DatabaseConfig struct {
		// common
		Endpoints []string `json:"endpoints"`
		Username  string   `json:"username"`
		Password  string   `json:"password"`
		// etcd tls
		Tls *TlsConfig `json:"tls"`
	}

	// TlsConfig ...
	TlsConfig struct {
		InsecureSkipVerify bool `json:"insecure_skip_verify"`
	}
)

func (c DatabaseConfig) tlsConfig() *tls.Config {
	if c.Tls == nil {
		return nil
	}
	return &tls.Config{
		InsecureSkipVerify: c.Tls.InsecureSkipVerify,
	}
}

func (c *Config) check() {
	if c.Prefix == "" {
		c.Prefix = defaultRegistryPrefix
	}
	if c.HeartBeatInterval == 0 {
		c.HeartBeatInterval = defaultHeartBeatInterval
	}
	if c.Storage.Separator == "" {
		c.Storage.Separator = defaultIdentitySeparator
	}
}

func (c *Config) getStoragePrefix() string {
	return fmt.Sprintf("%sstorage/", c.Prefix)
}

func (c *Config) getRegistryPrefix() string {
	return fmt.Sprintf("%sregistry/", c.Prefix)
}
