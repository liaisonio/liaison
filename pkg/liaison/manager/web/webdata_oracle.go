package web

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	go_ora "github.com/sijms/go-ora/v2"
)

var oracleServiceName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var oraclePLSQLBlock = regexp.MustCompile(`(?is)^\s*(begin\b|declare\b|create\s+(or\s+replace\s+)?(procedure|function|package|trigger|type)\b)`)

func oracleDSN(s *webDataSession, password string) (string, error) {
	if !oracleServiceName.MatchString(s.database) {
		return "", errors.New("a valid Oracle Service Name is required")
	}
	if strings.TrimSpace(s.connectionParams) != "" {
		return "", errors.New("custom Oracle connection parameters are not supported")
	}
	q := url.Values{"CONNECTION TIMEOUT": {"15"}, "TIMEOUT": {"30"}}
	switch s.tlsMode {
	case "", "disable":
	case "require", "true":
		q.Set("SSL", "true")
		q.Set("SSL VERIFY", "true")
	case "skip-verify":
		q.Set("SSL", "true")
		q.Set("SSL VERIFY", "false")
	default:
		return "", errors.New("unsupported Oracle TLS mode")
	}
	u := url.URL{Scheme: "oracle", Host: net.JoinHostPort(s.target.TargetHost, strconv.Itoa(s.target.TargetPort)), User: url.UserPassword(s.username, password), Path: "/" + s.database, RawQuery: q.Encode()}
	return u.String(), nil
}

type oracleDialer struct {
	address string
	dial    func(context.Context, string, string) (net.Conn, error)
}

func (d oracleDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// Listener redirects must not expand the authorized application target.
	if !strings.EqualFold(address, d.address) {
		return nil, errors.New("Oracle listener redirects are not supported")
	}
	return d.dial(ctx, network, address)
}

func (web *web) openWebDataOracle(ctx context.Context, s *webDataSession, password string) error {
	dsn, err := oracleDSN(s, password)
	if err != nil {
		return err
	}
	connector := go_ora.NewConnector(dsn).(*go_ora.OracleConnector)
	connector.Dialer(oracleDialer{address: net.JoinHostPort(s.target.TargetHost, strconv.Itoa(s.target.TargetPort)), dial: web.webDataDeadlineSafeDialContext(s.proxyID, "oracle")})
	connector.WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.target.TargetHost, InsecureSkipVerify: s.tlsMode == "skip-verify"}) // Explicit user-selected compatibility mode only.
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(webDataSessionTTL)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return err
	}
	if s.schema == "" {
		if err = db.QueryRowContext(ctx, `SELECT SYS_CONTEXT('USERENV','CURRENT_SCHEMA') FROM dual`).Scan(&s.schema); err != nil {
			db.Close()
			return err
		}
	} else {
		// Identifiers cannot be bound; quote them without expanding SQL syntax.
		if _, err = db.ExecContext(ctx, `ALTER SESSION SET CURRENT_SCHEMA = "`+strings.ReplaceAll(s.schema, `"`, `""`)+`"`); err != nil {
			db.Close()
			return err
		}
	}
	s.sqlDB = db
	return nil
}

func oracleStatement(statement string) string {
	statement = strings.TrimSpace(statement)
	if !oraclePLSQLBlock.MatchString(statement) {
		statement = strings.TrimSpace(strings.TrimSuffix(statement, ";"))
	}
	return statement
}

func (s *webDataSession) oracleMetadata(ctx context.Context) ([]webDataMetadataNode, error) {
	rows, err := s.sqlDB.QueryContext(ctx, `SELECT owner, object_name, object_type FROM all_objects WHERE owner=:1 AND object_type IN ('TABLE','VIEW') ORDER BY object_name`, s.schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := []webDataTableRef{}
	for rows.Next() {
		var table webDataTableRef
		if err = rows.Scan(&table.Namespace, &table.Name, &table.Kind); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return buildPostgresMetadata(tables), nil
}

func (s *webDataSession) oracleColumns(ctx context.Context, schema, name string) ([]map[string]any, error) {
	return querySQLRowsAsMaps(ctx, s.sqlDB, `SELECT column_name AS "column_name", data_type AS "data_type",
 CASE nullable WHEN 'Y' THEN 'YES' ELSE 'NO' END AS "is_nullable", data_default AS "column_default",
 char_length AS "character_maximum_length", data_precision AS "numeric_precision", data_scale AS "numeric_scale",
 CASE identity_column WHEN 'YES' THEN 1 ELSE 0 END AS "is_identity",
 CASE virtual_column WHEN 'YES' THEN 1 ELSE 0 END AS "is_computed"
 FROM all_tab_cols WHERE owner=:1 AND table_name=:2 AND hidden_column='NO' ORDER BY column_id`, schema, name)
}

func (s *webDataSession) oracleMetadataChildren(ctx context.Context, req webDataMetadataRequest) ([]webDataMetadataNode, error) {
	if req.NodeType != "table" || req.Name == "" {
		return nil, errors.New("select an Oracle table or view")
	}
	schema := firstNonEmpty(req.Schema, s.schema)
	columns, err := s.oracleColumns(ctx, schema, req.Name)
	if err != nil {
		return nil, err
	}
	nodes := make([]webDataMetadataNode, 0, len(columns))
	for _, col := range columns {
		name := fmt.Sprint(col["column_name"])
		nodes = append(nodes, webDataMetadataNode{Key: "oracle-column-" + schema + "-" + req.Name + "-" + name, Title: name, Type: "column", Value: fmt.Sprint(col["data_type"]), Meta: map[string]string{"schema": schema, "name": req.Name, "column": name}})
	}
	return nodes, nil
}

func (s *webDataSession) oracleObjectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	if req.ObjectType != "table" || req.Name == "" {
		return nil, errors.New("select an Oracle table or view")
	}
	schema := firstNonEmpty(req.Schema, s.schema)
	columns, err := s.oracleColumns(ctx, schema, req.Name)
	if err != nil {
		return nil, err
	}
	indexes, err := querySQLRowsAsMaps(ctx, s.sqlDB, `SELECT i.index_name AS "indexname", c.column_name AS "column_name", c.column_position AS "key_ordinal",
 CASE WHEN EXISTS (SELECT 1 FROM all_constraints p WHERE p.owner=i.table_owner AND p.table_name=i.table_name AND p.index_name=i.index_name AND p.constraint_type='P') THEN 1 ELSE 0 END AS "is_primary_key",
 CASE i.uniqueness WHEN 'UNIQUE' THEN 1 ELSE 0 END AS "is_unique"
 FROM all_indexes i JOIN all_ind_columns c ON c.index_owner=i.owner AND c.index_name=i.index_name
 WHERE i.table_owner=:1 AND i.table_name=:2 ORDER BY i.index_name,c.column_position`, schema, req.Name)
	if err != nil {
		return nil, err
	}
	return &webDataObjectResponse{ObjectType: "table", Database: s.database, Schema: schema, Name: req.Name, Columns: columns, Indexes: indexes, Message: schema + "." + req.Name}, nil
}
