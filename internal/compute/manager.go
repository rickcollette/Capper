package compute

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Manager provides high-level compute lifecycle operations.
type Manager struct {
	store *Store
}

// NewManager returns a Manager backed by the given store.
func NewManager(s *Store) *Manager {
	return &Manager{store: s}
}

// ---- hosts ------------------------------------------------------------------

// RegisterLocal creates or updates the "local" host entry using runtime-detected
// CPU and memory info where available.
func (m *Manager) RegisterLocal() (Host, error) {
	h := Host{
		ID:          "host_local",
		Name:        "local",
		Address:     "127.0.0.1",
		Status:      HostStatusReady,
		Labels:      map[string]string{"type": "local"},
		CPUCount:    runtime.NumCPU(),
		MemoryBytes: 0, // filled by caller from /proc/meminfo if needed
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.UpsertHost(h); err != nil {
		return Host{}, err
	}
	return h, nil
}

// GetHost returns a host by name or ID.
func (m *Manager) GetHost(nameOrID string) (Host, error) {
	h, err := m.store.GetHost(nameOrID)
	if err != nil {
		return Host{}, fmt.Errorf("compute: host %q not found: %w", nameOrID, err)
	}
	return h, nil
}

// ListHosts returns all registered hosts.
func (m *Manager) ListHosts() ([]Host, error) {
	return m.store.ListHosts()
}

// DrainHost prevents new instances from being scheduled on a host.
func (m *Manager) DrainHost(nameOrID string) error {
	return m.store.UpdateHostStatus(nameOrID, HostStatusDrained, time.Now().UTC().Format(time.RFC3339))
}

// UncordonHost marks a host ready again.
func (m *Manager) UncordonHost(nameOrID string) error {
	return m.store.UpdateHostStatus(nameOrID, HostStatusReady, time.Now().UTC().Format(time.RFC3339))
}

// ---- templates --------------------------------------------------------------

// CreateTemplate stores a new instance template. Returns an error if the name
// already exists.
func (m *Manager) CreateTemplate(doc TemplateDoc) (Template, error) {
	if doc.Name == "" {
		return Template{}, fmt.Errorf("compute: template name is required")
	}
	if doc.Image == "" {
		return Template{}, fmt.Errorf("compute: template image is required")
	}
	t := Template{
		ID:        newID("tmpl"),
		Name:      strings.ToLower(doc.Name),
		Image:     doc.Image,
		Runtime:   doc.Runtime,
		Doc:       doc,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.InsertTemplate(t); err != nil {
		return Template{}, err
	}
	return t, nil
}

// GetTemplate returns a template by name or ID.
func (m *Manager) GetTemplate(nameOrID string) (Template, error) {
	t, err := m.store.GetTemplate(nameOrID)
	if err != nil {
		return Template{}, fmt.Errorf("compute: template %q not found: %w", nameOrID, err)
	}
	return t, nil
}

// ListTemplates returns all templates.
func (m *Manager) ListTemplates() ([]Template, error) {
	return m.store.ListTemplates()
}

// DeleteTemplate removes a template. Returns an error if any group references it.
func (m *Manager) DeleteTemplate(nameOrID string) error {
	t, err := m.store.GetTemplate(nameOrID)
	if err != nil {
		return fmt.Errorf("compute: template %q not found", nameOrID)
	}
	inUse, err := m.store.TemplateInUse(t.ID)
	if err != nil {
		return err
	}
	if inUse {
		return fmt.Errorf("compute: template %q is referenced by one or more groups; delete the groups first", t.Name)
	}
	return m.store.DeleteTemplate(t.ID)
}

// ---- groups -----------------------------------------------------------------

// CreateGroup creates a new instance group backed by the named template.
func (m *Manager) CreateGroup(name, templateName string, min, desired, max int) (Group, error) {
	if desired < min {
		return Group{}, fmt.Errorf("compute: desired (%d) must be >= min (%d)", desired, min)
	}
	if desired > max {
		return Group{}, fmt.Errorf("compute: desired (%d) must be <= max (%d)", desired, max)
	}
	t, err := m.store.GetTemplate(templateName)
	if err != nil {
		return Group{}, fmt.Errorf("compute: template %q not found", templateName)
	}
	g := Group{
		ID:           newID("grp"),
		Name:         strings.ToLower(name),
		TemplateID:   t.ID,
		TemplateName: t.Name,
		MinSize:      min,
		DesiredSize:  desired,
		MaxSize:      max,
		Status:       GroupStatusActive,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.InsertGroup(g); err != nil {
		return Group{}, err
	}
	return g, nil
}

// GetGroup returns a group by name or ID, with template name populated.
func (m *Manager) GetGroup(nameOrID string) (Group, error) {
	g, err := m.store.GetGroup(nameOrID)
	if err != nil {
		return Group{}, fmt.Errorf("compute: group %q not found: %w", nameOrID, err)
	}
	return g, nil
}

// ListGroups returns all groups.
func (m *Manager) ListGroups() ([]Group, error) {
	return m.store.ListGroups()
}

// ScaleGroup updates the desired replica count for a group.
func (m *Manager) ScaleGroup(nameOrID string, desired int) error {
	g, err := m.store.GetGroup(nameOrID)
	if err != nil {
		return fmt.Errorf("compute: group %q not found", nameOrID)
	}
	if desired < g.MinSize {
		return fmt.Errorf("compute: desired %d is below min %d", desired, g.MinSize)
	}
	if desired > g.MaxSize {
		return fmt.Errorf("compute: desired %d exceeds max %d", desired, g.MaxSize)
	}
	return m.store.UpdateGroupDesired(nameOrID, desired)
}

// DeleteGroup removes a group and its instance membership records.
func (m *Manager) DeleteGroup(nameOrID string) error {
	return m.store.DeleteGroup(nameOrID)
}

// ListGroupInstances returns the instance IDs tracked for a group.
func (m *Manager) ListGroupInstances(nameOrID string) ([]string, error) {
	g, err := m.store.GetGroup(nameOrID)
	if err != nil {
		return nil, fmt.Errorf("compute: group %q not found", nameOrID)
	}
	return m.store.ListGroupInstances(g.ID)
}

// ---- run-from-template ------------------------------------------------------

// RunFromTemplate expands a template into a RunSpec. The caller is responsible
// for actually launching the instance (via InstanceManager.Run or similar) and
// then calling RegisterGroupInstance if the instance belongs to a group.
func (m *Manager) RunFromTemplate(templateName, instanceName string) (RunSpec, error) {
	t, err := m.store.GetTemplate(templateName)
	if err != nil {
		return RunSpec{}, fmt.Errorf("compute: template %q not found", templateName)
	}
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("%s-%s", t.Name, shortID())
	}
	res := t.Doc.Resources
	typeName := t.Doc.InstanceTypeName
	if typeName != "" {
		it, err := m.store.GetInstanceType(typeName)
		if err != nil {
			return RunSpec{}, fmt.Errorf("compute: instance type %q not found for template %q", typeName, templateName)
		}
		if it.MemoryBytes > 0 {
			res.MemoryBytes = it.MemoryBytes
		}
		if it.PIDLimit > 0 {
			res.MaxProcesses = int64(it.PIDLimit)
		}
	}
	return RunSpec{
		TemplateName:     t.Name,
		Image:            t.Image,
		Resources:        res,
		InstanceName:     name,
		InstanceTypeName: typeName,
	}, nil
}

// RegisterGroupInstance records that an instance was launched as part of a group.
func (m *Manager) RegisterGroupInstance(groupNameOrID, instanceID string) error {
	g, err := m.store.GetGroup(groupNameOrID)
	if err != nil {
		return fmt.Errorf("compute: group %q not found", groupNameOrID)
	}
	return m.store.AddGroupInstance(g.ID, instanceID, time.Now().UTC().Format(time.RFC3339))
}

// UnregisterGroupInstance removes an instance from a group's membership records.
func (m *Manager) UnregisterGroupInstance(groupNameOrID, instanceID string) error {
	g, err := m.store.GetGroup(groupNameOrID)
	if err != nil {
		return fmt.Errorf("compute: group %q not found", groupNameOrID)
	}
	return m.store.RemoveGroupInstance(g.ID, instanceID)
}

// ---- reconcile --------------------------------------------------------------

// InstanceStatus is the minimal view of an instance that Reconcile needs in
// order to decide whether an instance is still alive. The caller supplies an
// InstanceStatusFunc so that compute never imports manager or store.
type InstanceStatus struct {
	ID     string
	Status string // "running", "stopped", "failed", …
}

// InstanceStatusFunc returns the current status of an instance by ID, or a
// "not found" error when the instance no longer exists in the store.
type InstanceStatusFunc func(instanceID string) (InstanceStatus, error)

// Reconcile ensures the actual number of running replicas matches the group's
// desired count.  It launches missing replicas using runner and removes stale
// membership records for instances that are no longer alive (but does NOT stop
// or delete running extras — that is left to a future daemon).
func (m *Manager) Reconcile(groupNameOrID string, status InstanceStatusFunc, runner RunFunc) (*ReconcileResult, error) {
	g, err := m.store.GetGroup(groupNameOrID)
	if err != nil {
		return nil, fmt.Errorf("compute: group %q not found", groupNameOrID)
	}
	t, err := m.store.GetTemplate(g.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("compute: template for group %q not found", g.Name)
	}

	result := &ReconcileResult{GroupID: g.ID, Desired: g.DesiredSize}

	// Audit existing membership records.
	tracked, err := m.store.ListGroupInstances(g.ID)
	if err != nil {
		return nil, err
	}
	alive := make([]string, 0, len(tracked))
	for _, instID := range tracked {
		ist, err := status(instID)
		if err != nil || ist.Status == "stopped" || ist.Status == "failed" || ist.Status == "removed" {
			// Instance is gone — clean up membership.
			_ = m.store.RemoveGroupInstance(g.ID, instID)
			if err == nil {
				result.Removed = append(result.Removed, instID)
			}
			continue
		}
		alive = append(alive, instID)
	}
	result.Actual = len(alive)

	// Scale up to reach desired.
	for i := result.Actual; i < g.DesiredSize; i++ {
		instName := fmt.Sprintf("%s-%s", g.Name, shortID())
		instID, err := runner(t.Image, t.Doc.Resources, instName)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("launch %s: %v", instName, err))
			continue
		}
		if addErr := m.store.AddGroupInstance(g.ID, instID, time.Now().UTC().Format(time.RFC3339)); addErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("register %s: %v", instID, addErr))
		}
		result.Created = append(result.Created, instID)
		result.Actual++
	}

	return result, nil
}

// ---- instance types ---------------------------------------------------------

// SeedBuiltinTypes seeds the standard cap-m*, cap-c*, and cap-g* instance types.
// Safe to call multiple times; existing locked types are updated in-place.
func (m *Manager) SeedBuiltinTypes() error {
	now := time.Now().UTC().Format(time.RFC3339)
	types := []InstanceType{
		{ID: "itype_cap_m1", Name: "cap-m1", Family: InstanceTypeFamilyMemory, CPUCount: 1, MemoryBytes: 512 << 20, PIDLimit: 256, Locked: true, Description: "1 CPU · 512MB · tiny tools"},
		{ID: "itype_cap_m2", Name: "cap-m2", Family: InstanceTypeFamilyMemory, CPUCount: 1, MemoryBytes: 1 << 30, PIDLimit: 256, Locked: true, Description: "1 CPU · 1GB · generic shell"},
		{ID: "itype_cap_m3", Name: "cap-m3", Family: InstanceTypeFamilyMemory, CPUCount: 1, MemoryBytes: 2 << 30, PIDLimit: 256, Locked: true, Description: "1 CPU · 2GB · debug"},
		{ID: "itype_cap_m4", Name: "cap-m4", Family: InstanceTypeFamilyMemory, CPUCount: 1, MemoryBytes: 4 << 30, PIDLimit: 256, Locked: true, Description: "1 CPU · 4GB · small services"},
		{ID: "itype_cap_m5", Name: "cap-m5", Family: InstanceTypeFamilyMemory, CPUCount: 1, MemoryBytes: 8 << 30, PIDLimit: 512, Locked: true, Description: "1 CPU · 8GB · memory tasks"},
		{ID: "itype_cap_m6", Name: "cap-m6", Family: InstanceTypeFamilyMemory, CPUCount: 1, MemoryBytes: 16 << 30, PIDLimit: 512, Locked: true, Description: "1 CPU · 16GB · high-memory 1 CPU"},
		{ID: "itype_cap_c1", Name: "cap-c1", Family: InstanceTypeFamilyCompute, CPUCount: 1, MemoryBytes: 1 << 30, PIDLimit: 256, Locked: true, Description: "1 CPU · 1GB · basic compute"},
		{ID: "itype_cap_c2", Name: "cap-c2", Family: InstanceTypeFamilyCompute, CPUCount: 2, MemoryBytes: 2 << 30, PIDLimit: 256, Locked: true, Description: "2 CPU · 2GB · small service"},
		{ID: "itype_cap_c3", Name: "cap-c3", Family: InstanceTypeFamilyCompute, CPUCount: 4, MemoryBytes: 4 << 30, PIDLimit: 512, Locked: true, Description: "4 CPU · 4GB · medium service"},
		{ID: "itype_cap_c4", Name: "cap-c4", Family: InstanceTypeFamilyCompute, CPUCount: 6, MemoryBytes: 8 << 30, PIDLimit: 512, Locked: true, Description: "6 CPU · 8GB · backend/build"},
		{ID: "itype_cap_c5", Name: "cap-c5", Family: InstanceTypeFamilyCompute, CPUCount: 8, MemoryBytes: 16 << 30, PIDLimit: 1024, Locked: true, Description: "8 CPU · 16GB · CPU-heavy"},
		{ID: "itype_cap_g1", Name: "cap-g1", Family: InstanceTypeFamilyGPU, CPUCount: 2, MemoryBytes: 16 << 30, PIDLimit: 512, GPUEligible: true, GPUCount: 1, Locked: true, Description: "2 CPU · 16GB · 1 GPU · small AI GPU"},
		{ID: "itype_cap_g2", Name: "cap-g2", Family: InstanceTypeFamilyGPU, CPUCount: 4, MemoryBytes: 32 << 30, PIDLimit: 512, GPUEligible: true, GPUCount: 1, Locked: true, Description: "4 CPU · 32GB · 1 GPU · larger AI GPU"},
	}
	for i := range types {
		types[i].CreatedAt = now
		if err := m.store.UpsertInstanceType(types[i]); err != nil {
			return fmt.Errorf("seed instance type %s: %w", types[i].Name, err)
		}
	}
	return nil
}

// CreateInstanceType creates a new (custom, non-locked) instance type.
func (m *Manager) CreateInstanceType(it InstanceType) (InstanceType, error) {
	if it.Name == "" {
		return InstanceType{}, fmt.Errorf("compute: instance type name is required")
	}
	it.ID = newID("itype")
	it.Locked = false
	it.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.store.UpsertInstanceType(it); err != nil {
		return InstanceType{}, err
	}
	return it, nil
}

// GetInstanceType fetches an instance type by name or ID.
func (m *Manager) GetInstanceType(nameOrID string) (InstanceType, error) {
	return m.store.GetInstanceType(nameOrID)
}

// ListInstanceTypes returns all instance types ordered by family, name.
func (m *Manager) ListInstanceTypes() ([]InstanceType, error) {
	return m.store.ListInstanceTypes()
}

// DeleteInstanceType removes a non-locked instance type.
func (m *Manager) DeleteInstanceType(nameOrID string) error {
	return m.store.DeleteInstanceType(nameOrID)
}

// DeprecateInstanceType marks an instance type as deprecated and locks it.
func (m *Manager) DeprecateInstanceType(nameOrID string) (InstanceType, error) {
	return m.store.DeprecateInstanceType(nameOrID)
}

// ---- GPU devices ------------------------------------------------------------

// RegisterGPU adds or updates a GPU device record.
func (m *Manager) RegisterGPU(g GPUDevice) (GPUDevice, error) {
	if g.ID == "" {
		g.ID = newID("gpu")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if g.CreatedAt == "" {
		g.CreatedAt = now
	}
	g.UpdatedAt = now
	if g.Status == "" {
		g.Status = GPUStatusAvailable
	}
	if err := m.store.UpsertGPUDevice(g); err != nil {
		return GPUDevice{}, err
	}
	return g, nil
}

// GetGPUDevice returns a GPU device by ID.
func (m *Manager) GetGPUDevice(id string) (GPUDevice, error) {
	return m.store.GetGPUDevice(id)
}

// ListGPUDevices returns all registered GPU devices.
func (m *Manager) ListGPUDevices() ([]GPUDevice, error) {
	return m.store.ListGPUDevices()
}

// AssignGPU marks a GPU as assigned to the given instance.
func (m *Manager) AssignGPU(gpuID, instanceID string) error {
	g, err := m.store.GetGPUDevice(gpuID)
	if err != nil {
		return err
	}
	if g.Status != GPUStatusAvailable {
		return fmt.Errorf("compute: GPU %q is not available (status: %s)", gpuID, g.Status)
	}
	return m.store.UpdateGPUStatus(gpuID, GPUStatusAssigned, instanceID, time.Now().UTC().Format(time.RFC3339))
}

// ReleaseGPU marks a GPU as available again.
func (m *Manager) ReleaseGPU(gpuID string) error {
	return m.store.UpdateGPUStatus(gpuID, GPUStatusAvailable, "", time.Now().UTC().Format(time.RFC3339))
}

// RemoveGPU removes a GPU device record (only when not assigned).
func (m *Manager) RemoveGPU(gpuID string) error {
	g, err := m.store.GetGPUDevice(gpuID)
	if err != nil {
		return err
	}
	if g.Status == GPUStatusAssigned {
		return fmt.Errorf("compute: GPU %q is assigned to %s; release it first", gpuID, g.AssignedInstanceID)
	}
	return m.store.DeleteGPUDevice(gpuID)
}

// ---- helpers ----------------------------------------------------------------

func newID(prefix string) string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func shortID() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RegisterRemote registers a remote host by address. The host agent is expected
// to be reachable at addr (e.g. "192.168.1.10:7443"). Labels may be used for
// placement constraints.
func (m *Manager) RegisterRemote(addr string, labels map[string]string) (Host, error) {
	if addr == "" {
		return Host{}, fmt.Errorf("compute: address is required for remote host")
	}
	id := "host_" + addr
	h := Host{
		ID:        id,
		Name:      addr,
		Address:   addr,
		Status:    HostStatusReady,
		Labels:    labels,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.UpsertHost(h); err != nil {
		return Host{}, fmt.Errorf("compute: register remote host: %w", err)
	}
	return h, nil
}

// PlaceInstance chooses the best host for a new instance according to placement
// rules: filter by label selectors, then pick the host with the most available
// capacity. Returns the local host if no remote hosts match.
func (m *Manager) PlaceInstance(requiredLabels map[string]string, cpuNeeded int, memNeeded int64) (Host, error) {
	hosts, err := m.store.ListHosts()
	if err != nil {
		return Host{}, fmt.Errorf("compute: list hosts for placement: %w", err)
	}
	var candidates []Host
	for _, h := range hosts {
		if h.Status != HostStatusReady {
			continue
		}
		match := true
		for k, v := range requiredLabels {
			if h.Labels[k] != v {
				match = false
				break
			}
		}
		if match {
			candidates = append(candidates, h)
		}
	}
	if len(candidates) == 0 {
		return Host{}, fmt.Errorf("compute: no hosts match placement constraints %v", requiredLabels)
	}
	// Simple selection: pick host with lowest current load (approximated by
	// CPU count; a real implementation would track running instance counts).
	best := candidates[0]
	for _, h := range candidates[1:] {
		if h.CPUCount > best.CPUCount {
			best = h
		}
	}
	return best, nil
}

// ---- host provisioning (Block 22) ------------------------------------------

// ProvisionResult describes the outcome of an automated host provisioning attempt.
type ProvisionResult struct {
	HostID  string
	Address string
	Status  string // "registered", "failed"
	Logs    string
}

// ProvisionHost registers a remote host record. Automated agent bootstrap is
// intentionally not implied by this method; callers that perform SSH/systemd
// installation should validate that separately before marking a host ready.
func (m *Manager) ProvisionHost(address, sshUser, sshKey string, labels map[string]string) (ProvisionResult, error) {
	h, err := m.RegisterRemote(address, labels)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("compute: provision host: register: %w", err)
	}
	return ProvisionResult{
		HostID:  h.ID,
		Address: address,
		Status:  "registered",
		Logs:    "host registered; no agent bootstrap was run",
	}, nil
}

// InventoryHosts returns all hosts that match the given label selectors.
func (m *Manager) InventoryHosts(selectors map[string]string) ([]Host, error) {
	hosts, err := m.store.ListHosts()
	if err != nil {
		return nil, err
	}
	if len(selectors) == 0 {
		return hosts, nil
	}
	var out []Host
	for _, h := range hosts {
		match := true
		for k, v := range selectors {
			if h.Labels[k] != v {
				match = false
				break
			}
		}
		if match {
			out = append(out, h)
		}
	}
	return out, nil
}
