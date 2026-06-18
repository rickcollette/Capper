package fail2ban

import (
	"context"
	"strings"
	"testing"

	"capper/internal/hostsec"
)

const overallStatus = `Status
|- Number of jail:      2
` + "`" + `- Jail list:   sshd, nginx-http-auth`

const sshdStatus = `Status for the jail: sshd
|- Filter
|  |- Currently failed: 1
|  |- Total failed:     5
|  ` + "`" + `- File list:        /var/log/auth.log
` + "`" + `- Actions
   |- Currently banned: 2
   |- Total banned:     3
   ` + "`" + `- Banned IP list:   1.2.3.4 5.6.7.8`

func fakeWorker(t *testing.T, banCalls *[]string) *Worker {
	t.Helper()
	r := hostsec.NewRunnerFunc("fail2ban-client", func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		switch {
		case len(args) == 1 && args[0] == "status":
			return []byte(overallStatus), nil
		case len(args) == 2 && args[0] == "status":
			return []byte(sshdStatus), nil
		case len(args) == 4 && args[0] == "set":
			if banCalls != nil {
				*banCalls = append(*banCalls, strings.Join(args, " "))
			}
			return []byte("1"), nil
		}
		return []byte(""), nil
	})
	return NewWithRunner(r)
}

func TestStatusParsing(t *testing.T) {
	w := fakeWorker(t, nil)
	st, err := w.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(st.Jails) != 2 {
		t.Fatalf("expected 2 jails, got %d", len(st.Jails))
	}
	sshd := st.Jails[0]
	if sshd.Name != "sshd" || sshd.CurrentlyBanned != 2 || sshd.TotalBanned != 3 {
		t.Fatalf("unexpected sshd jail: %+v", sshd)
	}
	if len(sshd.BannedIPs) != 2 || sshd.BannedIPs[0] != "1.2.3.4" {
		t.Fatalf("unexpected banned IPs: %v", sshd.BannedIPs)
	}
}

func TestBanUnbanInvokesClient(t *testing.T) {
	var calls []string
	w := fakeWorker(t, &calls)
	if err := w.Ban(context.Background(), "sshd", "9.9.9.9"); err != nil {
		t.Fatalf("ban: %v", err)
	}
	if err := w.Unban(context.Background(), "sshd", "9.9.9.9"); err != nil {
		t.Fatalf("unban: %v", err)
	}
	if len(calls) != 2 || calls[0] != "set sshd banip 9.9.9.9" || calls[1] != "set sshd unbanip 9.9.9.9" {
		t.Fatalf("unexpected client calls: %v", calls)
	}
}

func TestBanRequiresArgs(t *testing.T) {
	w := fakeWorker(t, nil)
	if err := w.Ban(context.Background(), "", "1.2.3.4"); err == nil {
		t.Fatal("expected error for empty jail")
	}
}

func TestAllowlistRoundTrip(t *testing.T) {
	var reloaded bool
	r := hostsec.NewRunnerFunc("fail2ban-client", func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "reload" {
			reloaded = true
		}
		return []byte("OK"), nil
	})
	w := NewWithRunner(r)
	w.SetAllowlistPath(t.TempDir() + "/capper-allowlist.local")

	if err := w.SetAllowlist(context.Background(), []string{"203.0.113.0/24", "198.51.100.7"}); err != nil {
		t.Fatalf("set allowlist: %v", err)
	}
	if !reloaded {
		t.Fatal("expected fail2ban reload after setting allowlist")
	}
	got, err := w.GetAllowlist()
	if err != nil {
		t.Fatalf("get allowlist: %v", err)
	}
	// Loopback bases are filtered out of the returned admin list.
	if len(got) != 2 || got[0] != "203.0.113.0/24" || got[1] != "198.51.100.7" {
		t.Fatalf("unexpected allowlist: %v", got)
	}
}

func TestEnsureBansSkipsActive(t *testing.T) {
	var bans []string
	r := hostsec.NewRunnerFunc("fail2ban-client", func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		if len(args) == 2 && args[0] == "status" {
			// sshd already has 1.2.3.4 banned.
			return []byte(sshdStatus), nil
		}
		if len(args) == 4 && args[0] == "set" && args[2] == "banip" {
			bans = append(bans, args[3])
		}
		return []byte("1"), nil
	})
	w := NewWithRunner(r)
	n, err := w.EnsureBans(context.Background(), map[string][]string{"sshd": {"1.2.3.4", "9.9.9.9"}})
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	// 1.2.3.4 is already banned (in sshdStatus); only 9.9.9.9 should be applied.
	if n != 1 || len(bans) != 1 || bans[0] != "9.9.9.9" {
		t.Fatalf("expected only 9.9.9.9 banned, got n=%d bans=%v", n, bans)
	}
}
