package frontierbound

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// KickEdge asks Frontier to close the currently connected Edge, if any.
func (fb *frontierBound) KickEdge(ctx context.Context, edgeID uint64) error {
	controlPlaneURL := normalizeControlPlaneURL(fb.controlPlaneURL)
	if controlPlaneURL == "" {
		return nil
	}

	endpoint := fmt.Sprintf("%s/v1/edges/%d", strings.TrimRight(controlPlaneURL, "/"), edgeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	client := fb.httpClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		fb.forgetEdge(edgeID)
		return nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("kick edge %d via frontier controlplane: status %d: %s", edgeID, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	fb.forgetEdge(edgeID)
	return nil
}

func normalizeControlPlaneURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" {
		return raw
	}
	return "http://" + raw
}
