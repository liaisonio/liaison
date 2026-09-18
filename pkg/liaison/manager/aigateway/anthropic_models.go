package aigateway

import (
	"context"
	"encoding/json"
	"io"
)

// 所有分页共享 Probe 的超时；失败时不返回部分目录。
func (u *Upstream) probeAnthropic(ctx context.Context, key string) ProbeResult {
	page := ""
	seenPages, seenModels := map[string]bool{}, map[string]bool{}
	models := []string{}
	for n := 0; n < 10; n++ {
		resp, err := u.requestProtocol(ctx, "GET", "models", key, "anthropic", nil, false, page)
		if err != nil {
			return ProbeResult{State: "unreachable"}
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
		closeErr := resp.Body.Close()
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return ProbeResult{State: "auth_required"}
		}
		if readErr != nil || closeErr != nil || resp.StatusCode != 200 || len(raw) > 1<<20 {
			return ProbeResult{State: "unknown"}
		}
		if _, err := responseObject(raw); err != nil {
			return ProbeResult{State: "unknown"}
		}
		var catalog struct {
			Data *[]struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"data"`
			HasMore *bool  `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if json.Unmarshal(raw, &catalog) != nil || catalog.Data == nil || catalog.HasMore == nil {
			return ProbeResult{State: "unknown"}
		}
		for _, m := range *catalog.Data {
			if m.Type != "model" || !validModel(m.ID) {
				return ProbeResult{State: "unknown"}
			}
			if !seenModels[m.ID] {
				models = append(models, m.ID)
				seenModels[m.ID] = true
			}
			if len(models) > 1000 {
				return ProbeResult{State: "unknown"}
			}
		}
		if !*catalog.HasMore {
			return ProbeResult{State: "compatible", Protocol: "anthropic", Models: models}
		}
		items := *catalog.Data
		if len(items) == 0 || catalog.LastID != items[len(items)-1].ID || seenPages[catalog.LastID] {
			return ProbeResult{State: "unknown"}
		}
		seenPages[catalog.LastID] = true
		page = catalog.LastID
	}
	return ProbeResult{State: "unknown"}
}
