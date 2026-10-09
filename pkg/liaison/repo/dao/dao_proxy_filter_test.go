package dao

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestProxyProtocolFilterPaginationAndScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "access.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("CREATE TABLE proxies (id INTEGER PRIMARY KEY, name TEXT, access_protocol TEXT, deleted_at DATETIME)").Error)
	require.NoError(t, db.Exec("INSERT INTO proxies (id,name,access_protocol) VALUES (1,'dev-ssh','webssh'), (2,'dev-web','web'), (3,'dev-files','websftp'), (4,'other','webssh')").Error)
	d := &dao{db: db}
	for _, tc := range []struct {
		name      string
		protocols []string
		ids       []uint
		scoped    bool
		page      int
		want      []uint
		total     int64
	}{
		{"all", nil, nil, false, 1, []uint{1, 2}, 3},
		{"filtered", []string{"webssh", "websftp"}, nil, false, 1, []uint{1, 3}, 2},
		{"second page", []string{"webssh", "websftp"}, nil, false, 2, []uint{}, 2},
		{"scoped", []string{"webssh", "websftp"}, []uint{2, 3}, true, 1, []uint{3}, 1},
		{"no grants", []string{"webssh"}, nil, true, 1, []uint{}, 0},
		{"unknown", []string{"future"}, nil, false, 1, []uint{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &ListProxiesQuery{Query: Query{Page: tc.page, PageSize: 2, Order: "id", ScopeApplied: tc.scoped}, Name: "dev", AccessProtocols: tc.protocols, IDs: tc.ids}
			rows, err := d.ListProxies(q)
			require.NoError(t, err)
			got := []uint{}
			for _, row := range rows {
				got = append(got, row.ID)
			}
			require.Equal(t, tc.want, got)
			count, err := d.CountProxies(q)
			require.NoError(t, err)
			require.Equal(t, tc.total, count)
		})
	}
}
