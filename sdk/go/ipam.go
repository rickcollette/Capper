package cappersdk

import "context"

// IPAMAPI accesses Capper's Public IPAM (routable IP pools / Elastic IPs).
type IPAMAPI struct{ c *Client }

// IPPool is a routable IP pool.
type IPPool struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	CIDR              string   `json:"cidr"`
	Scope             string   `json:"scope"`
	Gateway           string   `json:"gateway"`
	Usage             []string `json:"usage"`
	Status            string   `json:"status"`
	AllowAutoAllocate bool     `json:"allowAutoAllocate"`
}

// RoutableIP is a single reserved/allocated address.
type RoutableIP struct {
	ID         string `json:"id"`
	PoolID     string `json:"poolId"`
	Address    string `json:"address"`
	Status     string `json:"status"`
	Project    string `json:"project"`
	Name       string `json:"name"`
	Purpose    string `json:"purpose"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
}

// CreatePool creates a routable IP pool. excluded addresses are skipped.
func (a *IPAMAPI) CreatePool(ctx context.Context, pool IPPool, excluded []string) error {
	body := map[string]any{
		"name": pool.Name, "cidr": pool.CIDR, "scope": pool.Scope, "gateway": pool.Gateway,
		"usage": pool.Usage, "allowAutoAllocate": pool.AllowAutoAllocate, "excluded": excluded,
	}
	return a.c.post(ctx, "ip-pools", body, nil)
}

// ListPools returns all IP pools.
func (a *IPAMAPI) ListPools(ctx context.Context) ([]IPPool, error) {
	var out struct {
		Data []IPPool `json:"data"`
	}
	return out.Data, a.c.get(ctx, "ip-pools", &out)
}

// Reserve reserves an address from a pool.
func (a *IPAMAPI) Reserve(ctx context.Context, pool, name, purpose, address string) (RoutableIP, error) {
	body := map[string]any{"pool": pool, "name": name, "purpose": purpose, "address": address}
	var out struct {
		Data RoutableIP `json:"data"`
	}
	return out.Data, a.c.post(ctx, "ips/reserve", body, &out)
}

// ListIPs returns addresses, optionally filtered by pool/status.
func (a *IPAMAPI) ListIPs(ctx context.Context) ([]RoutableIP, error) {
	var out struct {
		Data []RoutableIP `json:"data"`
	}
	return out.Data, a.c.get(ctx, "ips", &out)
}

// Release returns an address to the pool.
func (a *IPAMAPI) Release(ctx context.Context, ipID string) error {
	return a.c.post(ctx, "ips/"+ipID+"/release", nil, nil)
}
