package cappersdk

import "context"

// ---- Organizations / Accounts ----------------------------------------------

// OrgsAPI accesses organizations and accounts (multi-tenancy).
type OrgsAPI struct{ c *Client }

// Org is an organization.
type Org struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// Account is an account within an organization.
type Account struct {
	ID          string `json:"id"`
	OrgID       string `json:"orgId"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Status      string `json:"status"`
	AccountType string `json:"accountType"`
}

// List returns all organizations.
func (a *OrgsAPI) List(ctx context.Context) ([]Org, error) {
	var out struct {
		Data []Org `json:"data"`
	}
	return out.Data, a.c.get(ctx, "orgs", &out)
}

// Create creates an organization.
func (a *OrgsAPI) Create(ctx context.Context, name string) (Org, error) {
	var out struct {
		Data Org `json:"data"`
	}
	return out.Data, a.c.post(ctx, "orgs", map[string]string{"name": name}, &out)
}

// ListAccounts returns the accounts in an organization.
func (a *OrgsAPI) ListAccounts(ctx context.Context, orgID string) ([]Account, error) {
	var out struct {
		Data []Account `json:"data"`
	}
	return out.Data, a.c.get(ctx, "orgs/"+orgID+"/accounts", &out)
}

// CreateAccount creates an account in an organization.
func (a *OrgsAPI) CreateAccount(ctx context.Context, orgID, name string) (Account, error) {
	var out struct {
		Data Account `json:"data"`
	}
	return out.Data, a.c.post(ctx, "orgs/"+orgID+"/accounts", map[string]string{"name": name}, &out)
}

// ---- Stacks ----------------------------------------------------------------

// StacksAPI accesses declarative stacks.
type StacksAPI struct{ c *Client }

// Stack is a declarative resource stack.
type Stack struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// List returns all stacks.
func (a *StacksAPI) List(ctx context.Context) ([]Stack, error) {
	var out struct {
		Data []Stack `json:"data"`
	}
	return out.Data, a.c.get(ctx, "stacks", &out)
}

// Get returns a stack by name.
func (a *StacksAPI) Get(ctx context.Context, name string) (Stack, error) {
	var out struct {
		Data Stack `json:"data"`
	}
	return out.Data, a.c.get(ctx, "stacks/"+name, &out)
}

// ---- Queues ----------------------------------------------------------------

// QueuesAPI accesses message queues.
type QueuesAPI struct{ c *Client }

// Queue is a message queue.
type Queue struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Project   string `json:"project"`
	CreatedAt string `json:"createdAt"`
}

// List returns all queues.
func (a *QueuesAPI) List(ctx context.Context) ([]Queue, error) {
	var out struct {
		Data []Queue `json:"data"`
	}
	return out.Data, a.c.get(ctx, "queues", &out)
}

// Publish enqueues a message onto a queue.
func (a *QueuesAPI) Publish(ctx context.Context, name string, message any) error {
	return a.c.post(ctx, "queues/"+name+"/publish", message, nil)
}

// ---- Backups ---------------------------------------------------------------

// BackupsAPI accesses backups.
type BackupsAPI struct{ c *Client }

// Backup is a stored backup record.
type Backup struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Project   string `json:"project"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}

// List returns all backups.
func (a *BackupsAPI) List(ctx context.Context) ([]Backup, error) {
	var out struct {
		Data []Backup `json:"data"`
	}
	return out.Data, a.c.get(ctx, "backups", &out)
}

// Restore restores a backup by ID.
func (a *BackupsAPI) Restore(ctx context.Context, id string) error {
	return a.c.post(ctx, "backups/"+id+"/restore", nil, nil)
}

// ---- Quotas ----------------------------------------------------------------

// QuotasAPI accesses account quotas.
type QuotasAPI struct{ c *Client }

// Quota is a resource quota.
type Quota struct {
	ResourceType string `json:"resourceType"`
	Limit        int    `json:"limit"`
	Used         int    `json:"used"`
}

// List returns quotas.
func (a *QuotasAPI) List(ctx context.Context) ([]Quota, error) {
	var out struct {
		Data []Quota `json:"data"`
	}
	return out.Data, a.c.get(ctx, "quotas", &out)
}

// ---- Ingress ---------------------------------------------------------------

// IngressAPI accesses ingress rules.
type IngressAPI struct{ c *Client }

// IngressRule is an ingress routing rule.
type IngressRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	PathPrefix string `json:"pathPrefix"`
	BackendLB  string `json:"backendLb"`
	TLSCert    string `json:"tlsCert,omitempty"`
}

// List returns all ingress rules.
func (a *IngressAPI) List(ctx context.Context) ([]IngressRule, error) {
	var out struct {
		Data []IngressRule `json:"data"`
	}
	return out.Data, a.c.get(ctx, "ingress", &out)
}
