package web

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/stretchr/testify/require"
)

func TestSQLServerDSN_TLSAndCredentialEncoding(t *testing.T) {
	for _, mode := range []string{"", "require", "skip-verify", "disable"} {
		t.Run(mode, func(t *testing.T) {
			s := &webDataSession{username: "test@user", database: "app db", tlsMode: mode, target: &controlplane.WebDataTarget{TargetHost: "db.example.test", TargetPort: 1433}}
			dsn, err := sqlServerDSN(s, "test:p@ss/?")
			require.NoError(t, err)
			u, err := url.Parse(dsn)
			require.NoError(t, err)
			password, _ := u.User.Password()
			require.Equal(t, "test:p@ss/?", password)
			require.Equal(t, "app db", u.Query().Get("database"))
			require.Equal(t, mode == "skip-verify", u.Query().Get("TrustServerCertificate") == "true")
			connector, err := mssql.NewConnector(dsn)
			require.NoError(t, err)
			called := false
			blocked := errors.New("test tunnel unavailable")
			connector.Dialer = sqlServerDialer{host: s.target.TargetHost, dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				called = true
				return nil, blocked
			}}
			_, err = connector.Connect(context.Background())
			require.Error(t, err)
			require.True(t, called, "driver must use the injected connector tunnel")
		})
	}
}

func TestSQLServerDSN_RejectsConnectionOverrides(t *testing.T) {
	for _, params := range []string{"server=other.example.test", "user id=sa", "database=other", "TrustServerCertificate=true", "encrypt=disable"} {
		s := &webDataSession{connectionParams: params, target: &controlplane.WebDataTarget{TargetHost: "db.example.test", TargetPort: 1433}}
		_, err := sqlServerDSN(s, "")
		require.Error(t, err)
	}
}
