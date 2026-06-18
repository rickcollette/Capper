package control

import "context"

// AdmissionHook is a single gate in the admission chain. If Admit returns a
// non-nil error the request is rejected with that error; no further hooks run.
type AdmissionHook interface {
	Name() string
	Admit(ctx context.Context, resourceType, action string) error
}

// AdmissionChain runs a sequence of AdmissionHooks in order.
// The first hook to return an error short-circuits the remaining hooks.
type AdmissionChain struct {
	Hooks []AdmissionHook
}

// Register appends h to the chain. Must be called before Admit.
func (c *AdmissionChain) Register(h AdmissionHook) {
	c.Hooks = append(c.Hooks, h)
}

// Admit runs every hook in order. Returns nil only if all hooks pass.
func (c *AdmissionChain) Admit(ctx context.Context, resourceType, action string) error {
	for _, h := range c.Hooks {
		if err := h.Admit(ctx, resourceType, action); err != nil {
			return err
		}
	}
	return nil
}
