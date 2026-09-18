package web

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/utils"
	"github.com/stretchr/testify/require"
)

type httpEntryFixture struct {
	controlplane.ControlPlane
	address string
	prefix  string
	denied  bool
}

func (b *httpEntryFixture) HTTPEntryTarget(ctx context.Context, id uint) (*controlplane.HTTPEntryTarget, error) {
	if b.denied || id != 1 || ctx.Value("user_id") == nil {
		return nil, errors.New("denied")
	}
	return &controlplane.HTTPEntryTarget{Mode: "path", URL: "https://console.example" + b.prefix, Address: b.address}, nil
}
func (b *httpEntryFixture) HTTPEntrySourceAllowed(uint, string) bool { return !b.denied }
func (b *httpEntryFixture) OpenHTTPEntryStream(ctx context.Context, id uint) (net.Conn, error) {
	if _, e := b.HTTPEntryTarget(ctx, id); e != nil {
		return nil, e
	}
	return (&net.Dialer{}).DialContext(ctx, "tcp", b.address)
}

func TestHTTPEntryLaunchAuthenticationAndRevocation(t *testing.T) {
	for _, prefix := range []string{"/access/1/web/", "/_liaison/a/1/"} {
		t.Run(prefix, func(t *testing.T) { testHTTPEntryLaunch(t, prefix) })
	}
}

func testHTTPEntryLaunch(t *testing.T, prefix string) {
	t.Helper()
	server, admin, _ := newPermissionHTTPTest(t)
	require.NoError(t, utils.SetJWTSecret("http-entry-unit-test-secret-long-enough"))
	token, err := utils.GenerateToken(admin.ID, admin.Email)
	require.NoError(t, err)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Cookie"))
		require.Equal(t, "/docs/a%2Fb?q=x%2Fy", r.URL.RequestURI())
		w.Write([]byte("website"))
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	backend := &httpEntryFixture{address: u.Host, prefix: prefix}
	server.controlPlane = backend
	server.httpEntries = &httpEntries{conf: &config.Configuration{}, grants: map[string]httpEntryGrant{}}
	r := httptest.NewRequest("POST", "https://console.example/api/v1/web-entries/1/launch", nil)
	w := httptest.NewRecorder()
	server.handleHTTPEntryAPI(w, r)
	require.Equal(t, 401, w.Code)
	r.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	server.handleHTTPEntryAPI(w, r)
	require.Equal(t, 200, w.Code)
	var response struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.NotContains(t, response.Data.URL, token)
	h := server.httpEntryFilter(http.NotFoundHandler())
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", response.Data.URL, nil))
	require.Equal(t, 303, w.Code)
	require.Equal(t, prefix, w.Header().Get("Location"))
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	require.True(t, cookies[0].HttpOnly)
	require.True(t, cookies[0].Secure)
	require.Equal(t, prefix, cookies[0].Path)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", response.Data.URL, nil))
	require.Equal(t, 401, w.Code)
	r = httptest.NewRequest("GET", "https://console.example"+prefix+"docs/a%2Fb?q=x%2Fy", nil)
	r.AddCookie(cookies[0])
	r.AddCookie(&http.Cookie{Name: "console", Value: "private"})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "website", w.Body.String())
	backend.denied = true
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, 403, w.Code)
}

func TestHTTPEntryAccessRouteBoundary(t *testing.T) {
	server := &web{httpEntries: &httpEntries{grants: map[string]httpEntryGrant{}, conf: &config.Configuration{}}, controlPlane: &httpEntryFixture{}}
	h := server.httpEntryFilter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, tc := range []struct {
		path string
		code int
	}{
		{"/access/1", 204}, {"/access/1/sessions/abc", 204}, {"/access/1/websocket", 204},
		{"/access/1/web/", 401}, {"/access/1/web/api/v1/iam/users", 401},
		{"/access/1/web/sessions/abc", 401}, {"/access/1/web/keys", 401},
		{"/access/no/web/", 404}, {"/access/01/web/", 404}, {"/access/0/web/", 404},
		{"/access/1%2Fweb/", 400}, {"/access/1/we%62/", 400},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "https://console.example"+tc.path, nil))
			require.Equal(t, tc.code, w.Code)
		})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "https://console.example/access/1/web?q=x%2Fy", nil))
	require.Equal(t, 307, w.Code)
	require.Equal(t, "/access/1/web/?q=x%2Fy", w.Header().Get("Location"))
}

func TestHTTPEntryGrantsAreSingleUseAndAccessBound(t *testing.T) {
	s := &httpEntries{grants: map[string]httpEntryGrant{}}
	key, ok := s.issue(httpEntryGrant{id: 1, mode: "path", token: "private", ticket: true, expires: time.Now().Add(time.Minute)})
	require.True(t, ok)
	_, ok = s.get(key, 2, "path", true)
	require.False(t, ok)
	_, ok = s.get(key, 1, "domain", true)
	require.False(t, ok)
	_, ok = s.get(key, 1, "path", false)
	require.False(t, ok)
	_, ok = s.get(key, 1, "path", true)
	require.True(t, ok)
	_, ok = s.get(key, 1, "path", true)
	require.False(t, ok)
	key, ok = s.issue(httpEntryGrant{id: 1, mode: "path", expires: time.Now().Add(-time.Second)})
	require.True(t, ok)
	_, ok = s.get(key, 1, "path", false)
	require.False(t, ok)
}

func TestHTTPEntryNamespaceNeverFallsThroughToConsole(t *testing.T) {
	conf := &config.Configuration{}
	conf.Manager.WebDomain = "apps.example"
	conf.Manager.ServerURL = "https://console.apps.example"
	web := &web{httpEntries: &httpEntries{grants: map[string]httpEntryGrant{}, conf: conf}}
	h := web.httpEntryFilter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, raw := range []string{"https://a-1.apps.example/api/v1/iam/users", "https://unknown.apps.example/", "https://console.example/_liaison/a/no/", "https://console.example/_liaison/a/01/", "https://console.example/_liaison/unknown"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", raw, nil))
		require.NotEqual(t, 204, w.Code, raw)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "https://console.example/api/v1/iam/users", nil))
	require.Equal(t, 204, w.Code)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "https://console.apps.example/api/v1/iam/users", nil))
	require.Equal(t, 204, w.Code)
}
