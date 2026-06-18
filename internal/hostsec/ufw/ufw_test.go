package ufw

import (
	"context"
	"strings"
	"testing"

	"capper/internal/hostsec"
)

const statusNumbered = `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     ALLOW IN    Anywhere
[ 2] 80/tcp                     ALLOW IN    203.0.113.0/24
[ 3] 3306                       DENY IN     Anywhere`

func TestStatusParsing(t *testing.T) {
	r := hostsec.NewRunnerFunc("ufw", func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		return []byte(statusNumbered), nil
	})
	w := NewWithRunner(r)
	st, err := w.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !st.Enabled {
		t.Fatal("expected enabled")
	}
	if len(st.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(st.Rules))
	}
	if st.Rules[0].Num != 1 || st.Rules[0].Action != "ALLOW" || st.Rules[0].To != "22/tcp" {
		t.Fatalf("unexpected rule 1: %+v", st.Rules[0])
	}
	if st.Rules[2].Action != "DENY" {
		t.Fatalf("unexpected rule 3 action: %+v", st.Rules[2])
	}
}

func TestAddRuleArgs(t *testing.T) {
	var got []string
	r := hostsec.NewRunnerFunc("ufw", func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		got = args
		return []byte("Rule added"), nil
	})
	w := NewWithRunner(r)
	if err := w.AddRule(context.Background(), AddRuleOptions{Action: "allow", Port: "443", Proto: "tcp", Comment: "capper"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	joined := strings.Join(got, " ")
	if joined != "allow 443/tcp comment capper" {
		t.Fatalf("unexpected args: %q", joined)
	}
}

func TestDeleteRuleForce(t *testing.T) {
	var got []string
	r := hostsec.NewRunnerFunc("ufw", func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		got = args
		return []byte("Deleting"), nil
	})
	w := NewWithRunner(r)
	if err := w.DeleteRule(context.Background(), 2); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if strings.Join(got, " ") != "--force delete 2" {
		t.Fatalf("unexpected args: %v", got)
	}
}

func TestInvalidAction(t *testing.T) {
	r := hostsec.NewRunnerFunc("ufw", func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		return nil, nil
	})
	w := NewWithRunner(r)
	if err := w.AddRule(context.Background(), AddRuleOptions{Action: "bogus", Port: "22"}); err == nil {
		t.Fatal("expected invalid action error")
	}
}

const statusVerbose = `Status: active
Logging: on (low)
Default: deny (incoming), allow (outgoing), disabled (routed)
New profiles: skip`

func TestParseDefaults(t *testing.T) {
	r := hostsec.NewRunnerFunc("ufw", func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		return []byte(statusVerbose), nil
	})
	w := NewWithRunner(r)
	d, err := w.GetDefaults(context.Background())
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if d.Incoming != "deny" || d.Outgoing != "allow" || d.Routed != "disabled" {
		t.Fatalf("unexpected defaults: %+v", d)
	}
}

func TestSetDefaultArgs(t *testing.T) {
	var got []string
	r := hostsec.NewRunnerFunc("ufw", func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		got = args
		return []byte("ok"), nil
	})
	w := NewWithRunner(r)
	if err := w.SetDefault(context.Background(), "incoming", "deny"); err != nil {
		t.Fatalf("set default: %v", err)
	}
	if strings.Join(got, " ") != "default deny incoming" {
		t.Fatalf("unexpected args: %v", got)
	}
	if err := w.SetDefault(context.Background(), "bogus", "deny"); err == nil {
		t.Fatal("expected invalid direction error")
	}
}
