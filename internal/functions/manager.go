package functions

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Invoker executes a function's code and returns its output. Implementations
// may run a container, a microVM, or a local process.
type Invoker interface {
	Invoke(ctx context.Context, fn Function, payload []byte) (stdout []byte, err error)
}

// ProcessInvoker runs a function as a one-shot local process: the function's
// Command (or Handler) is executed with the payload written to stdin and stdout
// captured as the result. This is the default runtime for the "process" and
// "container"-less isolation modes and is fully testable without a container
// runtime.
type ProcessInvoker struct{}

// Invoke runs the function's command with payload on stdin.
func (ProcessInvoker) Invoke(ctx context.Context, fn Function, payload []byte) ([]byte, error) {
	args := fn.Command
	if len(args) == 0 && fn.Handler != "" {
		args = []string{fn.Handler}
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("function %q has no command or handler to execute", fn.Name)
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	for k, v := range fn.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if errBuf.Len() > 0 {
			return out.Bytes(), fmt.Errorf("%v: %s", err, errBuf.String())
		}
		return out.Bytes(), err
	}
	return out.Bytes(), nil
}

// Manager orchestrates function lifecycle and invocation.
type Manager struct {
	store   *Store
	invoker Invoker
}

// NewManager wraps a Store with the given invoker (defaults to ProcessInvoker).
func NewManager(store *Store, invoker Invoker) *Manager {
	if invoker == nil {
		invoker = ProcessInvoker{}
	}
	return &Manager{store: store, invoker: invoker}
}

// Store exposes the underlying store.
func (m *Manager) Store() *Store { return m.store }

// InvokeResult is returned from a synchronous invocation.
type InvokeResult struct {
	InvocationID string `json:"invocationId"`
	Status       string `json:"status"`
	DurationMS   int64  `json:"durationMs"`
	Output       string `json:"output,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Invoke runs a function synchronously, recording the invocation lifecycle.
// triggerID and principal are optional provenance fields.
func (m *Manager) Invoke(ctx context.Context, fn Function, payload []byte, triggerID, principal, source string) (InvokeResult, error) {
	inv, err := m.store.StartInvocation(Invocation{
		Project: fn.Project, FunctionID: fn.ID, FunctionVersion: fn.Version,
		TriggerID: triggerID, Principal: principal, Source: source, Status: InvocationRunning,
	})
	if err != nil {
		return InvokeResult{}, err
	}

	timeout := time.Duration(fn.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = DefaultTimeoutMS * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	out, runErr := m.invoker.Invoke(runCtx, fn, payload)
	dur := time.Since(start).Milliseconds()

	status := InvocationSucceeded
	errMsg := ""
	if runErr != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			status = InvocationTimeout
		} else {
			status = InvocationFailed
		}
		errMsg = runErr.Error()
	}
	if err := m.store.FinishInvocation(inv.ID, status, dur, errMsg, string(out)); err != nil {
		return InvokeResult{}, err
	}
	return InvokeResult{
		InvocationID: inv.ID, Status: status, DurationMS: dur,
		Output: string(out), Error: errMsg,
	}, nil
}

// DispatchEvent fans an event out to every enabled trigger bound to (type,
// source), invoking each target function with the event payload. Returns the
// invocation results. Failures of one target do not stop the others.
func (m *Manager) DispatchEvent(ctx context.Context, triggerType, source string, payload []byte) ([]InvokeResult, error) {
	triggers, err := m.store.TriggersBySource(triggerType, source)
	if err != nil {
		return nil, err
	}
	var results []InvokeResult
	for _, t := range triggers {
		fn, err := m.store.GetFunction(t.FunctionID)
		if err != nil {
			continue
		}
		res, err := m.Invoke(ctx, fn, payload, t.ID, "event-router", triggerType+":"+source)
		if err != nil {
			results = append(results, InvokeResult{Status: InvocationFailed, Error: err.Error()})
			continue
		}
		results = append(results, res)
	}
	return results, nil
}
