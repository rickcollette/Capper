// Package hostsec provides admin control over host-OS security tooling
// (fail2ban, UFW). Each tool is owned by a single serialized worker so Capper
// never issues concurrent invocations of the same host command.
package hostsec

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

// Runner owns exclusive, serialized access to one host CLI tool. All commands
// flow through a single worker goroutine via a channel, so two admin requests
// can never run the underlying tool concurrently.
type Runner struct {
	bin  string
	jobs chan job
	once sync.Once

	// exec and avail are overridable in tests; they default to running the real
	// binary and looking it up on PATH respectively.
	exec  func(ctx context.Context, bin string, args ...string) ([]byte, error)
	avail func() bool
}

type job struct {
	ctx  context.Context
	args []string
	resp chan result
}

type result struct {
	out []byte
	err error
}

// NewRunner returns a Runner for the named binary (looked up on PATH).
func NewRunner(bin string) *Runner {
	r := &Runner{bin: bin, jobs: make(chan job, 64), exec: defaultExec}
	r.avail = func() bool {
		_, err := exec.LookPath(r.bin)
		return err == nil
	}
	return r
}

// NewRunnerFunc returns a Runner whose execution is provided by fn. It is used
// in tests to drive workers without invoking the real host tool.
func NewRunnerFunc(bin string, fn func(ctx context.Context, bin string, args ...string) ([]byte, error)) *Runner {
	return &Runner{bin: bin, jobs: make(chan job, 64), exec: fn, avail: func() bool { return true }}
}

func defaultExec(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return exec.CommandContext(cctx, bin, args...).CombinedOutput()
}

// start launches the single worker goroutine (once).
func (r *Runner) start() {
	r.once.Do(func() {
		go func() {
			for j := range r.jobs {
				out, err := r.exec(j.ctx, r.bin, j.args...)
				j.resp <- result{out: out, err: err}
			}
		}()
	})
}

// Run executes the tool with args, serialized behind the worker. The combined
// stdout+stderr is returned.
func (r *Runner) Run(ctx context.Context, args ...string) ([]byte, error) {
	r.start()
	resp := make(chan result, 1)
	select {
	case r.jobs <- job{ctx: ctx, args: args, resp: resp}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case res := <-resp:
		return res.out, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Available reports whether the tool is installed (on PATH).
func (r *Runner) Available() bool {
	if r.avail != nil {
		return r.avail()
	}
	_, err := exec.LookPath(r.bin)
	return err == nil
}

// Binary returns the tool's binary name.
func (r *Runner) Binary() string { return r.bin }
