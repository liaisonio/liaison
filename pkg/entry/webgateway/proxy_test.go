package webgateway

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestPathProxyPreservesRequestAndScopesCredentials(t *testing.T) {
	for _, prefix := range []string{"/access/7/web/", "/_liaison/a/7/"} {
		t.Run(prefix, func(t *testing.T) { testPathProxy(t, prefix) })
	}
}

func testPathProxy(t *testing.T, prefix string) {
	t.Helper()
	var seen *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Clone(context.Background())
		w.Header().Set("Location", "/login?next=%2Fhome")
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "upstream", Path: "/", HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "liaison_web_7", Value: "forged", Path: "/"})
		w.Header().Set("Clear-Site-Data", "\"*\"")
		w.Header().Set("Service-Worker-Allowed", "/")
		w.WriteHeader(302)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	p := New(u, prefix, func(ctx context.Context) (net.Conn, error) { return (&net.Dialer{}).DialContext(ctx, "tcp", u.Host) })
	r := httptest.NewRequest("GET", "https://console.example"+prefix+"folder/a%2Fb?q=x%2Fy", nil)
	r.Header.Set("Authorization", "Bearer header.eyJpc3MiOiJsaWFpc29uIn0.signature")
	r.Header.Set("Cookie", "console=private; liaison_web_7=ticket")
	r.Header.Set("X-Forwarded-Prefix", "/forged")
	r.Header.Set("X-Forwarded-For", "forged")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	require.Equal(t, 302, w.Code)
	require.Equal(t, "/folder/a%2Fb?q=x%2Fy", seen.RequestURI)
	require.Empty(t, seen.Header.Get("Authorization"))
	require.Empty(t, seen.Header.Get("Cookie"))
	require.Equal(t, strings.TrimSuffix(prefix, "/"), seen.Header.Get("X-Forwarded-Prefix"))
	require.NotContains(t, seen.Header.Get("X-Forwarded-For"), "forged")
	require.Equal(t, prefix+"login?next=%2Fhome", w.Header().Get("Location"))
	require.Empty(t, w.Header().Get("Clear-Site-Data"))
	require.Empty(t, w.Header().Get("Service-Worker-Allowed"))
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, prefix, cookies[0].Path)
	r = httptest.NewRequest("GET", "https://console.example"+prefix, nil)
	r.AddCookie(cookies[0])
	r.AddCookie(&http.Cookie{Name: "root-secret", Value: "private"})
	p.ServeHTTP(httptest.NewRecorder(), r)
	require.Equal(t, "session=upstream", seen.Header.Get("Cookie"))
}

func TestDomainProxyStreamsAndUpgrades(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" {
			c, e := upgrader.Upgrade(w, r, nil)
			if e != nil {
				return
			}
			defer c.Close()
			kind, data, e := c.ReadMessage()
			if e == nil {
				_ = c.WriteMessage(kind, data)
			}
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		io.WriteString(w, "data: last\n\n")
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	p := httptest.NewServer(New(u, "", func(ctx context.Context) (net.Conn, error) { return (&net.Dialer{}).DialContext(ctx, "tcp", u.Host) }))
	defer p.Close()
	response, e := http.Get(p.URL + "/events")
	require.NoError(t, e)
	defer response.Body.Close()
	body, e := io.ReadAll(response.Body)
	require.NoError(t, e)
	require.Equal(t, "data: first\n\ndata: last\n\n", string(body))
	c, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(p.URL, "http")+"/ws", nil)
	require.NoError(t, e)
	defer c.Close()
	require.NoError(t, c.WriteMessage(websocket.TextMessage, []byte("hello")))
	_, message, e := c.ReadMessage()
	require.NoError(t, e)
	require.Equal(t, "hello", string(message))
}

func TestAuthorizationRedactionDoesNotBreakApplicationLogin(t *testing.T) {
	for _, value := range []string{"Bearer liaison_pat_secret", "Bearer header.eyJpc3MiOiJsaWFpc29uIn0.signature"} {
		require.True(t, consoleAuthorization(value))
	}
	for _, value := range []string{"Basic dXNlcjpwYXNz", "Bearer application-token", "Bearer header.eyJpc3MiOiJhcHAifQ.signature"} {
		require.False(t, consoleAuthorization(value))
	}
}
