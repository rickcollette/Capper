package cappersdk

import (
	"context"
	"net/url"
)

// ---- Firewalls -------------------------------------------------------------

// FirewallsAPI accesses network firewall policies.
type FirewallsAPI struct{ c *Client }

// Firewall is a network firewall policy (keyed by network).
type Firewall struct {
	NetworkID            string `json:"networkID"`
	NetworkName          string `json:"networkName"`
	Mode                 string `json:"mode"`
	Backend              string `json:"backend"`
	DefaultForwardPolicy string `json:"defaultForwardPolicy"`
	DefaultIngressPolicy string `json:"defaultIngressPolicy"`
	DefaultEgressPolicy  string `json:"defaultEgressPolicy"`
	NATEnabled           bool   `json:"natEnabled"`
}

// FirewallRule is a single firewall rule.
type FirewallRule struct {
	ID        string `json:"id"`
	Direction string `json:"direction"`
	Action    string `json:"action"`
	Protocol  string `json:"protocol"`
	Port      string `json:"port"`
	Source    string `json:"source"`
	Enabled   bool   `json:"enabled"`
}

// List returns all firewalls.
func (a *FirewallsAPI) List(ctx context.Context) ([]Firewall, error) {
	var out struct {
		Data []Firewall `json:"data"`
	}
	return out.Data, a.c.get(ctx, "firewalls", &out)
}

// Get returns a firewall by name.
func (a *FirewallsAPI) Get(ctx context.Context, name string) (Firewall, error) {
	var out struct {
		Data Firewall `json:"data"`
	}
	return out.Data, a.c.get(ctx, "firewalls/"+name, &out)
}

// ListRules returns a firewall's rules.
func (a *FirewallsAPI) ListRules(ctx context.Context, name string) ([]FirewallRule, error) {
	var out struct {
		Data []FirewallRule `json:"data"`
	}
	return out.Data, a.c.get(ctx, "firewalls/"+name+"/rules", &out)
}

// Apply enforces a firewall's rules on the data plane.
func (a *FirewallsAPI) Apply(ctx context.Context, name string) error {
	return a.c.post(ctx, "firewalls/"+name+"/apply", nil, nil)
}

// ---- Secrets ---------------------------------------------------------------

// SecretsAPI accesses the secret store (values are never returned in lists).
type SecretsAPI struct{ c *Client }

// SecretMeta is a secret's metadata (no value).
type SecretMeta struct {
	Name      string `json:"name"`
	Version   int    `json:"version"`
	CreatedAt string `json:"createdAt"`
}

// List returns secret metadata.
func (a *SecretsAPI) List(ctx context.Context) ([]SecretMeta, error) {
	var out struct {
		Data []SecretMeta `json:"data"`
	}
	return out.Data, a.c.get(ctx, "secrets", &out)
}

// Create stores a new secret value.
func (a *SecretsAPI) Create(ctx context.Context, name, value string) error {
	return a.c.post(ctx, "secrets", map[string]string{"name": name, "value": value}, nil)
}

// Delete removes a secret.
func (a *SecretsAPI) Delete(ctx context.Context, name string) error {
	return a.c.del(ctx, "secrets/"+name)
}

// ---- Databases -------------------------------------------------------------

// DatabasesAPI accesses managed databases.
type DatabasesAPI struct{ c *Client }

// Database is a managed database instance.
type Database struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Engine  string `json:"engine"`
	Version string `json:"version"`
	Status  string `json:"status"`
}

// List returns all databases.
func (a *DatabasesAPI) List(ctx context.Context) ([]Database, error) {
	var out struct {
		Data []Database `json:"data"`
	}
	return out.Data, a.c.get(ctx, "databases", &out)
}

// Get returns a database by name.
func (a *DatabasesAPI) Get(ctx context.Context, name string) (Database, error) {
	var out struct {
		Data Database `json:"data"`
	}
	return out.Data, a.c.get(ctx, "databases/"+name, &out)
}

// Delete removes a database.
func (a *DatabasesAPI) Delete(ctx context.Context, name string) error {
	return a.c.del(ctx, "databases/"+name)
}

// ---- Certificates ----------------------------------------------------------

// CertificatesAPI accesses the certificate manager.
type CertificatesAPI struct{ c *Client }

// Certificate is a managed TLS certificate.
type Certificate struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	CommonName string   `json:"commonName"`
	SANs       []string `json:"sans"`
	Issuer     string   `json:"issuer"`
	Status     string   `json:"status"`
	NotAfter   string   `json:"notAfter"`
	AutoRenew  bool     `json:"autoRenew"`
}

// List returns all certificates.
func (a *CertificatesAPI) List(ctx context.Context) ([]Certificate, error) {
	var out struct {
		Data []Certificate `json:"data"`
	}
	return out.Data, a.c.get(ctx, "certificates", &out)
}

// Get returns a certificate by ID.
func (a *CertificatesAPI) Get(ctx context.Context, id string) (Certificate, error) {
	var out struct {
		Data Certificate `json:"data"`
	}
	return out.Data, a.c.get(ctx, "certificates/"+id, &out)
}

// Renew triggers a renewal for a certificate.
func (a *CertificatesAPI) Renew(ctx context.Context, id string) error {
	return a.c.post(ctx, "certificates/"+id+"/renew", nil, nil)
}

// Delete removes a certificate.
func (a *CertificatesAPI) Delete(ctx context.Context, id string) error {
	return a.c.del(ctx, "certificates/"+id)
}

// ---- Storage ---------------------------------------------------------------

// StorageAPI accesses block volumes and object buckets.
type StorageAPI struct{ c *Client }

// Volume is a block storage volume.
type Volume struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	SizeBytes          int64  `json:"sizeBytes"`
	Class              string `json:"class"`
	Backend            string `json:"backend"`
	Encrypted          bool   `json:"encrypted"`
	AttachedInstanceID string `json:"attachedInstanceId,omitempty"`
}

// Bucket is an object storage bucket.
type Bucket struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Backend    string `json:"backend"`
	Versioning bool   `json:"versioning"`
	Encrypted  bool   `json:"encrypted"`
}

// ListVolumes returns block volumes for a project.
func (a *StorageAPI) ListVolumes(ctx context.Context, project string) ([]Volume, error) {
	path := "storage/volumes"
	if project != "" {
		path += "?project=" + url.QueryEscape(project)
	}
	var out struct {
		Data []Volume `json:"data"`
	}
	return out.Data, a.c.get(ctx, path, &out)
}

// ListBuckets returns object storage buckets.
func (a *StorageAPI) ListBuckets(ctx context.Context) ([]Bucket, error) {
	var out struct {
		Data []Bucket `json:"data"`
	}
	return out.Data, a.c.get(ctx, "storage/buckets", &out)
}

// AttachVolume attaches a volume to an instance.
func (a *StorageAPI) AttachVolume(ctx context.Context, name, instanceID string) error {
	return a.c.post(ctx, "storage/volumes/"+name+"/attach", map[string]string{"instanceId": instanceID}, nil)
}

// DetachVolume detaches a volume.
func (a *StorageAPI) DetachVolume(ctx context.Context, name string) error {
	return a.c.post(ctx, "storage/volumes/"+name+"/detach", nil, nil)
}
