package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	baseconfig "github.com/liaisonio/liaison/pkg/config"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

const lifecycleInstance = "0123456789abcdef01234567"

func TestIDEDiagnosticHeaderOnFailure(t *testing.T) {
	g, cp, token := newLifecycleGateway(t)
	cp.mutationErr = errors.New("private upstream token must never be returned")
	result := lifecycleRequest(g, token, "POST", "accesses/access/runtime", `{"action":"start"}`)
	require.Equal(t, 500, result.Code)
	id := result.Header().Get("X-Request-ID")
	require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{32}$`), id)
	require.Contains(t, result.Body.String(), id)
	require.NotContains(t, result.Body.String(), "private upstream")
	again := lifecycleRequest(g, token, "POST", "accesses/access/runtime", `{"action":"start"}`)
	require.NotEqual(t, id, again.Header().Get("X-Request-ID"))
}

func TestIDESharedOriginUsesConsoleWithoutListener(t *testing.T) {
	g, _, token := newLifecycleGateway(t)
	g.conf.Manager.WebIDE.SharedOrigin = true
	g.conf.Manager.WebIDE.PortStart = 0
	g.conf.Manager.WebIDE.PortEnd = 0
	require.True(t, g.ready())
	link, err := g.launch(t.Context(), token, "access", lifecycleInstance, "/project")
	require.NoError(t, err)
	u, err := url.Parse(link)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", u.Host)
	require.Equal(t, "/ide/"+lifecycleInstance+"/", u.Path)
	entry := g.endpoints["access:"+lifecycleInstance]
	require.Nil(t, entry.listener)
	require.Nil(t, entry.server)
	request := httptest.NewRequest("GET", link, nil)
	response := httptest.NewRecorder()
	g.ServeHTTP(response, request)
	require.Equal(t, http.StatusSeeOther, response.Code)
	cookies := response.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, entry.prefix, cookies[0].Path)
	request = httptest.NewRequest("GET", entry.origin+entry.prefix, nil)
	request.AddCookie(cookies[0])
	response = httptest.NewRecorder()
	g.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
	// A valid browser grant for A cannot authorize a different instance B.
	other := "fedcba9876543210fedcba98"
	_, err = g.launch(t.Context(), token, "other", other, "/other")
	require.NoError(t, err)
	request = httptest.NewRequest("GET", entry.origin+"/ide/"+other+"/", nil)
	request.AddCookie(cookies[0])
	response = httptest.NewRecorder()
	g.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.NoError(t, g.releaseEndpoints("access", ""))
	response = httptest.NewRecorder()
	g.ServeHTTP(response, httptest.NewRequest("GET", link, nil))
	require.Equal(t, http.StatusGone, response.Code)
}

type lifecycleIDEControlPlane struct {
	*ideGatewayControlPlane
	mutationErr  error
	status       string
	runtimeCalls atomic.Int32
	openStarted  chan struct{}
	openContinue <-chan struct{}
}

func (cp *lifecycleIDEControlPlane) OpenWebIDEStream(ctx context.Context, access, instance string) (net.Conn, error) {
	if cp.openStarted != nil {
		select {
		case cp.openStarted <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		select {
		case <-cp.openContinue:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return cp.ideGatewayControlPlane.OpenWebIDEStream(ctx, access, instance)
}

func (cp *lifecycleIDEControlPlane) WebIDERuntime(context.Context, string, proto.WebIDERequest) (proto.WebIDEResult, error) {
	cp.runtimeCalls.Add(1)
	return proto.WebIDEResult{Version: 1, Status: cp.status}, cp.mutationErr
}
func (cp *lifecycleIDEControlPlane) SaveWebIDEAccess(_ context.Context, id string, in controlplane.WebIDEAccessInput) (*model.WebIDEAccess, error) {
	return &model.WebIDEAccess{ID: id, Enabled: in.Enabled}, cp.mutationErr
}
func (cp *lifecycleIDEControlPlane) DeleteWebIDEAccess(context.Context, string) error {
	return cp.mutationErr
}

func newLifecycleGateway(t *testing.T) (*webIDEGateway, *lifecycleIDEControlPlane, string) {
	t.Helper()
	w, admin, _ := newPermissionHTTPTest(t)
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		origin, err := url.Parse(r.Header.Get("Origin"))
		return err == nil && origin.Host == r.Header.Get("X-Forwarded-Host")
	}}
	upstream := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, req *http.Request) {
		if !websocket.IsWebSocketUpgrade(req) {
			out.WriteHeader(http.StatusNoContent)
			return
		}
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
	t.Cleanup(upstream.Close)
	cp := &lifecycleIDEControlPlane{ideGatewayControlPlane: &ideGatewayControlPlane{upstream: strings.TrimPrefix(upstream.URL, "http://")}, status: "ok"}
	w.controlPlane = cp
	pat, err := w.iamService.CreatePAT(admin.ID, "isolated-ide-lifecycle-test", nil)
	require.NoError(t, err)
	// Reuse Go's localhost test certificate, never deployment keys.
	seed := httptest.NewTLSServer(http.NotFoundHandler())
	cert := seed.TLS.Certificates[0]
	seed.Close()
	dir := t.TempDir()
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	require.NoError(t, err)
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	conf := &config.Configuration{}
	conf.Manager.ServerURL = "https://127.0.0.1"
	conf.Manager.Listen.Addr = "127.0.0.1:0"
	conf.Manager.Listen.TLS.Enable = true
	conf.Manager.Listen.TLS.Certs = []baseconfig.CertKey{{Cert: certFile, Key: keyFile}}
	conf.Manager.WebIDE = config.WebIDE{Enabled: true, PortStart: port, PortEnd: port}
	g := newWebIDEGateway(w, conf)
	w.ideGateway = g
	t.Cleanup(func() { require.NoError(t, g.Close()) })
	return g, cp, pat.Token
}

func lifecycleRequest(g *webIDEGateway, token, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/v1/webide/"+path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	out := httptest.NewRecorder()
	g.web.handleWebIDEHTTP(out, r)
	return out
}

func TestIDEStopReleasesPortAndInvalidatesOldGrants(t *testing.T) {
	g, _, token := newLifecycleGateway(t)
	first, err := g.launch(t.Context(), token, "access", lifecycleInstance, "/project")
	require.NoError(t, err)
	u, err := url.Parse(first)
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }} // Test TLS only.
	t.Cleanup(client.CloseIdleConnections)
	response, err := client.Get(first)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusSeeOther, response.StatusCode)
	oldCookie := response.Cookies()[0]
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, HandshakeTimeout: 3 * time.Second} // Test TLS only.
	headers := http.Header{"Origin": {"https://" + u.Host}, "Cookie": {oldCookie.Name + "=" + oldCookie.Value}}
	c, _, err := dialer.Dial("wss://"+u.Host+u.Path, headers)
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	out := lifecycleRequest(g, token, "POST", "accesses/access/runtime", `{"action":"stop","instance_id":"`+lifecycleInstance+`"}`)
	require.Equal(t, 200, out.Code, out.Body.String())
	require.NoError(t, c.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, _, err = c.ReadMessage()
	require.Error(t, err)
	var timeout net.Error
	if errors.As(err, &timeout) {
		require.False(t, timeout.Timeout(), "stop must close existing WebSockets immediately")
	}
	// The only allowed port must be reusable, with fresh authentication state.
	second, err := g.launch(t.Context(), token, "access", lifecycleInstance, "/project")
	require.NoError(t, err)
	v, err := url.Parse(second)
	require.NoError(t, err)
	require.Equal(t, u.Host, v.Host)
	req, err := http.NewRequest("GET", "https://"+v.Host+v.Path, nil)
	require.NoError(t, err)
	req.AddCookie(oldCookie)
	response, err = client.Do(req)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, 401, response.StatusCode)
	response, err = client.Get(first)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, 401, response.StatusCode)
	response, err = client.Get(second)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusSeeOther, response.StatusCode)
}

func TestIDEMutationsOnlyReleaseAuthorizedSuccessfulTargets(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body, status string
		denied, release                  bool
	}{
		{"disable", "PUT", "accesses/access", `{"enabled":false}`, "ok", false, true},
		{"delete", "DELETE", "accesses/access", ``, "ok", false, true},
		{"enable", "PUT", "accesses/access", `{"enabled":true}`, "ok", false, false},
		{"wrong-instance", "POST", "accesses/access/runtime", `{"action":"stop","instance_id":"fedcba987654321001234567"}`, "ok", false, false},
		{"wrong-access", "DELETE", "accesses/other", ``, "ok", false, false},
		{"denied", "DELETE", "accesses/access", ``, "ok", true, false},
		{"failed-stop", "POST", "accesses/access/runtime", `{"action":"stop","instance_id":"` + lifecycleInstance + `"}`, "unavailable", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, cp, token := newLifecycleGateway(t)
			_, err := g.launch(t.Context(), token, "access", lifecycleInstance, "/project")
			require.NoError(t, err)
			cp.status = tc.status
			if tc.denied {
				cp.mutationErr = iam.ErrForbidden
			}
			out := lifecycleRequest(g, token, tc.method, tc.path, tc.body)
			if tc.denied {
				require.Equal(t, 403, out.Code)
			} else {
				require.Equal(t, 200, out.Code, out.Body.String())
			}
			g.mu.Lock()
			_, present := g.endpoints["access:"+lifecycleInstance]
			g.mu.Unlock()
			require.Equal(t, !tc.release, present)
		})
	}
}

func TestIDEExpiredGrantsReleasePortWithoutStoppingDevice(t *testing.T) {
	g, cp, token := newLifecycleGateway(t)
	_, err := g.launch(t.Context(), token, "access", lifecycleInstance, "/project")
	require.NoError(t, err)
	g.mu.Lock()
	entry := g.endpoints["access:"+lifecycleInstance]
	g.mu.Unlock()
	entry.mu.Lock()
	entry.grants["browser"] = ideGrant{token: token, expires: time.Now().Add(time.Hour)}
	entry.mu.Unlock()
	require.NoError(t, g.reapExpired(time.Now().Add(time.Minute)))
	g.mu.Lock()
	count := len(g.endpoints)
	g.mu.Unlock()
	require.Equal(t, 1, count, "a live browser grant keeps its endpoint")
	require.NoError(t, g.reapExpired(time.Now().Add(2*time.Hour)))
	g.mu.Lock()
	count = len(g.endpoints)
	g.mu.Unlock()
	require.Zero(t, count)
	require.Zero(t, cp.runtimeCalls.Load(), "grant cleanup must not stop device processes")
	_, err = g.launch(t.Context(), token, "another-access", "fedcba987654321001234567", "/project")
	require.NoError(t, err, "expired endpoints must release their port")
}

func TestIDECollectorDoesNotInterruptTicketExchange(t *testing.T) {
	g, _, token := newLifecycleGateway(t)
	_, err := g.launch(t.Context(), token, "access", lifecycleInstance, "/project")
	require.NoError(t, err)
	g.mu.Lock()
	entry := g.endpoints["access:"+lifecycleInstance]
	g.mu.Unlock()
	entry.mu.Lock()
	clear(entry.grants)
	entry.pendingHandoffs = 1
	entry.mu.Unlock()
	require.NoError(t, g.reapExpired(time.Now().Add(time.Minute)))
	entry.mu.Lock()
	closed := entry.closed
	entry.pendingHandoffs = 0
	entry.mu.Unlock()
	require.False(t, closed, "ticket authentication has not completed yet")
	require.NoError(t, g.reapExpired(time.Now().Add(time.Minute)))
	entry.mu.Lock()
	closed = entry.closed
	entry.mu.Unlock()
	require.True(t, closed)
}

func TestIDEStopIsOrderedAfterInflightLaunch(t *testing.T) {
	g, cp, token := newLifecycleGateway(t)
	cp.openStarted = make(chan struct{}, 1)
	resume := make(chan struct{})
	cp.openContinue = resume
	var once sync.Once
	proceed := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(proceed)
	launched := make(chan error, 1)
	go func() {
		_, err := g.launch(t.Context(), token, "access", lifecycleInstance, "/project")
		launched <- err
	}()
	select {
	case <-cp.openStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("launch did not start")
	}
	// A canceled waiter neither invokes the device nor retires a live endpoint.
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	release, err := g.acquireAccess(ctx, "access")
	require.Nil(t, release)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	stopped := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		stopped <- lifecycleRequest(g, token, "POST", "accesses/access/runtime", `{"action":"stop","instance_id":"`+lifecycleInstance+`"}`)
	}()
	require.Zero(t, cp.runtimeCalls.Load())
	proceed()
	require.NoError(t, <-launched)
	out := <-stopped
	require.Equal(t, 200, out.Code, out.Body.String())
	var result map[string]any
	require.NoError(t, json.Unmarshal(out.Body.Bytes(), &result))
	require.Equal(t, float64(200), result["code"])
	g.mu.Lock()
	count := len(g.endpoints)
	g.mu.Unlock()
	require.Zero(t, count, "the completed stop must not leave a late-created listener")
}

func TestIDECloseRejectsLaunchAndReleasesListener(t *testing.T) {
	g, _, token := newLifecycleGateway(t)
	_, err := g.launch(t.Context(), token, "access", lifecycleInstance, "/project")
	require.NoError(t, err)
	require.NoError(t, g.Close())
	require.NoError(t, g.Close())
	_, err = g.launch(t.Context(), token, "access", lifecycleInstance, "/project")
	require.Error(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(g.conf.Manager.WebIDE.PortStart))
	require.NoError(t, err)
	require.NoError(t, listener.Close())
}
