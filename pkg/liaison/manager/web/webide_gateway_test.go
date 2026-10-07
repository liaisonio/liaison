package web

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

type ideGatewayControlPlane struct {
	controlplane.ControlPlane
	webIDEService
	revoked  atomic.Bool
	upstream string
}

func (cp *ideGatewayControlPlane) WebIDETarget(ctx context.Context, id string) (*model.WebIDEAccess, *model.WebIDEApplication, error) {
	if cp.revoked.Load() {
		return nil, nil, errors.New("revoked")
	}
	return &model.WebIDEAccess{ID: id, Enabled: true}, &model.WebIDEApplication{}, nil
}

func (cp *ideGatewayControlPlane) OpenWebIDEStream(ctx context.Context, access, instance string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "tcp", cp.upstream)
}

func TestIDEHandoffAndRequestBoundaries(t *testing.T) {
	w, admin, _ := newPermissionHTTPTest(t)
	cp := &ideGatewayControlPlane{}
	w.controlPlane = cp
	pat, err := w.iamService.CreatePAT(admin.ID, "ide-test", nil)
	require.NoError(t, err)
	g := &webIDEGateway{web: w}
	e := &ideEndpoint{access: "access", instance: "0123456789abcdef01234567", origin: "https://ide.example.test:9444", prefix: "/ide/0123456789abcdef01234567/", grants: map[string]ideGrant{}}
	ticket := "one-use-test-ticket"
	e.grants[ideGrantKey(ticket)] = ideGrant{token: pat.Token, expires: time.Now().Add(time.Minute), ticket: true, project: "/project with spaces"}
	call := func(path, origin, upgrade, cookie string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, e.origin+path, nil)
		r.TLS = &tls.ConnectionState{}
		r.Header.Set("Origin", origin)
		r.Header.Set("Upgrade", upgrade)
		if cookie != "" {
			r.Header.Set("Cookie", cookie)
		}
		out := httptest.NewRecorder()
		g.serve(e, out, r)
		return out
	}
	out := call(e.prefix+"?__liaison_ticket="+ticket, "", "", "")
	require.Equal(t, http.StatusSeeOther, out.Code, out.Body.String())
	require.Equal(t, e.prefix+"?folder=%2Fproject+with+spaces", out.Header().Get("Location"))
	cookies := out.Result().Cookies()
	require.Len(t, cookies, 1)
	require.True(t, cookies[0].HttpOnly)
	require.True(t, cookies[0].Secure)
	require.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
	require.NotContains(t, out.Header().Get("Set-Cookie"), pat.Token)
	require.Equal(t, 401, call(e.prefix+"?__liaison_ticket="+ticket, "", "", "").Code)
	cookie := cookies[0].Name + "=" + cookies[0].Value
	for _, tc := range []struct {
		path, origin, upgrade string
		code                  int
	}{
		{"/api/v1/iam/users", "", "", 404},
		{e.prefix, "https://console.example.test", "", 403},
		{e.prefix, "", "WebSocket", 403},
		{e.prefix + "proxy/3000/", "", "", 403},
		{e.prefix + "foo/../proxy/3000/", "", "", 400},
		{e.prefix + "%2e%2e/other", "", "", 400},
	} {
		require.Equal(t, tc.code, call(tc.path, tc.origin, tc.upgrade, cookie).Code, tc.path)
	}
	cp.revoked.Store(true)
	require.Equal(t, 403, call(e.prefix, "", "", cookie).Code)
}

func TestIDERevocationClosesOpenWebSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("exercises the 15 second revocation interval")
	}
	w, admin, _ := newPermissionHTTPTest(t)
	// Match code-server's actual origin policy. The public host and Origin
	// must agree, and forged forwarding headers must not reach the IDE.
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		origin, err := url.Parse(r.Header.Get("Origin"))
		return err == nil && origin.Host == r.Header.Get("X-Forwarded-Host") && r.Header.Get("Forwarded") == ""
	}}
	upstream := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, req *http.Request) {
		c, err := upgrader.Upgrade(out, req, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			if _, _, err = c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer upstream.Close()
	cp := &ideGatewayControlPlane{upstream: strings.TrimPrefix(upstream.URL, "http://")}
	w.controlPlane = cp
	pat, err := w.iamService.CreatePAT(admin.ID, "ide-websocket-test", nil)
	require.NoError(t, err)
	g := &webIDEGateway{web: w}
	e := &ideEndpoint{access: "access", instance: "0123456789abcdef01234567", prefix: "/ide/0123456789abcdef01234567/", grants: map[string]ideGrant{}}
	secret := "test-browser-secret"
	e.grants[ideGrantKey(secret)] = ideGrant{token: pat.Token, expires: time.Now().Add(time.Minute)}
	server := httptest.NewTLSServer(http.HandlerFunc(func(out http.ResponseWriter, req *http.Request) { g.serve(e, out, req) }))
	defer server.Close()
	e.origin = server.URL
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // Local test certificate only.
	headers := http.Header{"Origin": {e.origin}, "Cookie": {"liaison_web_ide_" + e.instance + "=" + secret}, "Forwarded": {"host=forged.invalid"}, "X-Forwarded-Host": {"forged.invalid"}}
	c, _, err := dialer.Dial("wss"+strings.TrimPrefix(server.URL, "https")+e.prefix, headers)
	require.NoError(t, err)
	defer c.Close()
	cp.revoked.Store(true)
	require.NoError(t, c.SetReadDeadline(time.Now().Add(20*time.Second)))
	_, _, err = c.ReadMessage()
	require.Error(t, err)
	var netError net.Error
	if errors.As(err, &netError) {
		require.False(t, netError.Timeout(), "revocation must close an existing socket, not await its timeout")
	}
}

func TestIDERequestDecoderBounds(t *testing.T) {
	for _, body := range []string{`{"unknown":true}`, `{} {}`, strings.Repeat(" ", 17<<10) + `{}`} {
		out := httptest.NewRecorder()
		var in struct {
			Name string `json:"name"`
		}
		require.False(t, decodeWebIDE(out, httptest.NewRequest("POST", "/", strings.NewReader(body)), &in))
		require.Equal(t, 400, out.Code)
	}
}
