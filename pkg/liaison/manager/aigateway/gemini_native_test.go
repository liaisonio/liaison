package aigateway

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiRequestPreservesStatelessNativeBody(t *testing.T) {
	body := `{"contents":[{"role":"user","parts":[{"text":"describe"},{"inlineData":{"mimeType":"image/png","data":"aGVsbG8="}}]}],"tools":[{"functionDeclarations":[{"name":"lookup","parameters":{"type":"object"}}]}],"generationConfig":{"thinkingConfig":{"includeThoughts":true}}}`
	p, err := PrepareGemini([]byte(body), "chat", true, map[string]string{"chat": "models/gemini-fixture"})
	require.NoError(t, err)
	require.Equal(t, body, string(p.Body))
	require.True(t, p.Stream)
	require.Equal(t, "models/gemini-fixture:streamGenerateContent", p.Operation)
	a, stream, ok := GeminiOperation("v1beta/models/chat:generateContent")
	require.True(t, ok)
	require.False(t, stream)
	require.Equal(t, "chat", a)
}

func TestGeminiRejectsUnownedReferencesAndUnsupportedRequests(t *testing.T) {
	for name, body := range map[string]string{
		"empty": `{}`, "null": `null`, "trailing": `{} {}`,
		"cache":               `{"contents":[{"parts":[{"text":"hi"}]}],"cachedContent":"cachedContents/other"}`,
		"file":                `{"contents":[{"parts":[{"fileData":{"fileUri":"files/other"}}]}]}`,
		"tool file":           `{"contents":[{"parts":[{"functionResponse":{"name":"x","response":{},"parts":[{"fileData":{"fileUri":"files/other"}}]}}]}]}`,
		"server tool":         `{"contents":[{"parts":[{"text":"hi"}]}],"tools":[{"urlContext":{}}]}`,
		"multiple candidates": `{"contents":[{"parts":[{"text":"hi"}]}],"generationConfig":{"candidateCount":2}}`,
		"null count":          `{"contents":[{"parts":[{"text":"hi"}]}],"generationConfig":{"candidateCount":null}}`,
		"null part":           `{"contents":[{"parts":[null]}]}`,
		"role":                `{"contents":[{"role":"developer","parts":[{"text":"hi"}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := PrepareGemini([]byte(body), "chat", false, map[string]string{"chat": "gemini-fixture"})
			require.Error(t, err)
		})
	}
	_, err := PrepareGemini([]byte(`{}`), "other", false, map[string]string{"chat": "fixture"})
	require.ErrorIs(t, err, ErrModelDenied)
	for _, path := range []string{"v1beta/models/../x:generateContent", "v1beta/models/x:generateContent?key=x", "v1beta/models/x:delete", "v1beta/models/x%2fy:generateContent"} {
		_, _, ok := GeminiOperation(path)
		require.False(t, ok, path)
	}
}

func TestGeminiNativeUsageCountsThoughtsWithoutDoubleCountingCache(t *testing.T) {
	u := Usage{}
	body := `{"modelVersion":"private","candidates":[{"index":0,"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":8,"candidatesTokenCount":2,"thoughtsTokenCount":5,"totalTokenCount":17}}`
	out, err := RewriteGeminiJSON([]byte(body), "chat", &u)
	require.NoError(t, err)
	require.True(t, u.Complete)
	require.EqualValues(t, 10, *u.Input)
	require.EqualValues(t, 7, *u.Output)
	require.NotContains(t, string(out), "private")
	require.Contains(t, string(out), `"modelVersion":"chat"`)
}

func TestGeminiStreamRequiresTerminalAndCleanEOF(t *testing.T) {
	first := `data: {"candidates":[{"content":{"parts":[{"text":"hi","thought":true}]}}]}` + "\n\n"
	last := `data: {"candidates":[{"finishReason":"STOP"}]}` + "\n\n"
	usage := `data: {"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2,"thoughtsTokenCount":1}}` + "\n\n"
	var out bytes.Buffer
	u := Usage{}
	err := RelayGeminiSSE(strings.NewReader(first+last+usage), "chat", func(p []byte) error { _, e := out.Write(p); return e }, &u)
	require.NoError(t, err)
	require.True(t, u.Complete)
	require.EqualValues(t, 3, *u.Output)
	require.Contains(t, out.String(), `"thought":true`)
	for name, body := range map[string]string{"truncated": first, "unframed": strings.TrimRight(last, "\n"), "after stop": last + first, "provider error": first + "data: {\"error\":{\"message\":\"private\"}}\n\n"} {
		t.Run(name, func(t *testing.T) {
			u := Usage{}
			require.Error(t, RelayGeminiSSE(strings.NewReader(body), "chat", func([]byte) error { return nil }, &u))
			require.False(t, u.Complete)
		})
	}
	u = Usage{}
	err = RelayGeminiSSE(strings.NewReader(last), "chat", func([]byte) error { return errors.New("disconnected") }, &u)
	require.Error(t, err)
	require.False(t, u.Complete)
}

func TestGeminiRejectsInvalidAccountingAndKeepsMissingUnknown(t *testing.T) {
	for _, metadata := range []string{`null`, `{"promptTokenCount":-1}`, `{"candidatesTokenCount":null}`, `{"candidatesTokenCount":9223372036854775807,"thoughtsTokenCount":1}`} {
		u := Usage{}
		_, err := RewriteGeminiJSON([]byte(`{"candidates":[{"finishReason":"STOP"}],"usageMetadata":`+metadata+`}`), "chat", &u)
		require.Error(t, err)
		require.False(t, u.Complete)
	}
	u := Usage{}
	_, err := RewriteGeminiJSON([]byte(`{"promptFeedback":{"blockReason":"SAFETY"}}`), "chat", &u)
	require.NoError(t, err)
	require.True(t, u.Complete)
	require.Nil(t, u.Input)
	require.Nil(t, u.Output)
}
