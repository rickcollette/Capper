package dns

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Manager provides high-level DNS lifecycle operations.
type Manager struct {
	store *Store
}

// NewManager returns a Manager backed by the given store.
func NewManager(s *Store) *Manager {
	return &Manager{store: s}
}

// ---- zones ------------------------------------------------------------------

// CreateZone creates a new hosted zone. If a zone with the same name (and
// networkID) already exists, it is returned unchanged (idempotent).
func (m *Manager) CreateZone(name, zoneType, networkID string, defaultTTL int, description string) (Zone, error) {
	name = normZoneName(name)
	existing, _ := m.store.GetZone(name, networkID)
	if existing.ID != "" {
		return existing, nil
	}
	if defaultTTL <= 0 {
		defaultTTL = 30
	}
	if zoneType == "" {
		zoneType = ZoneTypePrivate
	}
	z := Zone{
		ID:          newID("zone"),
		Name:        name,
		Type:        zoneType,
		NetworkID:   networkID,
		DefaultTTL:  defaultTTL,
		Description: description,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.InsertZone(z); err != nil {
		return Zone{}, err
	}
	// Auto-create system records: gateway and dns point to the first IP of the zone.
	// These are created lazily when the zone is attached to a real network.
	return z, nil
}

// GetZone returns a zone by name or ID.
func (m *Manager) GetZone(nameOrID, networkID string) (Zone, error) {
	z, err := m.store.GetZone(nameOrID, networkID)
	if err != nil {
		return Zone{}, fmt.Errorf("dns: zone %q not found: %w", nameOrID, err)
	}
	return z, nil
}

// ListZones returns all zones, optionally filtered to a network.
func (m *Manager) ListZones(networkID string) ([]Zone, error) {
	return m.store.ListZones(networkID)
}

// DeleteZone removes a zone and all its records, services, and system entries.
func (m *Manager) DeleteZone(nameOrID, networkID string) error {
	z, err := m.store.GetZone(nameOrID, networkID)
	if err != nil {
		return fmt.Errorf("dns: zone %q not found", nameOrID)
	}
	return m.store.DeleteZone(z.ID)
}

// ---- records ----------------------------------------------------------------

// CreateRecord adds a manual DNS record. Returns an error if the zone doesn't exist.
func (m *Manager) CreateRecord(zoneName, networkID, name, recType string, values []string, ttl int) (Record, error) {
	z, err := m.store.GetZone(zoneName, networkID)
	if err != nil {
		return Record{}, fmt.Errorf("dns: zone %q not found", zoneName)
	}
	if ttl <= 0 {
		ttl = z.DefaultTTL
	}
	recType = strings.ToUpper(recType)
	fqdn := normFQDN(name + "." + z.Name)
	r := Record{
		ID:        newID("rec"),
		ZoneID:    z.ID,
		Name:      strings.ToLower(name),
		FQDN:      fqdn,
		Type:      recType,
		Values:    values,
		TTL:       ttl,
		Source:    RecordSourceManual,
		Enabled:   true,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.InsertRecord(r); err != nil {
		return Record{}, err
	}
	return r, nil
}

// ListRecords returns all records in a zone.
func (m *Manager) ListRecords(zoneName, networkID string) ([]Record, error) {
	z, err := m.store.GetZone(zoneName, networkID)
	if err != nil {
		return nil, fmt.Errorf("dns: zone %q not found", zoneName)
	}
	return m.store.ListRecords(z.ID)
}

// DeleteRecord removes a record by ID, verifying it belongs to the zone.
func (m *Manager) DeleteRecord(zoneName, networkID, recordID string) error {
	z, err := m.store.GetZone(zoneName, networkID)
	if err != nil {
		return fmt.Errorf("dns: zone %q not found", zoneName)
	}
	r, err := m.store.GetRecord(recordID)
	if err != nil {
		return err
	}
	if r.ZoneID != z.ID {
		return fmt.Errorf("dns: record %q does not belong to zone %q", recordID, zoneName)
	}
	return m.store.DeleteRecord(recordID)
}

// ---- services ---------------------------------------------------------------

// CreateService registers a selector-backed service record.
func (m *Manager) CreateService(name, zoneName, networkID string, opts ServiceOptions) (ServiceRecord, error) {
	z, err := m.store.GetZone(zoneName, networkID)
	if err != nil {
		return ServiceRecord{}, fmt.Errorf("dns: zone %q not found", zoneName)
	}
	if opts.TTL <= 0 {
		opts.TTL = 5
	}
	if opts.RoutingPolicy == "" {
		opts.RoutingPolicy = RoutingMultivalue
	}
	if opts.Protocol == "" {
		opts.Protocol = "tcp"
	}
	svc := ServiceRecord{
		ID:            newID("svc"),
		ZoneID:        z.ID,
		NetworkID:     networkID,
		Name:          strings.ToLower(name),
		FQDN:          normFQDN(name + "." + z.Name),
		SelectorType:  opts.SelectorType,
		SelectorKey:   opts.SelectorKey,
		SelectorValue: opts.SelectorValue,
		Protocol:      opts.Protocol,
		Port:          opts.Port,
		TTL:           opts.TTL,
		HealthSource:  opts.HealthSource,
		RoutingPolicy: opts.RoutingPolicy,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.InsertService(svc); err != nil {
		return ServiceRecord{}, err
	}
	return svc, nil
}

// ListServices returns all service records for a zone.
func (m *Manager) ListServices(zoneName, networkID string) ([]ServiceRecord, error) {
	z, err := m.store.GetZone(zoneName, networkID)
	if err != nil {
		return nil, fmt.Errorf("dns: zone %q not found", zoneName)
	}
	return m.store.ListServices(z.ID)
}

// DeleteService removes a service record by ID.
func (m *Manager) DeleteService(zoneName, networkID, serviceID string) error {
	if _, err := m.store.GetZone(zoneName, networkID); err != nil {
		return fmt.Errorf("dns: zone %q not found", zoneName)
	}
	return m.store.DeleteService(serviceID)
}

// SetForwarders configures upstream DNS servers for a network or globally.
func (m *Manager) SetForwarders(networkID string, upstreams []string) error {
	f := Forwarder{
		ID:        newID("fwd"),
		NetworkID: networkID,
		Upstreams: upstreams,
		Enabled:   true,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return m.store.UpsertForwarder(f)
}

// ---- ServiceOptions ---------------------------------------------------------

// ServiceOptions carries parameters for service record creation.
type ServiceOptions struct {
	SelectorType  string
	SelectorKey   string
	SelectorValue string
	Protocol      string
	Port          int
	TTL           int
	HealthSource  string
	RoutingPolicy string
}

// ---- helpers ----------------------------------------------------------------

func newID(prefix string) string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func normZoneName(name string) string {
	return strings.ToLower(strings.TrimSuffix(name, "."))
}
