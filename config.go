package simple_registry

import (
	"crypto/tls"
)

var (
	TypeEtcd = "etcd"
)

type (
	// Config for registry
	Config struct {
		Type              string         `json:"type"`
		Database          DatabaseConfig `json:"database"`
		Prefix            string         `json:"prefix"`              // start with "/",and end with "/" in etcd
		HeartBeatInterval int64          `json:"heart_beat_interval"` // default 3s
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
		InsecureSkipVerify bool
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
}
