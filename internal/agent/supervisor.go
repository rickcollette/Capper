package agent

import (
	"os/exec"
	"sync"
	"time"
)

// ServiceState holds the runtime state of a supervised service.
type ServiceState struct {
	Name    string    `json:"name"`
	Desired string    `json:"desired"` // "running" | "stopped"
	Actual  string    `json:"actual"`  // "running" | "stopped" | "failed"
	PID     int       `json:"pid,omitempty"`
	Since   time.Time `json:"since"`
}

// Supervisor watches service desired states and manages child processes.
type Supervisor struct {
	mu       sync.Mutex
	services map[string]*ServiceState
	cmds     map[string]*exec.Cmd
}

// NewSupervisor creates an empty Supervisor.
func NewSupervisor() *Supervisor {
	return &Supervisor{
		services: make(map[string]*ServiceState),
		cmds:     make(map[string]*exec.Cmd),
	}
}

// SetDesired updates the desired state for a service and reconciles.
func (s *Supervisor) SetDesired(name, desired string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.services[name]
	if !ok {
		st = &ServiceState{Name: name, Actual: "stopped"}
		s.services[name] = st
	}
	st.Desired = desired
}

// ServiceStates returns a snapshot of all service states.
func (s *Supervisor) ServiceStates() []ServiceState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ServiceState, 0, len(s.services))
	for _, st := range s.services {
		// Refresh actual status by checking if the process is alive.
		if cmd, ok := s.cmds[st.Name]; ok && cmd.Process != nil {
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				st.Actual = "stopped"
			} else {
				st.Actual = "running"
				st.PID = cmd.Process.Pid
			}
		}
		out = append(out, *st)
	}
	return out
}

// Inventory returns a map of service name → actual state for heartbeat reporting.
func (s *Supervisor) Inventory() map[string]string {
	inv := make(map[string]string)
	for _, st := range s.ServiceStates() {
		inv[st.Name] = st.Actual
	}
	return inv
}
