package autoscalestore

import "database/sql"

// Store aggregates all autoscale sub-stores.
type Store struct {
	Policies  *PolicyStore
	Decisions *DecisionStore
	Samples   *SampleStore
}

// New creates a Store backed by db. Call InitSchema first.
func New(db *sql.DB) *Store {
	return &Store{
		Policies:  NewPolicyStore(db),
		Decisions: NewDecisionStore(db),
		Samples:   NewSampleStore(db),
	}
}
