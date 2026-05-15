package web

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestWebDataShouldAuditExecuteSQL(t *testing.T) {
	tests := []struct {
		name      string
		protocol  string
		statement string
		want      bool
	}{
		{name: "select is query", protocol: "mysql", statement: "SELECT * FROM users", want: false},
		{name: "show is query", protocol: "mysql", statement: "SHOW TABLES", want: false},
		{name: "explain is query", protocol: "postgresql", statement: "EXPLAIN UPDATE users SET name = 'a'", want: false},
		{name: "explain analyze select is query", protocol: "postgresql", statement: "EXPLAIN ANALYZE SELECT * FROM users", want: false},
		{name: "explain analyze update is audited", protocol: "postgresql", statement: "EXPLAIN ANALYZE UPDATE users SET name = 'a'", want: true},
		{name: "explain analyze option update is audited", protocol: "postgresql", statement: "EXPLAIN (ANALYZE, BUFFERS) DELETE FROM users WHERE id = 1", want: true},
		{name: "explain literal analyze update is query", protocol: "postgresql", statement: "EXPLAIN SELECT 'analyze update users'", want: false},
		{name: "with select is query", protocol: "postgresql", statement: "WITH q AS (SELECT 1) SELECT * FROM q", want: false},
		{name: "multi select is query", protocol: "mysql", statement: "SELECT 1; SELECT 2", want: false},
		{name: "select then update is audited", protocol: "mysql", statement: "SELECT 1; UPDATE users SET name = 'a'", want: true},
		{name: "select then drop is audited with literal semicolon", protocol: "mysql", statement: "SELECT ';'; DROP TABLE users", want: true},
		{name: "select with literal write word is query", protocol: "mysql", statement: "SELECT 'update users set name = a'", want: false},
		{name: "insert is audited", protocol: "mysql", statement: "INSERT INTO users(id) VALUES (1)", want: true},
		{name: "update returning is audited", protocol: "postgresql", statement: "UPDATE users SET name = 'a' RETURNING *", want: true},
		{name: "with delete is audited", protocol: "postgresql", statement: "WITH deleted AS (DELETE FROM users RETURNING *) SELECT * FROM deleted", want: true},
		{name: "drop is audited", protocol: "mysql", statement: "DROP TABLE users", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := webDataShouldAuditExecute(tt.protocol, tt.statement); got != tt.want {
				t.Fatalf("webDataShouldAuditExecute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoteClientIPInfoTrustsForwardedHeadersOnlyFromLoopback(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		wantIP     string
		wantSource string
	}{
		{
			name:       "untrusted peer ignores spoofed real ip",
			remoteAddr: "203.0.113.10:4231",
			headers:    map[string]string{"X-Real-IP": "198.51.100.7"},
			wantIP:     "203.0.113.10",
			wantSource: "remote_addr",
		},
		{
			name:       "loopback peer can pass real ip",
			remoteAddr: "127.0.0.1:4231",
			headers:    map[string]string{"X-Real-IP": "198.51.100.7"},
			wantIP:     "198.51.100.7",
			wantSource: "x-real-ip",
		},
		{
			name:       "loopback peer can pass forwarded for",
			remoteAddr: "[::1]:4231",
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.8, 10.0.0.1"},
			wantIP:     "198.51.100.8",
			wantSource: "x-forwarded-for",
		},
		{
			name:       "invalid forwarded header falls back",
			remoteAddr: "127.0.0.1:4231",
			headers:    map[string]string{"X-Real-IP": "not-an-ip"},
			wantIP:     "127.0.0.1",
			wantSource: "remote_addr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/audit", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			gotIP, gotSource := remoteClientIPInfo(req)
			if gotIP != tt.wantIP || gotSource != tt.wantSource {
				t.Fatalf("remoteClientIPInfo() = %q/%q, want %q/%q", gotIP, gotSource, tt.wantIP, tt.wantSource)
			}
		})
	}
}

func TestWebDataPostgresSSLMode(t *testing.T) {
	tests := []struct {
		tlsMode string
		want    string
	}{
		{tlsMode: "", want: "disable"},
		{tlsMode: "disable", want: "disable"},
		{tlsMode: "require", want: "require"},
		{tlsMode: "true", want: "require"},
		{tlsMode: "skip-verify", want: "require"},
		{tlsMode: "preferred", want: "prefer"},
	}
	for _, tt := range tests {
		t.Run(tt.tlsMode, func(t *testing.T) {
			if got := webDataPostgresSSLMode(tt.tlsMode); got != tt.want {
				t.Fatalf("webDataPostgresSSLMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateWebDataSessionRequestDirectConnectionPresence(t *testing.T) {
	var explicit createWebDataSessionRequest
	if err := json.Unmarshal([]byte(`{"direct_connection":false}`), &explicit); err != nil {
		t.Fatal(err)
	}
	if !explicit.DirectConnectionSet {
		t.Fatal("DirectConnectionSet = false, want true")
	}
	if explicit.DirectConnection {
		t.Fatal("DirectConnection = true, want false")
	}

	var omitted createWebDataSessionRequest
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.DirectConnectionSet {
		t.Fatal("DirectConnectionSet = true, want false")
	}
}

func TestApplyWebDataCredentialDefaultsKeepsExplicitDirectConnectionFalse(t *testing.T) {
	req := &createWebDataSessionRequest{
		DirectConnection:    false,
		DirectConnectionSet: true,
	}
	applyWebDataCredentialDefaults(req, &controlplane.WebDataCredentialSecret{
		Protocol:         "mongodb",
		Username:         "liaison",
		DirectConnection: true,
	})
	if req.DirectConnection {
		t.Fatal("DirectConnection = true, want explicit false")
	}

	omitted := &createWebDataSessionRequest{}
	applyWebDataCredentialDefaults(omitted, &controlplane.WebDataCredentialSecret{
		DirectConnection: true,
	})
	if !omitted.DirectConnection {
		t.Fatal("DirectConnection = false, want saved true")
	}
}

func TestApplyWebDataCredentialDefaultsKeepsExplicitRedisDBZero(t *testing.T) {
	var req createWebDataSessionRequest
	if err := json.Unmarshal([]byte(`{"redis_db":0}`), &req); err != nil {
		t.Fatal(err)
	}
	if !req.RedisDBSet {
		t.Fatal("RedisDBSet = false, want true")
	}
	applyWebDataCredentialDefaults(&req, &controlplane.WebDataCredentialSecret{
		RedisDB: 5,
	})
	if req.RedisDB != 0 {
		t.Fatalf("RedisDB = %d, want explicit 0", req.RedisDB)
	}

	var omitted createWebDataSessionRequest
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatal(err)
	}
	applyWebDataCredentialDefaults(&omitted, &controlplane.WebDataCredentialSecret{
		RedisDB: 5,
	})
	if omitted.RedisDB != 5 {
		t.Fatalf("RedisDB = %d, want saved 5", omitted.RedisDB)
	}
}

func TestWebDataShouldAuditExecuteRedis(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      bool
	}{
		{name: "get is query", statement: "GET user:1", want: false},
		{name: "scan is query", statement: "SCAN 0", want: false},
		{name: "hgetall is query", statement: "HGETALL user:1", want: false},
		{name: "set is audited", statement: "SET user:1 alice", want: true},
		{name: "del is audited", statement: "DEL user:1", want: true},
		{name: "config set is audited", statement: "CONFIG SET maxmemory 1mb", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := webDataShouldAuditExecute("redis", tt.statement); got != tt.want {
				t.Fatalf("webDataShouldAuditExecute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWebDataShouldAuditExecuteMongo(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      bool
	}{
		{name: "find is query", statement: `{ "find": "users", "filter": {} }`, want: false},
		{name: "count is query", statement: `{ "count": "users" }`, want: false},
		{name: "aggregate read is query", statement: `{ "aggregate": "users", "pipeline": [], "cursor": {} }`, want: false},
		{name: "aggregate out is audited", statement: `{ "aggregate": "users", "pipeline": [{ "$out": "users_copy" }], "cursor": {} }`, want: true},
		{name: "insert is audited", statement: `{ "insert": "users", "documents": [{ "name": "alice" }] }`, want: true},
		{name: "update is audited", statement: `{ "update": "users", "updates": [] }`, want: true},
		{name: "invalid command is audited", statement: `{`, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := webDataShouldAuditExecute("mongodb", tt.statement); got != tt.want {
				t.Fatalf("webDataShouldAuditExecute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeMongoObjectIDFilter(t *testing.T) {
	const hexID = "6a06a68baf1d59ac10b95539"
	objectID, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		filter any
		assert func(t *testing.T, got any)
	}{
		{
			name:   "direct id string",
			filter: bson.D{{Key: "_id", Value: hexID}},
			assert: func(t *testing.T, got any) {
				doc := got.(bson.D)
				if doc[0].Value != objectID {
					t.Fatalf("_id = %#v, want %#v", doc[0].Value, objectID)
				}
			},
		},
		{
			name: "operator id string",
			filter: bson.D{{Key: "_id", Value: bson.D{
				{Key: "$ne", Value: hexID},
				{Key: "$in", Value: bson.A{hexID}},
			}}},
			assert: func(t *testing.T, got any) {
				doc := got.(bson.D)
				operators := doc[0].Value.(bson.D)
				if operators[0].Value != objectID {
					t.Fatalf("$ne = %#v, want %#v", operators[0].Value, objectID)
				}
				inValues := operators[1].Value.(bson.A)
				if inValues[0] != objectID {
					t.Fatalf("$in[0] = %#v, want %#v", inValues[0], objectID)
				}
			},
		},
		{
			name: "and nested id string",
			filter: bson.D{{Key: "$and", Value: bson.A{
				bson.D{{Key: "_id", Value: hexID}},
				bson.D{{Key: "name", Value: hexID}},
			}}},
			assert: func(t *testing.T, got any) {
				doc := got.(bson.D)
				items := doc[0].Value.(bson.A)
				idDoc := items[0].(bson.D)
				nameDoc := items[1].(bson.D)
				if idDoc[0].Value != objectID {
					t.Fatalf("nested _id = %#v, want %#v", idDoc[0].Value, objectID)
				}
				if nameDoc[0].Value != hexID {
					t.Fatalf("name = %#v, want raw string", nameDoc[0].Value)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, normalizeMongoObjectIDFilter(tt.filter))
		})
	}
}
