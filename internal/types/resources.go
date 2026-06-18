package types

type ResourceLimits struct {
	MemoryBytes   int64 `json:"memoryBytes,omitempty"`
	DiskBytes     int64 `json:"diskBytes,omitempty"`
	CPUTimeSecs   int64 `json:"cpuTimeSecs,omitempty"`
	MaxProcesses  int64 `json:"maxProcesses,omitempty"`
	FileSizeBytes int64 `json:"fileSizeBytes,omitempty"`
}

func (r ResourceLimits) Empty() bool {
	return r.MemoryBytes == 0 && r.DiskBytes == 0 && r.CPUTimeSecs == 0 && r.MaxProcesses == 0 && r.FileSizeBytes == 0
}

type ResourceOverrides struct {
	Limits      ResourceLimits
	MemorySet   bool
	DiskSet     bool
	CPUTimeSet  bool
	PidsSet     bool
	FileSizeSet bool
}

func (o ResourceOverrides) Empty() bool {
	return !o.MemorySet && !o.DiskSet && !o.CPUTimeSet && !o.PidsSet && !o.FileSizeSet
}

func (o ResourceOverrides) Apply(base ResourceLimits) ResourceLimits {
	if o.MemorySet {
		base.MemoryBytes = o.Limits.MemoryBytes
	}
	if o.CPUTimeSet {
		base.CPUTimeSecs = o.Limits.CPUTimeSecs
	}
	if o.PidsSet {
		base.MaxProcesses = o.Limits.MaxProcesses
	}
	if o.DiskSet {
		base.DiskBytes = o.Limits.DiskBytes
	}
	if o.FileSizeSet {
		base.FileSizeBytes = o.Limits.FileSizeBytes
	}
	return base
}
