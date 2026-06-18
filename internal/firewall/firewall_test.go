package firewall

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testFirewall(networkID, name, mode string) Firewall {
	return Firewall{
		NetworkID:            networkID,
		NetworkName:          name,
		Mode:                 mode,
		Backend:              "nftables",
		DefaultForwardPolicy: ActionDeny,
		DefaultIngressPolicy: ActionDeny,
		DefaultEgressPolicy:  ActionDeny,
		AllowDNS:             true,
		AllowEstablished:     true,
		NATEnabled:           false,
		Status:               StatusPending,
	}
}

func testNetInfo() NetworkInfo {
	return NetworkInfo{
		ID:      "net_aabbccdd11223344",
		Name:    "devnet",
		Subnet:  "10.42.0.0/24",
		Gateway: "10.42.0.1",
		Bridge:  "capbr-devnet",
		Mode:    "nat",
	}
}

// ---------------------------------------------------------------------------
// Store tests
// ---------------------------------------------------------------------------

func TestInitSchemaIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := InitSchema(db); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if err := InitSchema(db); err != nil {
		t.Fatalf("second init: %v", err)
	}
}

func TestStoreInsertAndGet(t *testing.T) {
	s := NewStore(openTestDB(t))
	fw := testFirewall("net_001", "testnet", ModeStrict)
	if err := s.Insert(fw); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.Get(fw.NetworkID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.NetworkName != fw.NetworkName {
		t.Errorf("name: got %q want %q", got.NetworkName, fw.NetworkName)
	}
	if !got.AllowDNS {
		t.Error("AllowDNS should be true")
	}
	if got.NATEnabled {
		t.Error("NATEnabled should be false")
	}
}

func TestStoreInsertDuplicate(t *testing.T) {
	s := NewStore(openTestDB(t))
	fw := testFirewall("net_002", "dupnet", ModeStrict)
	if err := s.Insert(fw); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := s.Insert(fw); err == nil {
		t.Error("expected error on duplicate insert, got nil")
	}
}

func TestStoreUpdateStatus(t *testing.T) {
	s := NewStore(openTestDB(t))
	fw := testFirewall("net_003", "statnet", ModeStrict)
	s.Insert(fw)
	if err := s.UpdateStatus(fw.NetworkID, StatusApplied); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, _ := s.Get(fw.NetworkID)
	if got.Status != StatusApplied {
		t.Errorf("status: got %q want %q", got.Status, StatusApplied)
	}
	if got.LastAppliedAt == "" {
		t.Error("LastAppliedAt should be set after StatusApplied")
	}
}

func TestStoreList(t *testing.T) {
	s := NewStore(openTestDB(t))
	s.Insert(testFirewall("net_101", "alpha", ModeStrict))
	s.Insert(testFirewall("net_102", "beta", ModePermissive))
	fws, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(fws) != 2 {
		t.Errorf("expected 2 firewalls, got %d", len(fws))
	}
}

func TestStoreDeleteCascadesRules(t *testing.T) {
	s := NewStore(openTestDB(t))
	fw := testFirewall("net_del", "delnet", ModeStrict)
	s.Insert(fw)
	s.InsertRule(Rule{
		ID: "fwr_0001", NetworkID: fw.NetworkID, Priority: 100,
		Enabled: true, Action: ActionAllow, Direction: DirectionForward,
		From: Endpoint{Type: EndpointAny}, To: Endpoint{Type: EndpointGateway},
		Protocol: "tcp", CreatedAt: "2024-01-01T00:00:00Z",
	})
	if err := s.Delete(fw.NetworkID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rules, _ := s.ListRules(fw.NetworkID)
	if len(rules) != 0 {
		t.Errorf("expected no rules after firewall delete, got %d", len(rules))
	}
}

// ---------------------------------------------------------------------------
// Rule store tests
// ---------------------------------------------------------------------------

func TestRuleInsertAndGet(t *testing.T) {
	s := NewStore(openTestDB(t))
	fw := testFirewall("net_r01", "rulenet", ModeStrict)
	s.Insert(fw)

	r := Rule{
		ID: "fwr_abcd1234", NetworkID: fw.NetworkID, Priority: 100,
		Enabled: true, Action: ActionAllow, Direction: DirectionForward,
		From:      Endpoint{Type: EndpointLabel, Key: "role", Value: "web"},
		To:        Endpoint{Type: EndpointLabel, Key: "role", Value: "api"},
		Protocol:  "tcp", Ports: []int{8080},
		Description: "web to api", CreatedAt: "2024-01-01T00:00:00Z",
	}
	if err := s.InsertRule(r); err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	got, err := s.GetRule(r.ID)
	if err != nil {
		t.Fatalf("get rule: %v", err)
	}
	if got.From.Key != "role" || got.From.Value != "web" {
		t.Errorf("from endpoint: got %+v", got.From)
	}
	if len(got.Ports) != 1 || got.Ports[0] != 8080 {
		t.Errorf("ports: got %v", got.Ports)
	}
}

func TestRuleListSortedByPriority(t *testing.T) {
	s := NewStore(openTestDB(t))
	fw := testFirewall("net_r02", "prinet", ModeStrict)
	s.Insert(fw)

	for _, pri := range []int{500, 100, 300} {
		s.InsertRule(Rule{
			ID: newRuleID(), NetworkID: fw.NetworkID, Priority: pri,
			Enabled: true, Action: ActionAllow, Direction: DirectionForward,
			From: Endpoint{Type: EndpointAny}, To: Endpoint{Type: EndpointAny},
			Protocol: "any", CreatedAt: "2024-01-01T00:00:00Z",
		})
	}

	rules, err := s.ListRules(fw.NetworkID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0].Priority > rules[1].Priority || rules[1].Priority > rules[2].Priority {
		t.Errorf("rules not sorted: %d %d %d", rules[0].Priority, rules[1].Priority, rules[2].Priority)
	}
}

func TestRuleEnableDisable(t *testing.T) {
	s := NewStore(openTestDB(t))
	fw := testFirewall("net_r03", "tognet", ModeStrict)
	s.Insert(fw)

	r := Rule{
		ID: "fwr_toggle01", NetworkID: fw.NetworkID, Priority: 100,
		Enabled: true, Action: ActionDeny, Direction: DirectionForward,
		From: Endpoint{Type: EndpointAny}, To: Endpoint{Type: EndpointInternet},
		Protocol: "any", CreatedAt: "2024-01-01T00:00:00Z",
	}
	s.InsertRule(r)

	if err := s.SetRuleEnabled(r.ID, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	got, _ := s.GetRule(r.ID)
	if got.Enabled {
		t.Error("expected rule to be disabled")
	}

	if err := s.SetRuleEnabled(r.ID, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	got2, _ := s.GetRule(r.ID)
	if !got2.Enabled {
		t.Error("expected rule to be enabled")
	}
}

func TestRuleDelete(t *testing.T) {
	s := NewStore(openTestDB(t))
	fw := testFirewall("net_r04", "delrnet", ModeStrict)
	s.Insert(fw)

	r := Rule{
		ID: "fwr_delme0001", NetworkID: fw.NetworkID, Priority: 100,
		Enabled: true, Action: ActionAllow, Direction: DirectionForward,
		From: Endpoint{Type: EndpointAny}, To: Endpoint{Type: EndpointAny},
		Protocol: "any", CreatedAt: "2024-01-01T00:00:00Z",
	}
	s.InsertRule(r)
	if err := s.DeleteRule(r.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetRule(r.ID); err == nil {
		t.Error("expected error getting deleted rule")
	}
}

// ---------------------------------------------------------------------------
// Manager tests
// ---------------------------------------------------------------------------

func TestManagerInitIdempotent(t *testing.T) {
	s := NewStore(openTestDB(t))
	mgr := NewManager(s)

	fw1, err := mgr.Init("net_mgr01", "mgrnet", ModeStrict)
	if err != nil {
		t.Fatalf("first init: %v", err)
	}
	fw2, err := mgr.Init("net_mgr01", "mgrnet", ModePermissive) // mode change ignored
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	if fw1.Mode != fw2.Mode {
		t.Errorf("mode changed on second init: %q → %q", fw1.Mode, fw2.Mode)
	}
}

func TestManagerAddAndDeleteRule(t *testing.T) {
	s := NewStore(openTestDB(t))
	mgr := NewManager(s)
	mgr.Init("net_mgr02", "rulenet", ModeStrict)

	r, err := mgr.AddRule("net_mgr02", RuleSpec{
		Action:      ActionAllow,
		Direction:   DirectionForward,
		From:        Endpoint{Type: EndpointLabel, Key: "role", Value: "web"},
		To:          Endpoint{Type: EndpointLabel, Key: "role", Value: "api"},
		Protocol:    "tcp",
		Ports:       []int{8080},
		Description: "web to api",
	})
	if err != nil {
		t.Fatalf("add rule: %v", err)
	}
	if r.Priority != 100 {
		t.Errorf("expected default allow priority 100, got %d", r.Priority)
	}

	if err := mgr.DeleteRule("net_mgr02", r.ID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}
	rules, _ := s.ListRules("net_mgr02")
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after delete, got %d", len(rules))
	}
}

func TestManagerDefaultPriorities(t *testing.T) {
	s := NewStore(openTestDB(t))
	mgr := NewManager(s)
	mgr.Init("net_mgr03", "prinet", ModeStrict)

	allow, _ := mgr.AddRule("net_mgr03", RuleSpec{Action: ActionAllow})
	deny, _ := mgr.AddRule("net_mgr03", RuleSpec{Action: ActionDeny})

	if allow.Priority != 100 {
		t.Errorf("allow default priority: got %d want 100", allow.Priority)
	}
	if deny.Priority != 500 {
		t.Errorf("deny default priority: got %d want 500", deny.Priority)
	}
}

// ---------------------------------------------------------------------------
// Compiler tests (pure logic — no OS calls)
// ---------------------------------------------------------------------------

func TestCompileChainName(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"net_aabbccdd11223344", "capfw_aabbccdd"},
		{"net_0102030405060708", "capfw_01020304"},
		{"shortid", "capfw_shortid"},
	}
	for _, c := range cases {
		got := ChainName(c.id)
		if got != c.want {
			t.Errorf("ChainName(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func TestCompileBasicScript(t *testing.T) {
	net := testNetInfo()
	fw := testFirewall(net.ID, net.Name, ModeStrict)
	fw.AllowEstablished = true
	fw.AllowDNS = true

	script := Compile(CompileInput{
		Firewall: fw,
		Net:      net,
		Rules:    nil,
		LeaseIPs: nil,
		LabelIPs: nil,
	})

	// Must contain table declaration
	if !strings.Contains(script, "add table inet capper") {
		t.Error("script missing 'add table inet capper'")
	}
	// Must contain chain declaration
	chain := ChainName(net.ID)
	if !strings.Contains(script, chain) {
		t.Errorf("script missing chain %q", chain)
	}
	// Must contain flush
	if !strings.Contains(script, "flush chain") {
		t.Error("script missing flush chain")
	}
	// Must contain established/related accept
	if !strings.Contains(script, "established") {
		t.Error("script missing established/related accept")
	}
	// Must contain DNS allow
	if !strings.Contains(script, "udp dport 53") {
		t.Error("script missing DNS allow rule")
	}
	// Must contain default deny
	if !strings.Contains(script, "drop") {
		t.Error("script missing default drop")
	}
}

func TestCompileAllowRuleByInstance(t *testing.T) {
	net := testNetInfo()
	fw := testFirewall(net.ID, net.Name, ModeStrict)
	fw.AllowDNS = false
	fw.AllowEstablished = false

	rules := []Rule{{
		ID: "fwr_test0001", NetworkID: net.ID, Priority: 100,
		Enabled: true, Action: ActionAllow, Direction: DirectionForward,
		From: Endpoint{Type: EndpointInstance, Value: "inst_web"},
		To:   Endpoint{Type: EndpointInstance, Value: "inst_api"},
		Protocol: "tcp", Ports: []int{8080},
		CreatedAt: "2024-01-01T00:00:00Z",
	}}

	leaseIPs := map[string]string{
		"inst_web": "10.42.0.2",
		"inst_api": "10.42.0.3",
	}

	script := Compile(CompileInput{Firewall: fw, Net: net, Rules: rules, LeaseIPs: leaseIPs})

	if !strings.Contains(script, "10.42.0.2") {
		t.Error("script missing source IP 10.42.0.2")
	}
	if !strings.Contains(script, "10.42.0.3") {
		t.Error("script missing dest IP 10.42.0.3")
	}
	if !strings.Contains(script, "tcp dport 8080") {
		t.Error("script missing tcp dport 8080")
	}
	if !strings.Contains(script, "accept") {
		t.Error("script missing accept verdict")
	}
}

func TestCompileDenyRuleByLabel(t *testing.T) {
	net := testNetInfo()
	fw := testFirewall(net.ID, net.Name, ModeStrict)
	fw.AllowDNS = false
	fw.AllowEstablished = false

	rules := []Rule{{
		ID: "fwr_test0002", NetworkID: net.ID, Priority: 500,
		Enabled: true, Action: ActionDeny, Direction: DirectionForward,
		From: Endpoint{Type: EndpointLabel, Key: "role", Value: "web"},
		To:   Endpoint{Type: EndpointInternet},
		Protocol: "any",
		CreatedAt: "2024-01-01T00:00:00Z",
	}}

	labelIPs := map[string][]string{
		"role=web": {"10.42.0.2", "10.42.0.3"},
	}

	script := Compile(CompileInput{Firewall: fw, Net: net, Rules: rules, LabelIPs: labelIPs})

	if !strings.Contains(script, "10.42.0.2") {
		t.Error("script missing web IP set")
	}
	if !strings.Contains(script, "drop") {
		t.Error("script missing drop verdict")
	}
	if !strings.Contains(script, "oifname") {
		t.Error("script missing oifname for internet egress")
	}
}

func TestCompileDisabledRuleSkipped(t *testing.T) {
	net := testNetInfo()
	fw := testFirewall(net.ID, net.Name, ModeStrict)
	fw.AllowDNS = false
	fw.AllowEstablished = false

	rules := []Rule{{
		ID: "fwr_disabled", NetworkID: net.ID, Priority: 100,
		Enabled: false, Action: ActionAllow, Direction: DirectionForward,
		From: Endpoint{Type: EndpointAny}, To: Endpoint{Type: EndpointAny},
		Protocol: "tcp", Ports: []int{9999},
		CreatedAt: "2024-01-01T00:00:00Z",
	}}

	script := Compile(CompileInput{Firewall: fw, Net: net, Rules: rules})

	if strings.Contains(script, "9999") {
		t.Error("disabled rule should not appear in script")
	}
	if !strings.Contains(script, "[disabled]") {
		t.Error("disabled rule should appear as comment")
	}
}

func TestCompileUnresolvedLabelSkipped(t *testing.T) {
	net := testNetInfo()
	fw := testFirewall(net.ID, net.Name, ModeStrict)
	fw.AllowDNS = false
	fw.AllowEstablished = false

	rules := []Rule{{
		ID: "fwr_unres", NetworkID: net.ID, Priority: 100,
		Enabled: true, Action: ActionAllow, Direction: DirectionForward,
		From: Endpoint{Type: EndpointLabel, Key: "role", Value: "noexist"},
		To:   Endpoint{Type: EndpointAny},
		Protocol: "any",
		CreatedAt: "2024-01-01T00:00:00Z",
	}}

	// no LabelIPs provided → rule should be skipped
	script := Compile(CompileInput{Firewall: fw, Net: net, Rules: rules})

	if !strings.Contains(script, "[unresolved]") {
		t.Error("unresolved rule should appear as comment")
	}
}

func TestCompileGatewayCIDREndpoints(t *testing.T) {
	net := testNetInfo()
	fw := testFirewall(net.ID, net.Name, ModeStrict)
	fw.AllowDNS = false
	fw.AllowEstablished = false

	rules := []Rule{
		{
			ID: "fwr_gw", NetworkID: net.ID, Priority: 100,
			Enabled: true, Action: ActionAllow, Direction: DirectionForward,
			From: Endpoint{Type: EndpointAny},
			To:   Endpoint{Type: EndpointGateway},
			Protocol: "udp", Ports: []int{53},
			CreatedAt: "2024-01-01T00:00:00Z",
		},
		{
			ID: "fwr_cidr", NetworkID: net.ID, Priority: 200,
			Enabled: true, Action: ActionDeny, Direction: DirectionForward,
			From: Endpoint{Type: EndpointAny},
			To:   Endpoint{Type: EndpointCIDR, Value: "192.168.1.0/24"},
			Protocol: "any",
			CreatedAt: "2024-01-01T00:00:00Z",
		},
	}

	script := Compile(CompileInput{Firewall: fw, Net: net, Rules: rules})

	if !strings.Contains(script, net.Gateway) {
		t.Errorf("script missing gateway IP %s", net.Gateway)
	}
	if !strings.Contains(script, "192.168.1.0/24") {
		t.Error("script missing CIDR 192.168.1.0/24")
	}
}

func TestCompileMultiPort(t *testing.T) {
	net := testNetInfo()
	fw := testFirewall(net.ID, net.Name, ModeStrict)
	fw.AllowDNS = false
	fw.AllowEstablished = false

	rules := []Rule{{
		ID: "fwr_mp", NetworkID: net.ID, Priority: 100,
		Enabled: true, Action: ActionAllow, Direction: DirectionForward,
		From: Endpoint{Type: EndpointAny}, To: Endpoint{Type: EndpointNetwork},
		Protocol: "tcp", Ports: []int{80, 443, 8080},
		CreatedAt: "2024-01-01T00:00:00Z",
	}}

	script := Compile(CompileInput{Firewall: fw, Net: net, Rules: rules})

	if !strings.Contains(script, "{ 80, 443, 8080 }") {
		t.Errorf("script missing multi-port set; got:\n%s", script)
	}
}
