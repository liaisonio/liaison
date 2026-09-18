// Package dameng routes the built-in DM8 driver through authorized connectors.
package dameng

import (
	"context"
	"database/sql"
	"net"

	dm "gitee.com/chunanyong/dm"
)

// Available reports whether this build includes the native driver.
func Available() bool { return true }

func init() {
	dm.RegisterDialContext("liaison", func(ctx context.Context, address string) (net.Conn, error) {
		return tunnels.dial(ctx, address)
	})
}

func openDriver(dsn string) (*sql.DB, error) { return sql.Open("dm", dsn) }
