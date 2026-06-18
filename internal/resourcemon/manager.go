package resourcemon

import "fmt"

// Manager is the high-level entry point for the resource monitor.
type Manager struct {
	store *Store
}

// NewManager wraps a Store.
func NewManager(store *Store) *Manager { return &Manager{store: store} }

// Store exposes the underlying store for direct access.
func (m *Manager) Store() *Store { return m.store }

// SyncResult reports the outcome of a sync pass for one resource type.
type SyncResult struct {
	ResourceType string `json:"resourceType"`
	Upserted     int    `json:"upserted"`
	Removed      int    `json:"removed"`
}

// SyncType reconciles the registry for a single resource type: every item in
// items is upserted, and any previously-known resource of that type that is not
// present in items is soft-deleted. This keeps the inventory consistent with the
// authoritative source on every pass.
func (m *Manager) SyncType(resourceType string, items []Resource) (SyncResult, error) {
	res := SyncResult{ResourceType: resourceType}

	present := make(map[string]bool, len(items))
	for _, it := range items {
		it.ResourceType = resourceType
		saved, err := m.store.UpsertResource(it)
		if err != nil {
			return res, fmt.Errorf("resourcemon: sync %s/%s: %w", resourceType, it.Name, err)
		}
		present[saved.ID] = true
		res.Upserted++
	}

	existing, err := m.store.ListResources(ResourceFilter{ResourceType: resourceType, Limit: 100000})
	if err != nil {
		return res, err
	}
	for _, e := range existing {
		if !present[e.ID] {
			if err := m.store.MarkResourceDeleted(e.ID); err != nil {
				return res, err
			}
			res.Removed++
		}
	}
	return res, nil
}

// RecordMetric is a convenience wrapper for ingesting a single sample.
func (m *Manager) RecordMetric(s MetricSample) error { return m.store.InsertSample(s) }

// RecordEvent is a convenience wrapper for appending a resource event.
func (m *Manager) RecordEvent(e ResourceEvent) error { return m.store.RecordEvent(e) }

// PutDesiredConfig records a new desired-config version for a resource and
// computes its hash. Drift status is left for ReconcileDrift once an observed
// config is reported.
func (m *Manager) PutDesiredConfig(resourceID, desiredJSON, actor string) (ResourceConfig, error) {
	return m.store.PutConfigVersion(ResourceConfig{
		ResourceID:        resourceID,
		DesiredConfigJSON: desiredJSON,
		ConfigHash:        HashConfig(desiredJSON),
		DriftStatus:       DriftUnknown,
		CreatedBy:         actor,
	})
}

// ReportObservedConfig records an observed configuration for a resource by
// appending a new version that carries forward the latest desired config and
// computes drift immediately.
func (m *Manager) ReportObservedConfig(resourceID, observedJSON, actor string) (ResourceConfig, DriftResult, error) {
	latest, err := m.store.LatestConfig(resourceID)
	desired := "{}"
	if err == nil {
		desired = latest.DesiredConfigJSON
	}
	drift := DetectDrift(desired, observedJSON)
	cfg, err := m.store.PutConfigVersion(ResourceConfig{
		ResourceID:         resourceID,
		DesiredConfigJSON:  desired,
		ObservedConfigJSON: observedJSON,
		ConfigHash:         HashConfig(desired),
		DriftStatus:        drift.Status,
		DriftReason:        drift.Reason,
		CreatedBy:          actor,
	})
	return cfg, drift, err
}
