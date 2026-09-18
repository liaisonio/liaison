package dameng

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	dm "gitee.com/chunanyong/dm"
	"github.com/stretchr/testify/require"
)

func TestDamengDSNPreservesNativeCredentialsAndRestrictsOptions(t *testing.T) {
	for _, password := range []string{"Dm!pass", "p:/?@#&=+% 中文", "?dialName=evil&host=attacker@other:123", ""} {
		options := Options{Host: "private.internal", Port: 5236, Username: "a@b中文", Password: password}
		value := dsn(options, "session.invalid:5236")
		connector, err := (&dm.DmDriver{}).OpenConnector(value)
		require.NoError(t, err)
		// BuildDSN exposes the native parser's credentials using QueryEscape.
		want := "dm://" + url.QueryEscape(options.Username)
		if password != "" {
			want += ":" + url.QueryEscape(password)
		}
		want += "@session.invalid:5236"
		require.True(t, strings.HasPrefix(connector.(*dm.DmConnector).BuildDSN(), want), "native credentials changed")
		q, err := url.ParseQuery(value[strings.LastIndex(value, "?")+1:])
		require.NoError(t, err)
		require.Equal(t, "liaison", q.Get("dialName"))
		require.Equal(t, "0", q.Get("rwSeparate"))
		require.Equal(t, "off", q.Get("logLevel"))
		require.Empty(t, q.Get("host"))
	}
	require.Equal(t, `"A""B"`, QuoteIdentifier(`A"B`))
}

func TestDamengRoutesIsolatedAndRevocable(t *testing.T) {
	r := registry{routes: make(map[string]route)}
	_, _, err := r.add(Options{Host: "db", Port: 5236, Username: "wrong:user"}, func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("ambiguous username must not dial")
		return nil, nil
	})
	require.Error(t, err)
	require.Empty(t, r.routes)
	for _, port := range []int{0, -1, 65536} {
		_, _, err := r.add(Options{Host: "db", Port: port, Username: "u"}, func(context.Context, string, string) (net.Conn, error) { panic("not called") })
		require.Error(t, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key, revoke, err := r.add(Options{Host: "db", Port: 5236, Username: "u"}, func(ctx context.Context, network, address string) (net.Conn, error) {
				if network != "tcp" || address != "db:5236" {
					return nil, errors.New("wrong target")
				}
				return nil, errors.New("expected tunnel")
			})
			if err != nil {
				t.Error(err)
				return
			}
			defer revoke()
			_, err = r.dial(context.Background(), key)
			if err == nil || err.Error() != "expected tunnel" {
				t.Errorf("wrong route: %v", err)
			}
			revoke()
			revoke()
			_, err = r.dial(context.Background(), key)
			if err == nil {
				t.Error("revoked route accepted")
			}
		}()
	}
	wg.Wait()
	_, err = r.dial(context.Background(), "other:5236")
	require.Error(t, err)
	require.Empty(t, r.routes)
}

func TestDamengRevokeCancelsInFlightDial(t *testing.T) {
	r := registry{routes: make(map[string]route)}
	started := make(chan struct{})
	done := make(chan error, 1)
	key, revoke, err := r.add(Options{Host: "db", Port: 5236, Username: "u"}, func(ctx context.Context, _, _ string) (net.Conn, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	require.NoError(t, err)
	go func() { _, err := r.dial(context.Background(), key); done <- err }()
	<-started
	revoke()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("revoke did not cancel dial")
	}
}

func TestDamengNativeDriverUsesTunnel(t *testing.T) {
	require.True(t, Available(), "standard builds must include the Dameng driver")
	calls := 0
	_, revoke, err := Open(context.Background(), Options{Host: "private.example", Port: 5236, Username: "u@中文", Password: "do-not-expose:/?#%&+"}, func(ctx context.Context, network, address string) (net.Conn, error) {
		calls++
		require.Equal(t, "private.example:5236", address)
		return nil, errors.New("sentinel")
	})
	require.Error(t, err)
	require.Nil(t, revoke)
	require.Greater(t, calls, 0)
	require.False(t, strings.Contains(err.Error(), "do-not-expose"))
	tunnels.RLock()
	defer tunnels.RUnlock()
	require.Empty(t, tunnels.routes)
}
