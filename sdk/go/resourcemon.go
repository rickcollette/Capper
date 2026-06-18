package cappersdk

import (
	"context"
	"net/url"
)

// ResourceMonAPI accesses the Capper Resource Monitor (capper-observe):
// unified inventory, metrics, events, drift, and alerts.
type ResourceMonAPI struct{ c *Client }

// Resource is one entry in the unified inventory.
type Resource struct {
	ID           string            `json:"id"`
	ResourceType string            `json:"resourceType"`
	Name         string            `json:"name"`
	Project      string            `json:"project"`
	Status       string            `json:"status"`
	Health       string            `json:"health"`
	NodeID       string            `json:"nodeId"`
	Labels       map[string]string `json:"labels"`
	CreatedAt    string            `json:"createdAt"`
}

// MetricSample is a single time-series point.
type MetricSample struct {
	ResourceType string  `json:"resourceType"`
	ResourceID   string  `json:"resourceId"`
	MetricName   string  `json:"metricName"`
	Value        float64 `json:"value"`
	Unit         string  `json:"unit"`
	SampledAt    string  `json:"sampledAt"`
}

// Alert is an open/acknowledged/resolved alert.
type Alert struct {
	ID           string `json:"id"`
	RuleID       string `json:"ruleId"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	Severity     string `json:"severity"`
	Status       string `json:"status"`
	Title        string `json:"title"`
	Message      string `json:"message"`
	OpenedAt     string `json:"openedAt"`
}

// ListResources returns the inventory, optionally filtered by type/project.
func (a *ResourceMonAPI) ListResources(ctx context.Context, resourceType, project string) ([]Resource, error) {
	q := url.Values{}
	if resourceType != "" {
		q.Set("type", resourceType)
	}
	if project != "" {
		q.Set("project", project)
	}
	path := "resources"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out struct {
		Data []Resource `json:"data"`
	}
	return out.Data, a.c.get(ctx, path, &out)
}

// GetResource returns a single resource by ID.
func (a *ResourceMonAPI) GetResource(ctx context.Context, id string) (Resource, error) {
	var out struct {
		Data Resource `json:"data"`
	}
	return out.Data, a.c.get(ctx, "resources/"+id, &out)
}

// Sync projects live resources into the inventory.
func (a *ResourceMonAPI) Sync(ctx context.Context) error {
	return a.c.post(ctx, "resources/sync", nil, nil)
}

// IngestMetric records a single metric sample.
func (a *ResourceMonAPI) IngestMetric(ctx context.Context, m MetricSample) error {
	return a.c.post(ctx, "metrics/ingest", m, nil)
}

// QueryMetrics returns samples for a resource/metric over an optional range.
func (a *ResourceMonAPI) QueryMetrics(ctx context.Context, resourceType, resourceID, metric, rng string) ([]MetricSample, error) {
	q := url.Values{}
	q.Set("resourceType", resourceType)
	q.Set("resourceId", resourceID)
	q.Set("metric", metric)
	if rng != "" {
		q.Set("range", rng)
	}
	var out struct {
		Data []MetricSample `json:"data"`
	}
	return out.Data, a.c.get(ctx, "metrics/query?"+q.Encode(), &out)
}

// ListAlerts returns alerts, optionally filtered by status.
func (a *ResourceMonAPI) ListAlerts(ctx context.Context, status string) ([]Alert, error) {
	path := "alerts"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	var out struct {
		Data []Alert `json:"data"`
	}
	return out.Data, a.c.get(ctx, path, &out)
}

// AckAlert acknowledges an alert.
func (a *ResourceMonAPI) AckAlert(ctx context.Context, id string) error {
	return a.c.post(ctx, "alerts/"+id+"/ack", nil, nil)
}

// ResolveAlert resolves an alert.
func (a *ResourceMonAPI) ResolveAlert(ctx context.Context, id string) error {
	return a.c.post(ctx, "alerts/"+id+"/resolve", nil, nil)
}
