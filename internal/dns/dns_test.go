package dns

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

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

func TestInitSchemaIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for i := 0; i < 2; i++ {
		if err := InitSchema(db); err != nil {
			t.Fatalf("init %d: %v", i+1, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Store — zones
// ---------------------------------------------------------------------------

func TestStoreZoneInsertAndGet(t *testing.T) {
	s := NewStore(openTestDB(t))
	z := Zone{
		ID: "zone_001", Name: "devnet.cap", Type: ZoneTypePrivate,
		NetworkID: "net_abc", DefaultTTL: 30, CreatedAt: "2024-01-01T00:00:00Z",
	}
	if err := s.InsertZone(z); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetZone("devnet.cap", "net_abc")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got.ID != z.ID {
		t.Errorf("id: got %q want %q", got.ID, z.ID)
	}
	got2, err := s.GetZone("zone_001", "")
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got2.Name != z.Name {
		t.Errorf("name: got %q want %q", got2.Name, z.Name)
	}
}

func TestStoreZoneListAndDelete(t *testing.T) {
	s := NewStore(openTestDB(t))
	for i, name := range []string{"alpha.cap", "beta.cap", "gamma.cap"} {
		s.InsertZone(Zone{
			ID: newID("zone"), Name: name, Type: ZoneTypePrivate,
			DefaultTTL: 30, CreatedAt: "2024-01-01T00:00:00Z",
		})
		_ = i
	}
	zones, err := s.ListZones("")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(zones) != 3 {
		t.Errorf("expected 3 zones, got %d", len(zones))
	}
	if err := s.DeleteZone(zones[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	zones2, _ := s.ListZones("")
	if len(zones2) != 2 {
		t.Errorf("expected 2 zones after delete, got %d", len(zones2))
	}
}

func TestStoreFindZonesForName(t *testing.T) {
	s := NewStore(openTestDB(t))
	s.InsertZone(Zone{ID: "zone_short", Name: "cap", DefaultTTL: 30, CreatedAt: "2024-01-01T00:00:00Z"})
	s.InsertZone(Zone{ID: "zone_long", Name: "devnet.cap", DefaultTTL: 30, CreatedAt: "2024-01-01T00:00:00Z"})

	zones, err := s.FindZonesForName("web01.devnet.cap.")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(zones) == 0 {
		t.Fatal("expected at least one matching zone")
	}
	// Best match (longest suffix) should be devnet.cap
	if zones[0].Name != "devnet.cap" {
		t.Errorf("best match: got %q want %q", zones[0].Name, "devnet.cap")
	}
}

// ---------------------------------------------------------------------------
// Store — records
// ---------------------------------------------------------------------------

func TestStoreRecordInsertAndLookup(t *testing.T) {
	s := NewStore(openTestDB(t))
	zoneID := "zone_rec01"
	s.InsertZone(Zone{ID: zoneID, Name: "test.cap", DefaultTTL: 30, CreatedAt: "2024-01-01T00:00:00Z"})

	r := Record{
		ID: "rec_001", ZoneID: zoneID, Name: "web", FQDN: "web.test.cap.",
		Type: RecordTypeA, Values: []string{"10.1.2.3"}, TTL: 30,
		Source: RecordSourceManual, Enabled: true, CreatedAt: "2024-01-01T00:00:00Z",
	}
	if err := s.InsertRecord(r); err != nil {
		t.Fatalf("insert: %v", err)
	}

	recs, err := s.LookupRecords(zoneID, "web.test.cap.", RecordTypeA)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].Values[0] != "10.1.2.3" {
		t.Errorf("value: got %q want %q", recs[0].Values[0], "10.1.2.3")
	}
}

func TestStoreRecordLookupWildcardType(t *testing.T) {
	s := NewStore(openTestDB(t))
	zoneID := "zone_rec02"
	s.InsertZone(Zone{ID: zoneID, Name: "wild.cap", DefaultTTL: 30, CreatedAt: "2024-01-01T00:00:00Z"})

	s.InsertRecord(Record{
		ID: "rec_a01", ZoneID: zoneID, Name: "svc", FQDN: "svc.wild.cap.",
		Type: RecordTypeA, Values: []string{"10.0.0.1"}, TTL: 10,
		Source: RecordSourceManual, Enabled: true, CreatedAt: "2024-01-01T00:00:00Z",
	})
	s.InsertRecord(Record{
		ID: "rec_txt01", ZoneID: zoneID, Name: "svc", FQDN: "svc.wild.cap.",
		Type: RecordTypeTXT, Values: []string{"v=info"}, TTL: 10,
		Source: RecordSourceManual, Enabled: true, CreatedAt: "2024-01-01T00:00:00Z",
	})

	recs, err := s.LookupRecords(zoneID, "svc.wild.cap.", "*")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("expected 2 records (A+TXT), got %d", len(recs))
	}
}

func TestStoreRecordDelete(t *testing.T) {
	s := NewStore(openTestDB(t))
	zoneID := "zone_del01"
	s.InsertZone(Zone{ID: zoneID, Name: "del.cap", DefaultTTL: 30, CreatedAt: "2024-01-01T00:00:00Z"})

	s.InsertRecord(Record{
		ID: "rec_del01", ZoneID: zoneID, Name: "x", FQDN: "x.del.cap.",
		Type: RecordTypeA, Values: []string{"1.2.3.4"}, TTL: 30,
		Source: RecordSourceManual, Enabled: true, CreatedAt: "2024-01-01T00:00:00Z",
	})
	if err := s.DeleteRecord("rec_del01"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeleteRecord("rec_del01"); err == nil {
		t.Error("expected error deleting non-existent record")
	}
}

// ---------------------------------------------------------------------------
// Store — services
// ---------------------------------------------------------------------------

func TestStoreServiceInsertAndLookup(t *testing.T) {
	s := NewStore(openTestDB(t))
	zoneID := "zone_svc01"
	s.InsertZone(Zone{ID: zoneID, Name: "svc.cap", DefaultTTL: 5, CreatedAt: "2024-01-01T00:00:00Z"})

	svc := ServiceRecord{
		ID: "svc_001", ZoneID: zoneID, NetworkID: "net_abc",
		Name: "web", FQDN: "web.svc.cap.",
		SelectorType: SelectorTypeLabel, SelectorKey: "role", SelectorValue: "web",
		Protocol: "tcp", Port: 8080, TTL: 5, RoutingPolicy: RoutingMultivalue,
		CreatedAt: "2024-01-01T00:00:00Z",
	}
	if err := s.InsertService(svc); err != nil {
		t.Fatalf("insert service: %v", err)
	}

	got, found, err := s.LookupService(zoneID, "web.svc.cap.")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !found {
		t.Fatal("expected service to be found")
	}
	if got.Port != 8080 {
		t.Errorf("port: got %d want 8080", got.Port)
	}
}

func TestStoreDeleteZoneCascades(t *testing.T) {
	s := NewStore(openTestDB(t))
	zoneID := "zone_cas01"
	s.InsertZone(Zone{ID: zoneID, Name: "cascade.cap", DefaultTTL: 30, CreatedAt: "2024-01-01T00:00:00Z"})
	s.InsertRecord(Record{
		ID: "rec_cas01", ZoneID: zoneID, Name: "a", FQDN: "a.cascade.cap.",
		Type: RecordTypeA, Values: []string{"1.2.3.4"}, TTL: 30,
		Source: RecordSourceManual, Enabled: true, CreatedAt: "2024-01-01T00:00:00Z",
	})
	s.InsertService(ServiceRecord{
		ID: "svc_cas01", ZoneID: zoneID, NetworkID: "net_x",
		Name: "web", FQDN: "web.cascade.cap.",
		SelectorType: SelectorTypeLabel, SelectorValue: "web",
		Protocol: "tcp", Port: 80, TTL: 5, RoutingPolicy: RoutingMultivalue,
		CreatedAt: "2024-01-01T00:00:00Z",
	})

	if err := s.DeleteZone(zoneID); err != nil {
		t.Fatalf("delete zone: %v", err)
	}
	recs, _ := s.ListRecords(zoneID)
	if len(recs) != 0 {
		t.Errorf("expected no records after zone delete, got %d", len(recs))
	}
	svcs, _ := s.ListServices(zoneID)
	if len(svcs) != 0 {
		t.Errorf("expected no services after zone delete, got %d", len(svcs))
	}
}

// ---------------------------------------------------------------------------
// Manager
// ---------------------------------------------------------------------------

func TestManagerCreateZoneIdempotent(t *testing.T) {
	mgr := NewManager(NewStore(openTestDB(t)))
	z1, err := mgr.CreateZone("myzone.cap", ZoneTypePrivate, "", 30, "")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	z2, err := mgr.CreateZone("myzone.cap", ZoneTypePrivate, "", 60, "") // different TTL
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if z1.ID != z2.ID {
		t.Errorf("expected same zone on second create: got %q and %q", z1.ID, z2.ID)
	}
	if z2.DefaultTTL != 30 {
		t.Errorf("TTL should not change on idempotent create: got %d", z2.DefaultTTL)
	}
}

func TestManagerCreateAndListRecord(t *testing.T) {
	mgr := NewManager(NewStore(openTestDB(t)))
	mgr.CreateZone("app.cap", ZoneTypePrivate, "", 30, "")

	r, err := mgr.CreateRecord("app.cap", "", "db", RecordTypeA, []string{"10.0.0.1"}, 0)
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	if !strings.Contains(r.FQDN, "db.app.cap") {
		t.Errorf("unexpected FQDN: %s", r.FQDN)
	}

	records, err := mgr.ListRecords("app.cap", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestManagerDeleteRecord(t *testing.T) {
	mgr := NewManager(NewStore(openTestDB(t)))
	mgr.CreateZone("del.cap", ZoneTypePrivate, "", 30, "")
	r, _ := mgr.CreateRecord("del.cap", "", "x", RecordTypeA, []string{"1.2.3.4"}, 0)

	if err := mgr.DeleteRecord("del.cap", "", r.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	records, _ := mgr.ListRecords("del.cap", "")
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestManagerCreateService(t *testing.T) {
	db := openTestDB(t)
	s := NewStore(db)
	mgr := NewManager(s)
	mgr.CreateZone("disco.cap", ZoneTypePrivate, "net_x", 5, "")

	svc, err := mgr.CreateService("web", "disco.cap", "net_x", ServiceOptions{
		SelectorType:  SelectorTypeLabel,
		SelectorKey:   "role",
		SelectorValue: "web",
		Protocol:      "tcp",
		Port:          8080,
		TTL:           5,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if svc.Port != 8080 {
		t.Errorf("port: got %d want 8080", svc.Port)
	}
	if !strings.HasSuffix(svc.FQDN, "disco.cap.") {
		t.Errorf("FQDN should end with zone: %s", svc.FQDN)
	}
}

// ---------------------------------------------------------------------------
// Resolver (pure in-memory, no network)
// ---------------------------------------------------------------------------

func setupResolverZone(t *testing.T) (*Store, Zone) {
	t.Helper()
	s := NewStore(openTestDB(t))
	z := Zone{
		ID: "zone_res01", Name: "lab.cap", Type: ZoneTypePrivate,
		NetworkID: "net_lab", DefaultTTL: 30, CreatedAt: "2024-01-01T00:00:00Z",
	}
	s.InsertZone(z)
	return s, z
}

func TestResolverARecord(t *testing.T) {
	s, z := setupResolverZone(t)
	s.InsertRecord(Record{
		ID: "rec_res_a", ZoneID: z.ID, Name: "web", FQDN: "web.lab.cap.",
		Type: RecordTypeA, Values: []string{"10.9.0.2"}, TTL: 30,
		Source: RecordSourceManual, Enabled: true, CreatedAt: "2024-01-01T00:00:00Z",
	})

	r := NewResolver(s, nil, nil)
	rrs, err := r.Query("web.lab.cap", "A")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rrs) != 1 {
		t.Fatalf("expected 1 RR, got %d", len(rrs))
	}
	if !strings.Contains(rrs[0].String(), "10.9.0.2") {
		t.Errorf("expected 10.9.0.2 in answer: %s", rrs[0])
	}
}

func TestResolverCNAMERecord(t *testing.T) {
	s, z := setupResolverZone(t)
	s.InsertRecord(Record{
		ID: "rec_cname", ZoneID: z.ID, Name: "api", FQDN: "api.lab.cap.",
		Type: RecordTypeCNAME, Values: []string{"web.lab.cap"}, TTL: 30,
		Source: RecordSourceManual, Enabled: true, CreatedAt: "2024-01-01T00:00:00Z",
	})

	r := NewResolver(s, nil, nil)
	rrs, _ := r.Query("api.lab.cap", "CNAME")
	if len(rrs) == 0 {
		t.Fatal("expected CNAME answer, got none")
	}
	if !strings.Contains(rrs[0].String(), "CNAME") {
		t.Errorf("expected CNAME record: %s", rrs[0])
	}
}

func TestResolverTXTRecord(t *testing.T) {
	s, z := setupResolverZone(t)
	s.InsertRecord(Record{
		ID: "rec_txt", ZoneID: z.ID, Name: "info", FQDN: "info.lab.cap.",
		Type: RecordTypeTXT, Values: []string{"env=test", "version=1"}, TTL: 10,
		Source: RecordSourceManual, Enabled: true, CreatedAt: "2024-01-01T00:00:00Z",
	})

	r := NewResolver(s, nil, nil)
	rrs, _ := r.Query("info.lab.cap", "TXT")
	if len(rrs) == 0 {
		t.Fatal("expected TXT answer")
	}
}

func TestResolverNXDOMAIN(t *testing.T) {
	s, _ := setupResolverZone(t)
	r := NewResolver(s, nil, nil)
	rrs, err := r.Query("noexist.lab.cap", "A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rrs) != 0 {
		t.Errorf("expected no answers for NXDOMAIN, got %d", len(rrs))
	}
}

func TestResolverServiceRecord(t *testing.T) {
	s, z := setupResolverZone(t)
	s.InsertService(ServiceRecord{
		ID: "svc_res01", ZoneID: z.ID, NetworkID: "net_lab",
		Name: "web", FQDN: "web.lab.cap.",
		SelectorType: SelectorTypeLabel, SelectorKey: "role", SelectorValue: "web",
		Protocol: "tcp", Port: 8080, TTL: 5, RoutingPolicy: RoutingMultivalue,
		CreatedAt: "2024-01-01T00:00:00Z",
	})

	// labelFunc returns two IPs for role=web
	labelFn := func(networkID, key, value string) []string {
		if networkID == "net_lab" && key == "role" && value == "web" {
			return []string{"10.9.0.10", "10.9.0.11"}
		}
		return nil
	}

	r := NewResolver(s, labelFn, nil)
	rrs, err := r.Query("web.lab.cap", "A")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rrs) != 2 {
		t.Fatalf("expected 2 A records from service, got %d", len(rrs))
	}
}

func TestResolverNoUpstreamReturnsEmpty(t *testing.T) {
	s := NewStore(openTestDB(t)) // empty store, no zones
	r := NewResolver(s, nil, nil)
	rrs, err := r.Query("google.com", "A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rrs) != 0 {
		t.Errorf("expected empty answer with no upstreams configured, got %d RRs", len(rrs))
	}
}

func TestFormatRRs(t *testing.T) {
	out := FormatRRs(nil)
	if out != "(no records)" {
		t.Errorf("FormatRRs(nil) = %q", out)
	}
}
