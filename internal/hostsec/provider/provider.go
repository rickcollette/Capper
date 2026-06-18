// Package provider exposes the process-wide singleton host-security workers so
// that every caller — the control API, the background reconciler, and the node
// agent — drives each host tool through one exclusive serialized worker. Sharing
// a single instance is what guarantees Capper never runs two concurrent
// fail2ban-client / ufw invocations.
package provider

import (
	"sync"

	"capper/internal/hostsec/fail2ban"
	"capper/internal/hostsec/ufw"
)

var (
	once sync.Once
	f2b  *fail2ban.Worker
	uf   *ufw.Worker
)

func initAll() {
	f2b = fail2ban.New()
	uf = ufw.New()
}

// Fail2ban returns the process-wide fail2ban worker.
func Fail2ban() *fail2ban.Worker {
	once.Do(initAll)
	return f2b
}

// UFW returns the process-wide UFW worker.
func UFW() *ufw.Worker {
	once.Do(initAll)
	return uf
}
