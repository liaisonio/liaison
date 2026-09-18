package aigateway

import (
	"errors"
	"testing"
)

func TestPreparePlaygroundRequestBoundary(t *testing.T) {
	const request = `{"model":"public","messages":[{"role":"user","content":"hello"}],"max_tokens":1024,"stream":true}`
	for _, tc := range []struct {
		name, protocol, suffix string
		invalid                bool
	}{
		{"qwen", "qwen", "", false},
		{"gemini", "gemini", "", false},
		{"whitespace", "gemini", "\n\t ", false},
		{"second object", "qwen", `{}`, true},
		{"trailing null", "gemini", ` null`, true},
		{"trailing garbage", "qwen", ` invalid`, true},
		{"unknown protocol", "unknown", "", true},
		{"empty protocol", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PreparePlayground([]byte(request+tc.suffix), map[string]string{"public": "upstream"}, tc.protocol)
			if tc.invalid {
				if !errors.Is(err, ErrUnsupported) {
					t.Fatalf("expected ErrUnsupported, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("valid request rejected: %v", err)
			}
		})
	}
}
