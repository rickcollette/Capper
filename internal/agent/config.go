package agent

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the parsed /etc/capper/agent.yaml.
type Config struct {
	Node         NodeConfig         `yaml:"node"`
	ControlPlane ControlPlaneConfig `yaml:"controlPlane"`
	Services     ServicesConfig     `yaml:"services"`
}

type NodeConfig struct {
	Name          string   `yaml:"name"`
	Realm         string   `yaml:"realm"`
	Region        string   `yaml:"region"`
	Zone          string   `yaml:"zone"`
	Roles         []string `yaml:"roles"`
	FailureDomain string   `yaml:"failureDomain"`
}

type ControlPlaneConfig struct {
	URL               string        `yaml:"url"`
	HeartbeatInterval time.Duration `yaml:"heartbeatInterval"`
	TLSVerify         bool          `yaml:"tlsVerify"`
}

type ServicesConfig struct {
	Compute    bool `yaml:"compute"`
	SharedDisk bool `yaml:"sharedDisk"`
	S3         bool `yaml:"s3"`
	Network    bool `yaml:"network"`
	Ingress    bool `yaml:"ingress"`
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Node: NodeConfig{
			Name:          "devbox",
			Realm:         "local",
			Region:        "local",
			Zone:          "local-a",
			Roles:         []string{"all-in-one"},
			FailureDomain: "local-0",
		},
		ControlPlane: ControlPlaneConfig{
			URL:               "http://localhost:8080",
			HeartbeatInterval: 10 * time.Second,
			TLSVerify:         false,
		},
	}
}

// LoadConfig reads and parses a YAML config file.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("agent: read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("agent: parse config %s: %w", path, err)
	}
	if cfg.ControlPlane.HeartbeatInterval == 0 {
		cfg.ControlPlane.HeartbeatInterval = 10 * time.Second
	}
	return cfg, nil
}
