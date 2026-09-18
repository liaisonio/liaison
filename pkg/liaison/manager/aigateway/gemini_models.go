package aigateway

import (
	"context"
	"encoding/json"
	"io"
	"strings"
)

// Bounded pagination never exposes a partial catalog as a successful probe.
func (u *Upstream) probeGemini(ctx context.Context, key string) ProbeResult {
	page := ""
	seenPages := map[string]bool{}
	seenModels := map[string]bool{}
	models := []string{}
	for n := 0; n < 10; n++ {
		resp, err := u.requestProtocol(ctx, "GET", "models", key, "gemini", nil, false, page)
		if err != nil {
			return ProbeResult{State: "unreachable"}
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
		resp.Body.Close()
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return ProbeResult{State: "auth_required"}
		}
		if err != nil || resp.StatusCode != 200 || len(raw) > 1<<20 {
			return ProbeResult{State: "unknown"}
		}
		if _, err := responseObject(raw); err != nil {
			return ProbeResult{State: "unknown"}
		}
		var catalog struct {
			Models *[]struct {
				Name    string   `json:"name"`
				Methods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			Next string `json:"nextPageToken"`
		}
		if json.Unmarshal(raw, &catalog) != nil || catalog.Models == nil {
			return ProbeResult{State: "unknown"}
		}
		for _, m := range *catalog.Models {
			id := strings.TrimPrefix(m.Name, "models/")
			if id == m.Name || !safeGeminiModel(id) {
				return ProbeResult{State: "unknown"}
			}
			supported := len(m.Methods) == 0
			for _, method := range m.Methods {
				supported = supported || method == "generateContent"
			}
			if supported && !seenModels[id] {
				models = append(models, id)
				seenModels[id] = true
			}
			if len(models) > 1000 {
				return ProbeResult{State: "unknown"}
			}
		}
		if catalog.Next == "" {
			return ProbeResult{State: "compatible", Protocol: "gemini", Models: models}
		}
		if len(catalog.Next) > 4096 || seenPages[catalog.Next] {
			return ProbeResult{State: "unknown"}
		}
		seenPages[catalog.Next] = true
		page = catalog.Next
	}
	return ProbeResult{State: "unknown"}
}
