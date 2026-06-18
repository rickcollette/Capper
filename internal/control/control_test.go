package control_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"capper/internal/control"
)

// ---- event bus tests --------------------------------------------------------

// TestBusPublishSubscribe verifies that a subscriber receives a published event.
func TestBusPublishSubscribe(t *testing.T) {
	b := control.NewBus()
	ch := b.Subscribe("instance")

	e := control.Event{Type: "created", ResourceType: "instance", ResourceID: "inst_001"}
	b.Publish(e)

	select {
	case got := <-ch:
		if got.ResourceID != e.ResourceID {
			t.Errorf("got event %q, want %q", got.ResourceID, e.ResourceID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout: did not receive event within 100ms")
	}
}

// TestBusWildcardSubscriber verifies that a "*" subscriber receives all events.
func TestBusWildcardSubscriber(t *testing.T) {
	b := control.NewBus()
	ch := b.Subscribe("*")

	for _, rt := range []string{"instance", "network", "volume"} {
		b.Publish(control.Event{Type: "created", ResourceType: rt, ResourceID: rt + "_1"})
	}

	received := map[string]bool{}
	deadline := time.After(200 * time.Millisecond)
	for len(received) < 3 {
		select {
		case e := <-ch:
			received[e.ResourceType] = true
		case <-deadline:
			t.Fatalf("timeout: only received %v, want 3 types", received)
		}
	}
}

// TestBusNonBlockingOnSlowConsumer verifies that a full subscriber channel
// does not block the publisher.
func TestBusNonBlockingOnSlowConsumer(t *testing.T) {
	b := control.NewBus()
	_ = b.Subscribe("instance") // subscribe but never read = slow consumer

	done := make(chan struct{})
	go func() {
		// Flood with 200 events; bus buffer is 128 — excess must be dropped, not blocked.
		for i := 0; i < 200; i++ {
			b.Publish(control.Event{Type: "x", ResourceType: "instance", ResourceID: fmt.Sprintf("i%d", i)})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Publish blocked on a slow subscriber")
	}
}

// TestBusUnsubscribedTypeIgnored verifies events for unsubscribed types are silently dropped.
func TestBusUnsubscribedTypeIgnored(t *testing.T) {
	b := control.NewBus()
	ch := b.Subscribe("volume") // not subscribing to "network"
	b.Publish(control.Event{Type: "created", ResourceType: "network", ResourceID: "net_1"})

	select {
	case e := <-ch:
		t.Errorf("did not expect a network event on a volume subscriber, got %+v", e)
	case <-time.After(50 * time.Millisecond):
		// correct — nothing received
	}
}

// ---- reconciler loop tests --------------------------------------------------

// countingReconciler increments a counter each time Reconcile is called.
type countingReconciler struct {
	name  string
	count atomic.Int64
}

func (r *countingReconciler) Name() string { return r.name }
func (r *countingReconciler) Reconcile(_ context.Context) error {
	r.count.Add(1)
	return nil
}

// TestReconcilerLoopRunsOnInterval verifies that each reconciler is called at
// least twice within a generous timeout.
func TestReconcilerLoopRunsOnInterval(t *testing.T) {
	loop := control.NewReconcilerLoop(20 * time.Millisecond)
	r := &countingReconciler{name: "test"}
	loop.Register(r)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	loop.Run(ctx) // blocks until ctx cancelled

	if n := r.count.Load(); n < 2 {
		t.Errorf("expected at least 2 reconcile calls, got %d", n)
	}
}

// TestReconcilerLoopCancels verifies Run returns promptly after ctx cancel.
func TestReconcilerLoopCancels(t *testing.T) {
	loop := control.NewReconcilerLoop(1 * time.Second)
	loop.Register(&countingReconciler{name: "slow"})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		loop.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return within 500ms after cancel")
	}
}

// ---- admission chain tests --------------------------------------------------

type allowHook struct{ name string }

func (h *allowHook) Name() string { return h.name }
func (h *allowHook) Admit(_ context.Context, _, _ string) error { return nil }

type denyHook struct{ name string }

func (h *denyHook) Name() string { return h.name }
func (h *denyHook) Admit(_ context.Context, _, _ string) error {
	return fmt.Errorf("%s denied the request", h.name)
}

type recordHook struct {
	name   string
	called bool
}

func (h *recordHook) Name() string { return h.name }
func (h *recordHook) Admit(_ context.Context, _, _ string) error {
	h.called = true
	return nil
}

// TestAdmissionAllPass verifies that all-allow hooks return nil.
func TestAdmissionAllPass(t *testing.T) {
	chain := &control.AdmissionChain{}
	chain.Register(&allowHook{"a"})
	chain.Register(&allowHook{"b"})
	if err := chain.Admit(context.Background(), "instance", "create"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestAdmissionDenyShortCircuits verifies the first deny stops the chain.
func TestAdmissionDenyShortCircuits(t *testing.T) {
	after := &recordHook{name: "after-deny"}
	chain := &control.AdmissionChain{}
	chain.Register(&allowHook{"before"})
	chain.Register(&denyHook{"blocker"})
	chain.Register(after)

	err := chain.Admit(context.Background(), "instance", "delete")
	if err == nil {
		t.Fatal("expected denial error, got nil")
	}
	if after.called {
		t.Error("hook after deny should not have been called")
	}
}

// TestAdmissionEmptyChainPasses verifies no hooks = allow.
func TestAdmissionEmptyChainPasses(t *testing.T) {
	chain := &control.AdmissionChain{}
	if err := chain.Admit(context.Background(), "image", "create"); err != nil {
		t.Errorf("empty chain should allow: %v", err)
	}
}
