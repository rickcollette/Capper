// Package oci generates OCI Runtime Spec bundles from Capper capsule manifests.
// The generated config.json targets OCI Runtime Specification 1.0 and is
// compatible with crun and runc.
package oci

import (
	"encoding/json"
	"os"
	"path/filepath"

	"capper/internal/types"
)

// rootless reports whether the current process is not running as root.
// crun/runc require a "user" namespace entry in the config when rootless.
func rootless() bool {
	return os.Getuid() != 0
}

// Spec is a minimal OCI Runtime Spec config.json representation.
// Only the fields required by crun/runc for capsule execution are included.
type Spec struct {
	OCIVersion  string            `json:"ociVersion"`
	Process     Process           `json:"process"`
	Root        Root              `json:"root"`
	Hostname    string            `json:"hostname"`
	Mounts      []Mount           `json:"mounts"`
	Linux       Linux             `json:"linux"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type Process struct {
	Terminal        bool     `json:"terminal"`
	User            User     `json:"user"`
	Args            []string `json:"args"`
	Env             []string `json:"env"`
	Cwd             string   `json:"cwd"`
	NoNewPrivileges bool     `json:"noNewPrivileges"`
}

type User struct {
	UID int `json:"uid"`
	GID int `json:"gid"`
}

type Root struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly"`
}

type Mount struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Options     []string `json:"options,omitempty"`
}

type Linux struct {
	Namespaces []Namespace `json:"namespaces"`
	Resources  Resources   `json:"resources,omitempty"`
}

type Namespace struct {
	Type string `json:"type"`
}

type Resources struct {
	Memory *MemoryResources `json:"memory,omitempty"`
	Pids   *PidsResources   `json:"pids,omitempty"`
}

type MemoryResources struct {
	Limit int64 `json:"limit,omitempty"`
}

type PidsResources struct {
	Limit int64 `json:"limit,omitempty"`
}

// Generate creates an OCI bundle directory at bundleDir.
// The rootfs must already exist at <bundleDir>/rootfs.
// Returns the path to the written config.json.
func Generate(bundleDir string, manifest types.CapsuleManifest) (string, error) {
	args := append([]string{}, manifest.Entrypoint...)
	args = append(args, manifest.Args...)

	env := []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	for k, v := range manifest.Env {
		env = append(env, k+"="+v)
	}

	namespaces := []Namespace{
		{Type: "pid"},
		{Type: "ipc"},
		{Type: "uts"},
		{Type: "mount"},
	}
	if !manifest.Network.Enabled {
		namespaces = append(namespaces, Namespace{Type: "network"})
	}
	// crun/runc require a user namespace when running rootless so that the
	// container UID/GID map is valid. Without it, the runtime errors out.
	if rootless() {
		namespaces = append(namespaces, Namespace{Type: "user"})
	}

	var res Resources
	if manifest.Resources.MemoryBytes > 0 {
		res.Memory = &MemoryResources{Limit: manifest.Resources.MemoryBytes}
	}
	if manifest.Resources.MaxProcesses > 0 {
		res.Pids = &PidsResources{Limit: manifest.Resources.MaxProcesses}
	}

	mounts := []Mount{
		{Destination: "/proc", Type: "proc", Source: "proc"},
		{Destination: "/dev", Type: "tmpfs", Source: "tmpfs", Options: []string{"nosuid", "strictatime", "mode=755", "size=65536k"}},
		{Destination: "/dev/pts", Type: "devpts", Source: "devpts", Options: []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"}},
		{Destination: "/sys", Type: "sysfs", Source: "sysfs", Options: []string{"nosuid", "noexec", "nodev", "ro"}},
		{Destination: "/tmp", Type: "tmpfs", Source: "tmpfs", Options: []string{"nosuid", "nodev"}},
	}
	for _, m := range manifest.Mounts {
		mt := m.Type
		if mt == "" {
			mt = "bind"
		}
		opts := []string{"bind", "rprivate"}
		if m.ReadOnly {
			opts = append(opts, "ro")
		}
		if mt == "tmpfs" {
			opts = []string{"nosuid", "nodev"}
		}
		mounts = append(mounts, Mount{
			Destination: m.Target,
			Type:        mt,
			Source:      m.Source,
			Options:     opts,
		})
	}

	var annotations map[string]string
	if len(manifest.Network.Ports) > 0 {
		portsJSON, err := json.Marshal(manifest.Network.Ports)
		if err == nil {
			annotations = map[string]string{"org.capper.ports": string(portsJSON)}
		}
	}

	hostname := manifest.Hostname
	if hostname == "" {
		hostname = manifest.Name
	}

	spec := Spec{
		OCIVersion: "1.0.0",
		Process: Process{
			Terminal:        false,
			User:            User{UID: manifest.User.UID, GID: manifest.User.GID},
			Args:            args,
			Env:             env,
			Cwd:             manifest.WorkingDir,
			NoNewPrivileges: true,
		},
		Root:        Root{Path: "rootfs", Readonly: false},
		Hostname:    hostname,
		Mounts:      mounts,
		Annotations: annotations,
		Linux: Linux{
			Namespaces: namespaces,
			Resources:  res,
		},
	}

	configPath := filepath.Join(bundleDir, "config.json")
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return configPath, nil
}
