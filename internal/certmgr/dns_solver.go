package certmgr

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"time"
)

// DNSSolver is the interface all DNS challenge providers implement.
type DNSSolver interface {
	Present(ctx context.Context, domain, token, keyAuth string) error
	CleanUp(ctx context.Context, domain, token, keyAuth string) error
	WaitForPropagation(ctx context.Context, fqdn, expectedValue string) error
}

// ManualDNSSolver prompts the operator to create the TXT record manually.
type ManualDNSSolver struct{}

func (m *ManualDNSSolver) Present(ctx context.Context, domain, token, keyAuth string) error {
	keyAuthSum := sha256KeyAuth(keyAuth)
	fmt.Printf("\nCreate TXT record: _acme-challenge.%s → %s\nPress Enter when done...\n", domain, keyAuthSum)
	var buf string
	fmt.Scanln(&buf)
	return nil
}

func (m *ManualDNSSolver) CleanUp(ctx context.Context, domain, token, keyAuth string) error {
	fmt.Printf("Delete TXT record: _acme-challenge.%s\n", domain)
	return nil
}

func (m *ManualDNSSolver) WaitForPropagation(ctx context.Context, fqdn, expectedValue string) error {
	deadline := time.Now().Add(120 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		records, err := net.DefaultResolver.LookupTXT(ctx, fqdn)
		if err == nil {
			for _, r := range records {
				if r == expectedValue {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("DNS propagation timeout")
		}
		time.Sleep(5 * time.Second)
	}
}

func sha256KeyAuth(keyAuth string) string {
	sum := sha256.Sum256([]byte(keyAuth))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
