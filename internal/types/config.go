package types

import (
	"encoding/json"
	"fmt"
	"os"
)

type CreateConfig struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	RootFS     string            `json:"rootfs"`
	Hostname   string            `json:"hostname"`
	Entrypoint []string          `json:"entrypoint"`
	Args       []string          `json:"args"`
	Env        map[string]string `json:"env"`
	WorkingDir string            `json:"workingDir"`
	Shell      string            `json:"shell"`
	User       UserConfig        `json:"user"`
	Network    NetworkConfig     `json:"network"`
	Resources  ResourceLimits    `json:"resources"`
}

type UserConfig struct {
	UID int `json:"uid"`
	GID int `json:"gid"`
}

type NetworkConfig struct {
	Enabled bool          `json:"enabled"`
	Ports   []PortMapping `json:"ports,omitempty"`
}

func LoadCreateConfig(path string) (*CreateConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config file not found: %s", path)
	}
	dec := json.NewDecoder(bytesReader(data))
	dec.DisallowUnknownFields()
	var cfg CreateConfig
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *CreateConfig) ApplyDefaults() {
	if c.Args == nil {
		c.Args = []string{}
	}
	if c.Env == nil {
		c.Env = map[string]string{}
	}
	if c.WorkingDir == "" {
		c.WorkingDir = "/"
	}
	if c.Shell == "" {
		c.Shell = "/bin/sh"
	}
	if c.Hostname == "" {
		c.Hostname = c.Name
	}
}

func (c *CreateConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("config field \"name\" is required")
	}
	if c.Version == "" {
		return fmt.Errorf("config field \"version\" is required")
	}
	if c.RootFS == "" {
		return fmt.Errorf("config field \"rootfs\" is required")
	}
	if len(c.Entrypoint) == 0 {
		return fmt.Errorf("config field \"entrypoint\" is required and must contain at least one item")
	}
	return nil
}
