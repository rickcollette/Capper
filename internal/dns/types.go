package dns

const (
	ZoneTypePrivate = "private"
	ZoneTypeSystem  = "system"

	RecordSourceManual   = "manual"
	RecordSourceInstance = "instance"
	RecordSourceSystem   = "system"
	RecordSourceService  = "service"
	RecordSourceLB       = "lb"

	RecordTypeA     = "A"
	RecordTypeAAAA  = "AAAA"
	RecordTypeCNAME = "CNAME"
	RecordTypeTXT   = "TXT"
	RecordTypeSRV   = "SRV"
	RecordTypePTR   = "PTR"
	RecordTypeMX    = "MX"

	SelectorTypeLabel = "label"
	SelectorTypeImage = "image"

	RoutingSimple     = "simple"
	RoutingMultivalue = "multivalue"
	RoutingWeighted   = "weighted"

	// Topology-aware routing policies (Phase 5).
	RoutingZoneLocal    = "zone-local"
	RoutingRegionLocal  = "region-local"
	RoutingRealmFailover = "realm-failover"
	RoutingLatency      = "latency"
	RoutingHealthAware  = "health-aware"
	RoutingDrainAware   = "drain-aware"
)

// Zone is a DNS namespace managed by Capper.
type Zone struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	NetworkID   string            `json:"networkID,omitempty"`
	DefaultTTL  int               `json:"defaultTTL"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   string            `json:"createdAt"`
}

// Record is a single DNS record in a zone.
type Record struct {
	ID        string   `json:"id"`
	ZoneID    string   `json:"zoneID"`
	Name      string   `json:"name"`
	FQDN      string   `json:"fqdn"`
	Type      string   `json:"type"`
	Values    []string `json:"values"`
	TTL       int      `json:"ttl"`
	Source    string   `json:"source"`
	Enabled   bool     `json:"enabled"`
	Weight    int      `json:"weight,omitempty"`
	Priority  int      `json:"priority,omitempty"`
	CreatedAt string   `json:"createdAt"`
}

// ServiceRecord is a selector-backed dynamic record that resolves to
// instance IPs matching the selector at query time.
type ServiceRecord struct {
	ID            string `json:"id"`
	ZoneID        string `json:"zoneID"`
	NetworkID     string `json:"networkID"`
	Name          string `json:"name"`
	FQDN          string `json:"fqdn"`
	SelectorType  string `json:"selectorType"`
	SelectorKey   string `json:"selectorKey,omitempty"`
	SelectorValue string `json:"selectorValue"`
	Protocol      string `json:"protocol"`
	Port          int    `json:"port"`
	TTL           int    `json:"ttl"`
	HealthSource  string `json:"healthSource,omitempty"`
	RoutingPolicy string `json:"routingPolicy"`
	CreatedAt     string `json:"createdAt"`
}

// Forwarder configures upstream DNS servers for a network or globally.
type Forwarder struct {
	ID        string   `json:"id"`
	NetworkID string   `json:"networkID,omitempty"`
	Upstreams []string `json:"upstreams"`
	Enabled   bool     `json:"enabled"`
	CreatedAt string   `json:"createdAt"`
}
