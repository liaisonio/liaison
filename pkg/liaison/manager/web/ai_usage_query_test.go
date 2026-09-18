package web

import "testing"

func TestAIUsageQuery_OnlySupportedWindows(t *testing.T) {
	for _, raw := range []string{"hours=1", "hours=6", "hours=24", "hours=168", "hours=720"} {
		if !validUsageQuery(raw) {
			t.Errorf("rejected %q", raw)
		}
	}
	for _, raw := range []string{"hours=0", "hours=2", "hours=-1", "hours=721", "hours=24&hours=1", "hours=24&user_id=2", "alt=sse", "hours="} {
		if validUsageQuery(raw) {
			t.Errorf("accepted %q", raw)
		}
	}
}
