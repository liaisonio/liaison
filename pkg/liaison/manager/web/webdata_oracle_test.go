package web

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	go_ora "github.com/sijms/go-ora/v2"
	"github.com/stretchr/testify/require"
)

func TestOracleDSN_UsesScopedDialer(t *testing.T) {
	for _, mode := range []string{"disable", "require", "skip-verify"} {
		t.Run(mode, func(t *testing.T) {
			s := &webDataSession{username: "demo", database: "FREEPDB1", tlsMode: mode, target: &controlplane.WebDataTarget{TargetHost: "private.example.test", TargetPort: 1521}}
			dsn, err := oracleDSN(s, "p@ss:/?#")
			require.NoError(t, err)
			u, err := url.Parse(dsn)
			require.NoError(t, err)
			password, _ := u.User.Password()
			require.Equal(t, "p@ss:/?#", password)
			called := false
			connector := go_ora.NewConnector(dsn).(*go_ora.OracleConnector)
			connector.Dialer(oracleDialer{address: "private.example.test:1521", dial: func(context.Context, string, string) (net.Conn, error) {
				called = true
				return nil, errors.New("test tunnel unavailable")
			}})
			_, err = connector.Connect(context.Background())
			require.Error(t, err)
			require.True(t, called)
		})
	}
}

func TestOracleDSN_RejectsOverrides(t *testing.T) {
	for _, service := range []string{"", "x)(HOST=other)", "a/b", "a?server=other"} {
		_, err := oracleDSN(&webDataSession{database: service}, "")
		require.Error(t, err)
	}
	_, err := oracleDSN(&webDataSession{database: "FREEPDB1", connectionParams: "SERVER=other"}, "")
	require.Error(t, err)
	d := oracleDialer{address: "db.example.test:1521", dial: func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("redirect must not dial")
		return nil, nil
	}}
	_, err = d.DialContext(context.Background(), "tcp", "other.example.test:1521")
	require.Error(t, err)
}

func TestOracleStatement_Terminators(t *testing.T) {
	require.Equal(t, "SELECT 1 FROM dual", oracleStatement(" SELECT 1 FROM dual; "))
	require.Equal(t, "BEGIN NULL; END;", oracleStatement("BEGIN NULL; END;"))
	require.True(t, webDataExecuteIsQuery("oracle", "SELECT 1 FROM dual"))
}
