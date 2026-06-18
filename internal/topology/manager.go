package topology

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"
)

// Manager wraps the topology store and provides higher-level operations.
type Manager struct {
	store *Store
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{store: NewStore(db)}
}

func (m *Manager) Store() *Store { return m.store }

// EnsureLocalTopology creates the default local realm/region/zone/node if
// no topology records exist. This keeps single-node mode working transparently.
func (m *Manager) EnsureLocalTopology() error {
	realms, err := m.store.ListRealms()
	if err != nil {
		return err
	}
	if len(realms) > 0 {
		return nil
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	realm := Realm{
		ID:          "rlm_local",
		Slug:        "local",
		Name:        "Local Realm",
		Description: "Default single-node realm (auto-created)",
		Status:      StatusActive,
	}
	if err := m.store.InsertRealm(realm); err != nil {
		log.Printf("[topology] auto-create realm: %v", err)
	}

	region := Region{
		ID:      "reg_local",
		RealmID: "rlm_local",
		Slug:    "local",
		Name:    "Local Region",
		Status:  StatusActive,
	}
	if err := m.store.InsertRegion(region); err != nil {
		log.Printf("[topology] auto-create region: %v", err)
	}

	zone := Zone{
		ID:       "zon_local",
		RealmID:  "rlm_local",
		RegionID: "reg_local",
		Slug:     "local",
		Name:     "Local Zone",
		Status:   StatusActive,
	}
	if err := m.store.InsertZone(zone); err != nil {
		log.Printf("[topology] auto-create zone: %v", err)
	}

	node := Node{
		ID:       fmt.Sprintf("nod_%s", hostname),
		RealmID:  "rlm_local",
		RegionID: "reg_local",
		ZoneID:   "zon_local",
		Slug:     hostname,
		Name:     hostname,
		Address:  "127.0.0.1",
		Status:   StatusReady,
	}
	if _, err := m.store.InsertNode(node); err != nil {
		log.Printf("[topology] auto-create node: %v", err)
	}

	return nil
}

// DefaultRealm returns the first realm (or the "local" realm for single-node mode).
func (m *Manager) DefaultRealm() (Realm, error) {
	return m.store.GetRealm("local")
}

// StartHeartbeatMonitor runs a background goroutine that marks nodes offline
// when they haven't sent a heartbeat within staleThreshold.
func (m *Manager) StartHeartbeatMonitor(ctx context.Context, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				stale, err := m.store.ListStaleNodes(90 * time.Second)
				if err != nil {
					log.Printf("[topology] heartbeat monitor: %v", err)
					continue
				}
				for _, n := range stale {
					if err := m.store.UpdateNodeStatus(n.ID, StatusOffline); err != nil {
						log.Printf("[topology] mark offline %s: %v", n.ID, err)
					}
				}
			}
		}
	}()
}
