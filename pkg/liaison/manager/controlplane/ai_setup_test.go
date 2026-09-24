package controlplane

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio"
	"github.com/stretchr/testify/require"
)

func TestAISetupURL(t *testing.T) {
	for _, raw := range []string{"http://localhost:8000/v1", "https://models.example.com", "http://127.0.0.1:11434/api/"} {
		t.Run(raw, func(t *testing.T) {
			target, config, err := parseAISetup(AISetupRequest{EdgeID: 1, Protocol: "openai-compatible", ServiceURL: raw})
			require.NoError(t, err)
			require.NotEmpty(t, target.Host)
			require.Equal(t, target.BasePath, config.BasePath)
		})
	}
	for _, raw := range []string{"", "localhost:8000", "ftp://host/v1", "http://u:pass@host/v1", "http://host/v1?key=secret", "http://host/v1#fragment", "http://host:", "http://host:0", "http://host:65536", "http://host/a/../v1", "http://host/%2e%2e/v1", "http://host/v1?", "http://host/v1#"} {
		t.Run(raw, func(t *testing.T) {
			_, _, err := parseAISetup(AISetupRequest{EdgeID: 1, Protocol: "openai-compatible", ServiceURL: raw})
			require.ErrorIs(t, err, ErrAIInvalid)
		})
	}
}

type setupTestStream struct {
	geminio.Stream
	conn net.Conn
}

func (s setupTestStream) Read(p []byte) (int, error)         { return s.conn.Read(p) }
func (s setupTestStream) Write(p []byte) (int, error)        { return s.conn.Write(p) }
func (s setupTestStream) Close() error                       { return s.conn.Close() }
func (s setupTestStream) LocalAddr() net.Addr                { return s.conn.LocalAddr() }
func (s setupTestStream) RemoteAddr() net.Addr               { return s.conn.RemoteAddr() }
func (s setupTestStream) SetDeadline(t time.Time) error      { return s.conn.SetDeadline(t) }
func (s setupTestStream) SetReadDeadline(t time.Time) error  { return s.conn.SetReadDeadline(t) }
func (s setupTestStream) SetWriteDeadline(t time.Time) error { return s.conn.SetWriteDeadline(t) }

type setupTestFrontier struct {
	frontierbound.FrontierBound
	open func(context.Context, uint64) (geminio.Stream, error)
}

func (f setupTestFrontier) OpenStream(ctx context.Context, id uint64) (geminio.Stream, error) {
	return f.open(ctx, id)
}

func TestAISetupProbeUsesConnectorWithoutSaving(t *testing.T) {
	cp, r := newTestControlPlane(t)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	_, one, _ := seedResourceScopeUsers(t, r)
	edge, _ := createTestEdgeApplication(t, r)
	edge.Online = model.EdgeOnlineStatusOnline
	edge.Status = model.EdgeStatusRunning
	require.NoError(t, r.UpdateEdge(edge))
	require.NoError(t, claimResource(one, r, resourceConnector, uint64(edge.ID)))
	auth, err := iam.NewIAMService(r)
	require.NoError(t, err)
	s, err := cp.NewAIService(auth, make([]byte, 32))
	require.NoError(t, err)
	cp.probeSlots = make(chan struct{}, 4)
	done := make(chan error, 1)
	cp.frontierBound = setupTestFrontier{open: func(ctx context.Context, id uint64) (geminio.Stream, error) {
		if id != uint64(edge.ID) {
			return nil, errors.New("incorrect connector")
		}
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			done <- func() error {
				header := make([]byte, 4)
				if _, err := io.ReadFull(server, header); err != nil {
					return err
				}
				body := make([]byte, binary.BigEndian.Uint32(header))
				if _, err := io.ReadFull(server, body); err != nil {
					return err
				}
				var target proto.Dst
				if err := json.Unmarshal(body, &target); err != nil {
					return err
				}
				if target.Addr != "localhost:18003" || target.ApplicationID != 0 || target.ProxyID != 0 {
					return errors.New("unexpected target attribution")
				}
				req, err := http.ReadRequest(bufio.NewReader(server))
				if err != nil {
					return err
				}
				defer req.Body.Close()
				if req.URL.Path != "/v1/models" || req.Header.Get("Authorization") != "Bearer fixture-key" {
					return errors.New("unexpected model discovery request")
				}
				payload := `{"object":"list","data":[{"id":"local-model"}]}`
				_, err = fmt.Fprintf(server, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(payload), payload)
				return err
			}()
		}()
		return setupTestStream{conn: client}, nil
	}}
	before, err := r.CountApplications(&dao.ListApplicationsQuery{})
	require.NoError(t, err)
	result, err := s.ProbeSetup(one, AISetupRequest{EdgeID: uint64(edge.ID), ServiceURL: "http://localhost:18003/v1", Protocol: "openai-compatible", APIKey: "fixture-key"})
	require.NoError(t, err)
	require.Equal(t, "compatible", result.State)
	require.Equal(t, []string{"local-model"}, result.Models)
	require.NoError(t, <-done)
	after, err := r.CountApplications(&dao.ListApplicationsQuery{})
	require.NoError(t, err)
	require.Equal(t, before, after)
}

type failingSetupRepo struct{ dao.Dao }

func (r failingSetupRepo) Begin() dao.Dao { return failingSetupTx{r.Dao.Begin()} }

type failingSetupTx struct{ dao.Dao }

func (r failingSetupTx) SaveAIAccess(context.Context, *model.AIAccess) error {
	return errors.New("injected access persistence failure")
}

func TestAISetupAtomicCreationAndScope(t *testing.T) {
	cp, r := newTestControlPlane(t)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	_, one, two := seedResourceScopeUsers(t, r)
	edge, _ := createTestEdgeApplication(t, r)
	require.NoError(t, claimResource(one, r, resourceConnector, uint64(edge.ID)))
	auth, err := iam.NewIAMService(r)
	require.NoError(t, err)
	s, err := cp.NewAIService(auth, make([]byte, 32))
	require.NoError(t, err)
	v := AISetupRequest{EdgeID: uint64(edge.ID), ServiceURL: "http://127.0.0.1:18001/v1", Protocol: "openai-compatible", Name: "Local API", APIKey: "fixture-secret", Models: map[string]string{"chat": "local-model"}}
	before, err := r.CountApplications(&dao.ListApplicationsQuery{})
	require.NoError(t, err)
	_, err = s.CreateSetup(two, v)
	require.ErrorIs(t, err, iam.ErrForbidden)
	_, err = s.CreateSetup(context.Background(), v)
	require.ErrorIs(t, err, iam.ErrForbidden)
	cp.repo = failingSetupRepo{r}
	_, err = s.CreateSetup(one, v)
	require.ErrorContains(t, err, "injected")
	count, err := r.CountApplications(&dao.ListApplicationsQuery{})
	require.NoError(t, err)
	require.Equal(t, before, count)
	proxies, err := r.CountProxies(&dao.ListProxiesQuery{})
	require.NoError(t, err)
	require.Zero(t, proxies)
	cp.repo = r
	created, err := s.CreateSetup(one, v)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	view, err := s.Workspace(one, created.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"chat"}, view.Models)
	require.True(t, view.Enabled)
	_, err = s.Workspace(two, created.ID)
	require.Error(t, err)
	config, err := r.GetAIApplication(one, created.ApplicationID)
	require.NoError(t, err)
	require.NotContains(t, config.EncryptedKey, "fixture-secret")
	require.NotEmpty(t, config.EncryptedKey)
	_, err = s.CreateSetup(one, v)
	require.ErrorIs(t, err, ErrAIExistingApplication)
	count, err = r.CountApplications(&dao.ListApplicationsQuery{})
	require.NoError(t, err)
	require.Equal(t, before+1, count)
	cp.probeSlots = make(chan struct{}, 4)
	v.ServiceURL = "http://127.0.0.1:18002/v1"
	probe, err := s.ProbeSetup(one, v)
	require.NoError(t, err)
	require.Equal(t, "unreachable", probe.State)
	count, err = r.CountApplications(&dao.ListApplicationsQuery{})
	require.NoError(t, err)
	require.Equal(t, before+1, count)
	_, err = s.ProbeSetup(two, v)
	require.ErrorIs(t, err, iam.ErrForbidden)
}
