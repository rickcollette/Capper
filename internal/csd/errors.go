package csd

import "errors"

var (
	ErrNotFound      = errors.New("csd: not found")
	ErrAlreadyExists = errors.New("csd: already exists")
	ErrStaleLease    = errors.New("csd: stale lease")
	ErrStaleEpoch    = errors.New("csd: stale epoch")
	ErrLeaseConflict = errors.New("csd: lease conflict")
	ErrNoQuorum      = errors.New("csd: no quorum")
	ErrAccessDenied  = errors.New("csd: access denied")
	ErrVolumeActive  = errors.New("csd: volume has active attachments")
	ErrReadonly      = errors.New("csd: volume is readonly")
	ErrSizeExceeded  = errors.New("csd: size quota exceeded")
)
