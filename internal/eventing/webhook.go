package eventing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookPayload is the body sent to webhook endpoints.
type WebhookPayload struct {
	EventType string `json:"eventType"`
	Action    string `json:"action"`
	Project   string `json:"project"`
	Timestamp string `json:"timestamp"`
	RuleName  string `json:"ruleName,omitempty"`
}

// FireWebhook sends an HTTP POST to url with the given payload.
// Returns an error if the server responds with a non-2xx status.
func FireWebhook(ctx context.Context, url string, payload WebhookPayload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("webhook: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "capper-eventing/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: post %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: %s returned %d", url, resp.StatusCode)
	}
	return nil
}
