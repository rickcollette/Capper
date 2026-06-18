package resourcemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
)

// HashConfig returns a stable SHA-256 hash of a JSON config document. Keys are
// canonicalized by round-tripping through a map so key order does not affect
// the hash.
func HashConfig(configJSON string) string {
	canonical := canonicalJSON(configJSON)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func canonicalJSON(s string) string {
	if s == "" {
		return "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.Marshal(v) // Go marshals map keys in sorted order.
	if err != nil {
		return s
	}
	return string(b)
}

// DriftResult describes the outcome of comparing desired vs observed config.
type DriftResult struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// DetectDrift compares desired and observed configuration JSON and classifies
// the drift. An empty observed config is treated as "unknown" (not yet
// reported), not as drift.
func DetectDrift(desiredJSON, observedJSON string) DriftResult {
	if canonicalJSON(observedJSON) == "{}" {
		return DriftResult{Status: DriftUnknown, Reason: "no observed configuration reported yet"}
	}
	var desired, observed map[string]any
	if err := json.Unmarshal([]byte(jsonOrEmpty(desiredJSON)), &desired); err != nil {
		return DriftResult{Status: DriftErrored, Reason: "invalid desired config: " + err.Error()}
	}
	if err := json.Unmarshal([]byte(jsonOrEmpty(observedJSON)), &observed); err != nil {
		return DriftResult{Status: DriftErrored, Reason: "invalid observed config: " + err.Error()}
	}
	if reflect.DeepEqual(desired, observed) {
		return DriftResult{Status: DriftInSync}
	}
	diffs := diffKeys(desired, observed)
	return DriftResult{Status: DriftDrifted, Reason: fmt.Sprintf("fields differ: %v", diffs)}
}

// diffKeys returns the set of top-level keys whose values differ between desired
// and observed (including keys present in only one).
func diffKeys(desired, observed map[string]any) []string {
	seen := map[string]bool{}
	var diffs []string
	for k, dv := range desired {
		if ov, ok := observed[k]; !ok || !reflect.DeepEqual(dv, ov) {
			if !seen[k] {
				diffs = append(diffs, k)
				seen[k] = true
			}
		}
	}
	for k := range observed {
		if _, ok := desired[k]; !ok && !seen[k] {
			diffs = append(diffs, k)
			seen[k] = true
		}
	}
	return diffs
}

// ReconcileDrift evaluates drift for a resource's latest config version and
// persists the resulting drift status. Returns the computed result.
func (s *Store) ReconcileDrift(resourceID string) (DriftResult, error) {
	cfg, err := s.LatestConfig(resourceID)
	if err != nil {
		return DriftResult{}, err
	}
	res := DetectDrift(cfg.DesiredConfigJSON, cfg.ObservedConfigJSON)
	if err := s.SetDriftStatus(cfg.ID, res.Status, res.Reason); err != nil {
		return res, err
	}
	return res, nil
}

// RepairDrift records a repair by appending a new config version whose
// last-applied equals the desired config and whose drift is cleared. The actual
// re-application to the live resource is performed by the owning service; this
// records operator intent and resets the drift baseline.
func (s *Store) RepairDrift(resourceID, actor string) (ResourceConfig, error) {
	cfg, err := s.LatestConfig(resourceID)
	if err != nil {
		return ResourceConfig{}, err
	}
	newCfg := ResourceConfig{
		ResourceID:         resourceID,
		DesiredConfigJSON:  cfg.DesiredConfigJSON,
		ObservedConfigJSON: cfg.DesiredConfigJSON,
		LastAppliedJSON:    cfg.DesiredConfigJSON,
		ConfigHash:         HashConfig(cfg.DesiredConfigJSON),
		DriftStatus:        DriftInSync,
		DriftReason:        "repaired by " + actor,
		CreatedBy:          actor,
	}
	return s.PutConfigVersion(newCfg)
}

// ListDrifted returns the latest config version of every resource currently in
// a drifted state.
func (s *Store) ListDrifted() ([]ResourceConfig, error) {
	rows, err := s.db.Query(`SELECT c.id, c.resource_id, c.version, c.desired_config_json,
		c.observed_config_json, c.last_applied_config_json, c.config_hash, c.drift_status,
		c.drift_reason, c.created_by, c.created_at
		FROM rmon_resource_configs c
		JOIN (SELECT resource_id, MAX(version) AS mv FROM rmon_resource_configs GROUP BY resource_id) m
		  ON c.resource_id=m.resource_id AND c.version=m.mv
		WHERE c.drift_status=? ORDER BY c.created_at DESC`, DriftDrifted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceConfig
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
