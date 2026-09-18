package dameng

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

var ErrUnavailable = errors.New("Dameng driver is not included in this build")

type DialContext func(context.Context, string, string) (net.Conn, error)

// Options deliberately excludes arbitrary DSN properties, redirects and local files.
type Options struct {
	Host, Username, Password string
	Port                     int
}

type route struct {
	address string
	dial    DialContext
	ctx     context.Context
	cancel  context.CancelFunc
}
type registry struct {
	sync.RWMutex
	routes map[string]route
}

// The native driver's dial hook is registered once at initialization. Session
// registrations live here, not in a potentially unsynchronized third-party map.
var tunnels = registry{routes: make(map[string]route)}

func (r *registry) add(options Options, dial DialContext) (string, func(), error) {
	if dial == nil || options.Host == "" || options.Port < 1 || options.Port > 65535 || options.Username == "" {
		return "", nil, errors.New("Dameng host, port, username and connector are required")
	}
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", nil, err
	}
	// Never put the real host in the native DSN. An unexpected native dial
	// fails closed even if the driver does not invoke the custom hook.
	key := net.JoinHostPort(hex.EncodeToString(token[:])+".invalid", strconv.Itoa(options.Port))
	r.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	r.routes[key] = route{net.JoinHostPort(options.Host, strconv.Itoa(options.Port)), dial, ctx, cancel}
	r.Unlock()
	return key, func() { cancel(); r.Lock(); delete(r.routes, key); r.Unlock() }, nil
}

func (r *registry) dial(ctx context.Context, address string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.RLock()
	target, ok := r.routes[address]
	r.RUnlock()
	if !ok {
		return nil, errors.New("Dameng connector route is unavailable")
	}
	dialCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(target.ctx, cancel)
	defer stop()
	defer cancel()
	conn, err := target.dial(dialCtx, "tcp", target.address)
	if err != nil {
		return nil, err
	}
	if target.ctx.Err() != nil || ctx.Err() != nil {
		// The session was revoked while a dial was in flight. Do not return its socket.
		if closeErr := conn.Close(); closeErr != nil {
			return nil, errors.New("revoked Dameng connection could not close cleanly")
		}
		return nil, context.Canceled
	}
	return conn, nil
}

func dsn(options Options, address string) string {
	q := url.Values{
		"dialName": {"liaison"}, "connectTimeout": {"15000"}, "socketTimeout": {"30"},
		"rwSeparate": {"0"}, "doSwitch": {"0"}, "driverReconnect": {"false"},
		"logLevel": {"off"}, "statEnable": {"false"}, "maxRows": {"1001"},
		"addressRemap": {""}, "userRemap": {""},
	}
	u := url.URL{Scheme: "dm", Host: address, User: url.UserPassword(options.Username, options.Password), RawQuery: q.Encode()}
	return u.String()
}

// Open returns an idempotent route revocation function. Call it on failed open
// and session close. Dialing is always delegated to the authorized connector.
func Open(ctx context.Context, options Options, dial DialContext) (*sql.DB, func(), error) {
	if !Available() {
		return nil, nil, ErrUnavailable
	}
	address, revoke, err := tunnels.add(options, dial)
	if err != nil {
		return nil, nil, err
	}
	db, err := openDriver(dsn(options, address))
	if err != nil {
		revoke()
		return nil, nil, errors.New("Dameng driver initialization failed")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err = db.PingContext(ctx); err != nil {
		revoke()
		// Preserve cancellation, but never propagate a driver error containing a DSN.
		closeErr := db.Close()
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		if closeErr != nil {
			return nil, nil, errors.New("Dameng connection failed and could not close cleanly")
		}
		return nil, nil, errors.New("Dameng connection failed; check connector, server and credentials")
	}
	return db, revoke, nil
}

// QuoteIdentifier is for identifiers only; metadata values must use bind parameters.
func QuoteIdentifier(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }
