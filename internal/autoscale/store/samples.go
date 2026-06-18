package autoscalestore

import (
	"database/sql"
	"encoding/json"
	"time"

	"capper/internal/autoscale"
)

// SampleStore persists and queries metric samples.
type SampleStore struct {
	db *sql.DB
}

func NewSampleStore(db *sql.DB) *SampleStore { return &SampleStore{db: db} }

func (s *SampleStore) Insert(m autoscale.MetricSample) error {
	if m.SampledAt == "" {
		m.SampledAt = time.Now().UTC().Format(time.RFC3339)
	}
	labelsJSON, _ := json.Marshal(m.Labels)
	_, err := s.db.Exec(`
		INSERT INTO autoscale_metric_samples
		  (id, project, scope, resource_id, metric_name, value, labels_json, sampled_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		m.ID, m.Project, m.Scope, m.ResourceID, m.MetricName,
		m.Value, string(labelsJSON), m.SampledAt,
	)
	return err
}

// Average returns the mean value of a metric for a resource within the past windowSecs.
func (s *SampleStore) Average(scope, resourceID, metricName string, windowSecs int) (float64, bool) {
	cutoff := time.Now().Add(-time.Duration(windowSecs) * time.Second).UTC().Format(time.RFC3339)
	row := s.db.QueryRow(`
		SELECT AVG(value), COUNT(*)
		FROM autoscale_metric_samples
		WHERE scope=? AND resource_id=? AND metric_name=? AND sampled_at >= ?`,
		scope, resourceID, metricName, cutoff,
	)
	var avg float64
	var count int
	if err := row.Scan(&avg, &count); err != nil || count == 0 {
		return 0, false
	}
	return avg, true
}

// Latest returns the most recent sample value for a resource/metric.
func (s *SampleStore) Latest(scope, resourceID, metricName string) (float64, bool) {
	row := s.db.QueryRow(`
		SELECT value FROM autoscale_metric_samples
		WHERE scope=? AND resource_id=? AND metric_name=?
		ORDER BY sampled_at DESC LIMIT 1`,
		scope, resourceID, metricName,
	)
	var v float64
	if err := row.Scan(&v); err != nil {
		return 0, false
	}
	return v, true
}

// Prune removes samples older than maxAgeSecs.
func (s *SampleStore) Prune(maxAgeSecs int) (int, error) {
	cutoff := time.Now().Add(-time.Duration(maxAgeSecs) * time.Second).UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`DELETE FROM autoscale_metric_samples WHERE sampled_at < ?`, cutoff,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
