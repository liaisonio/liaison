package web

import (
	"context"
	"crypto/tls"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	edgeide "github.com/liaisonio/liaison/pkg/edge/webide"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

type nativeIDEControlPlane struct {
	*ideGatewayControlPlane
	runtime *edgeide.Service
	owner   string
}

func (cp *nativeIDEControlPlane) OpenWebIDEStream(ctx context.Context, access, instance string) (net.Conn, error) {
	return cp.runtime.Dial(ctx, cp.owner, access, instance)
}

// Uses a verified upstream archive and an isolated profile, never the user's
// installed IDE, projects, credentials or production Manager. Opt in locally.
func TestIDENativeGateway(t *testing.T) {
	archive, parent := os.Getenv("LIAISON_WEBIDE_TEST_ARCHIVE"), os.Getenv("LIAISON_WEBIDE_TEST_ROOT")
	if archive == "" || parent == "" {
		t.Skip("native archive and private test parent not configured")
	}
	root, err := os.MkdirTemp(parent, ".ide-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(root)) })
	project := filepath.Join(root, "project")
	require.NoError(t, os.Mkdir(project, 0700))
	runtime, err := edgeide.New(root)
	require.NoError(t, err)
	f, err := os.Open(archive)
	require.NoError(t, err)
	_, err = edgeide.InstallOffline(t.Context(), filepath.Join(root, "packages"), f)
	require.NoError(t, f.Close())
	require.NoError(t, err)
	discovered := runtime.Handle(t.Context(), proto.WebIDERequest{Version: 1, OwnerID: "native-test", Action: "discover"})
	require.Len(t, discovered.Installations, 1)
	installed := runtime.Handle(t.Context(), proto.WebIDERequest{Version: 1, OwnerID: "native-test", Action: "install"})
	require.Equal(t, "ok", installed.Status, "existing verified installation must be reusable")
	q := proto.WebIDERequest{Version: 1, OwnerID: "native-test", AccessID: "native-access", ApplicationID: "native-app", InstallationID: discovered.Installations[0].ID, Project: project, Action: "start"}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		all := runtime.Handle(ctx, proto.WebIDERequest{Version: 1, OwnerID: q.OwnerID, Action: "instances"})
		for _, instance := range all.Instances {
			result := runtime.Handle(ctx, proto.WebIDERequest{Version: 1, OwnerID: q.OwnerID, AccessID: q.AccessID, InstanceID: instance.ID, Action: "stop"})
			require.Equal(t, "ok", result.Status)
		}
	})
	started := runtime.Handle(t.Context(), q)
	require.Equal(t, "ok", started.Status)
	require.Len(t, started.Instances, 1)
	id := started.Instances[0].ID
	w, admin, _ := newPermissionHTTPTest(t)
	cp := &nativeIDEControlPlane{ideGatewayControlPlane: &ideGatewayControlPlane{}, runtime: runtime, owner: q.OwnerID}
	w.controlPlane = cp
	pat, err := w.iamService.CreatePAT(admin.ID, "native-ide-test", nil)
	require.NoError(t, err)
	webDir, err := filepath.Abs("../../../../web/public")
	require.NoError(t, err)
	g := newWebIDEGateway(w, &config.Configuration{Manager: config.Manager{WebDir: webDir}})
	t.Cleanup(func() { require.NoError(t, g.Close()) })
	e := &ideEndpoint{access: q.AccessID, instance: id, prefix: "/ide/" + id + "/", grants: map[string]ideGrant{}}
	ticket := "native-one-use-ticket"
	e.grants[ideGrantKey(ticket)] = ideGrant{token: pat.Token, expires: time.Now().Add(time.Minute), ticket: true, project: project, theme: "dark"}
	g.endpoints = map[string]*ideEndpoint{e.access + ":" + e.instance: e}
	server := httptest.NewTLSServer(requestTimeoutFilter(g))
	t.Cleanup(server.Close)
	e.origin = server.URL
	client := server.Client()
	client.Timeout = 20 * time.Second
	client.Jar, err = cookiejar.New(nil)
	require.NoError(t, err)
	response, err := client.Get(server.URL + e.prefix + "?__liaison_ticket=" + ticket)
	require.NoError(t, err)
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode, string(body))
	require.Contains(t, string(body), "vscode-workbench")
	require.Contains(t, string(body), "liaison-ide-loading-theme")
	require.Contains(t, string(body), "Dark Modern")
	require.NotContains(t, string(body), pat.Token)
	require.NotContains(t, response.Request.URL.String(), ticket)
	// Exercise the actual browser entrypoint's local scripts and styles through
	// the gateway, catching base-path regressions that /healthz cannot detect.
	tokens := html.NewTokenizer(strings.NewReader(string(body)))
	assets := 0
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			require.ErrorIs(t, tokens.Err(), io.EOF)
			break
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		tag := tokens.Token()
		if tag.Data != "script" && tag.Data != "link" {
			continue
		}
		for _, a := range tag.Attr {
			if a.Key != "src" && a.Key != "href" {
				continue
			}
			u, err := url.Parse(a.Val)
			require.NoError(t, err)
			asset := response.Request.URL.ResolveReference(u)
			if asset.Host != response.Request.URL.Host {
				continue
			}
			require.True(t, strings.HasPrefix(asset.Path, e.prefix), "asset escapes authorized IDE path: %s", asset.Path)
			r, err := client.Get(asset.String())
			require.NoError(t, err)
			data, copyErr := io.ReadAll(r.Body)
			r.Body.Close()
			require.NoError(t, copyErr)
			require.Equal(t, http.StatusOK, r.StatusCode, asset.Path)
			if strings.HasSuffix(asset.Path, "/favicon.ico") {
				require.Equal(t, "6bd4b1e1", asset.Query().Get("liaison"))
				icon, err := os.ReadFile(filepath.Join(webDir, "favicon.ico"))
				require.NoError(t, err)
				require.Equal(t, icon, data)
				require.Equal(t, "image/x-icon", r.Header.Get("Content-Type"))
			}
			assets++
		}
	}
	require.Positive(t, assets)
	u, err := url.Parse(server.URL + e.prefix)
	require.NoError(t, err)
	cookieHeader := &http.Request{Header: http.Header{}}
	for _, cookie := range client.Jar.Cookies(u) {
		cookieHeader.AddCookie(cookie)
	}
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, HandshakeTimeout: 10 * time.Second} // Isolated test TLS only.
	headers := http.Header{"Origin": {server.URL}, "Cookie": {cookieHeader.Header.Get("Cookie")}}
	c, res, err := dialer.Dial("wss"+strings.TrimPrefix(server.URL, "https")+e.prefix+"?reconnectionToken=native-test&reconnection=false&skipWebSocketFrames=false", headers)
	if err != nil && res != nil {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		res.Body.Close()
		t.Logf("WebSocket status %d: %s", res.StatusCode, data)
	}
	require.NoError(t, err)
	require.NoError(t, c.Close())
	// An opt-in browser driver gets a one-use URL file, never a real account or
	// production endpoint. Run it while the isolated fixture is alive; teardown
	// always stops the native instance and removes the profile afterwards.
	if script := os.Getenv("LIAISON_WEBIDE_BROWSER_SCRIPT"); script != "" {
		require.NoError(t, os.WriteFile(filepath.Join(project, "browser-smoke.txt"), []byte("initial IDE fixture\n"), 0600))
		// A fresh local repository, with no commits, remotes or user hooks.
		gitCtx, gitCancel := context.WithTimeout(t.Context(), 10*time.Second)
		gitOutput, gitErr := exec.CommandContext(gitCtx, "git", "-c", "init.templateDir=", "init", "--quiet", project).CombinedOutput()
		gitCancel()
		require.NoError(t, gitErr, string(gitOutput))
		qa, err := os.MkdirTemp("", "liaison-ide-browser-")
		require.NoError(t, err)
		defer os.RemoveAll(qa)
		browserTicket, err := ideRandom()
		require.NoError(t, err)
		e.mu.Lock()
		e.grants[ideGrantKey(browserTicket)] = ideGrant{token: pat.Token, expires: time.Now().Add(5 * time.Minute), ticket: true, project: project, theme: "dark"}
		e.mu.Unlock()
		require.NoError(t, os.WriteFile(filepath.Join(qa, "url"), []byte(server.URL+e.prefix+"?__liaison_ticket="+browserTicket), 0600))
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
		defer cancel()
		output, err := exec.CommandContext(ctx, "node", script, filepath.Join(qa, "url"), root).CombinedOutput()
		require.NoError(t, err, string(output))
		saved, err := os.ReadFile(filepath.Join(project, "browser-smoke.txt"))
		require.NoError(t, err)
		require.Equal(t, "LIAISON_BROWSER_SAVE_OK", string(saved), "browser must save through the remote editor")
		var extensionLogs strings.Builder
		err = filepath.WalkDir(filepath.Join(root, id, "data", "logs"), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Name() != "remoteexthost.log" || !entry.Type().IsRegular() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err == nil {
				extensionLogs.Write(data)
			}
			return err
		})
		require.NoError(t, err)
		require.Contains(t, extensionLogs.String(), "ExtensionService#_doActivateExtension vscode.git,", "Git must activate in the remote Node host")
		require.Contains(t, extensionLogs.String(), "ExtensionService#_doActivateExtension vscode.git-base,", "Git's dependency must activate in the remote Node host")
		t.Log(string(output))
	}
	// Reconstruct the connector registry while its native child remains alive.
	// This exercises the same persisted state used after a connector reconnect.
	restored, err := edgeide.New(root)
	require.NoError(t, err)
	resumed := restored.Handle(t.Context(), q)
	require.Equal(t, "ok", resumed.Status)
	require.Len(t, resumed.Instances, 1)
	require.Equal(t, id, resumed.Instances[0].ID)
	require.Equal(t, q.ApplicationID, resumed.Instances[0].ApplicationID)
	require.Equal(t, started.Instances[0].StartedAt, resumed.Instances[0].StartedAt, "reconnect must not restart the IDE")
	connection, err := restored.Dial(t.Context(), q.OwnerID, q.AccessID, id)
	require.NoError(t, err)
	require.NoError(t, connection.Close())
	// A Manager shutdown invalidates grants, but must not stop the native IDE.
	require.NoError(t, g.Close())
	response, err = client.Get(server.URL + e.prefix)
	require.NoError(t, err)
	response.Body.Close()
	require.NotEqual(t, http.StatusOK, response.StatusCode)
	stillRunning := restored.Handle(t.Context(), proto.WebIDERequest{Version: 1, OwnerID: q.OwnerID, Action: "instances"})
	require.Len(t, stillRunning.Instances, 1)
	require.Equal(t, "running", stillRunning.Instances[0].Status)
	restarted := newWebIDEGateway(w, g.conf)
	fresh := &ideEndpoint{access: q.AccessID, instance: id, prefix: e.prefix, grants: map[string]ideGrant{}}
	restarted.endpoints = map[string]*ideEndpoint{fresh.access + ":" + id: fresh}
	restartedServer := httptest.NewTLSServer(requestTimeoutFilter(restarted))
	t.Cleanup(restartedServer.Close)
	t.Cleanup(func() { require.NoError(t, restarted.Close()) })
	fresh.origin = restartedServer.URL
	response, err = client.Get(restartedServer.URL + fresh.prefix)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusUnauthorized, response.StatusCode, "old Manager cookie must not survive restart")
	freshTicket, err := ideRandom()
	require.NoError(t, err)
	fresh.mu.Lock()
	fresh.grants[ideGrantKey(freshTicket)] = ideGrant{token: pat.Token, expires: time.Now().Add(time.Minute), ticket: true, project: project}
	fresh.mu.Unlock()
	response, err = client.Get(restartedServer.URL + fresh.prefix + "?__liaison_ticket=" + freshTicket)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode, "reopening must reuse the running native IDE")
	cp.revoked.Store(true)
	response, err = client.Get(restartedServer.URL + fresh.prefix)
	require.NoError(t, err)
	response.Body.Close()
	require.NotEqual(t, http.StatusOK, response.StatusCode, "revoked access must invalidate an existing page")
	t.Log("PASS connector registry recovery preserves process; Manager shutdown revokes old grant without stopping native IDE")
}
