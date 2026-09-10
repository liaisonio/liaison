package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	mssql "github.com/microsoft/go-mssqldb"
)

type sqlServerDialer struct {
	host string
	dial func(context.Context, string, string) (net.Conn, error)
}

// HostName keeps private DNS resolution on the connector network.
func (d sqlServerDialer) HostName() string { return d.host }

func (d sqlServerDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dial(ctx, network, address)
}

func sqlServerDSN(s *webDataSession, password string) (string, error) {
	params, err := parseWebDataConnectionParams(s.connectionParams)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	// Do not allow custom parameters to redirect the connection or replace auth.
	for key, value := range params {
		switch strings.ToLower(key) {
		case "app name", "applicationintent", "packet size":
			q.Set(strings.ToLower(key), value)
		default:
			return "", fmt.Errorf("unsupported SQL Server parameter %q", key)
		}
	}
	q.Set("database", firstNonEmpty(s.database, "master"))
	q.Set("connection timeout", "15")
	q.Set("encrypt", "true")
	q.Set("TrustServerCertificate", "false")
	switch s.tlsMode {
	case "disable":
		q.Set("encrypt", "disable")
	case "skip-verify":
		q.Set("TrustServerCertificate", "true")
	case "", "require", "true":
	default:
		return "", errors.New("unsupported SQL Server TLS mode")
	}
	u := url.URL{Scheme: "sqlserver", Host: net.JoinHostPort(s.target.TargetHost, strconv.Itoa(s.target.TargetPort)), User: url.UserPassword(s.username, password), RawQuery: q.Encode()}
	return u.String(), nil
}

func (web *web) openWebDataSQLServer(ctx context.Context, s *webDataSession, password string) error {
	dsn, err := sqlServerDSN(s, password)
	if err != nil {
		return err
	}
	connector, err := mssql.NewConnector(dsn)
	if err != nil {
		return errors.New("invalid SQL Server connection configuration")
	}
	connector.Dialer = sqlServerDialer{host: s.target.TargetHost, dial: web.webDataDeadlineSafeDialContext(s.proxyID, "sqlserver")}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(webDataSessionTTL)
	if err := db.PingContext(ctx); err != nil {
		db.Close() // Failed connection is not retained.
		return err
	}
	s.sqlDB = db
	s.database = firstNonEmpty(s.database, "master")
	return nil
}

func (s *webDataSession) sqlServerMetadata(ctx context.Context) ([]webDataMetadataNode, error) {
	rows, err := s.sqlDB.QueryContext(ctx, `SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM INFORMATION_SCHEMA.TABLES ORDER BY TABLE_SCHEMA, TABLE_NAME`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := []webDataTableRef{}
	for rows.Next() {
		var table webDataTableRef
		if err := rows.Scan(&table.Namespace, &table.Name, &table.Kind); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return buildPostgresMetadata(tables), nil // Same schema/table tree shape.
}

func (s *webDataSession) sqlServerColumns(ctx context.Context, schema, name string) ([]map[string]any, error) {
	return querySQLRowsAsMaps(ctx, s.sqlDB, `SELECT COLUMN_NAME AS column_name, DATA_TYPE AS data_type,
IS_NULLABLE AS is_nullable, COLUMN_DEFAULT AS column_default, CHARACTER_MAXIMUM_LENGTH AS character_maximum_length,
NUMERIC_PRECISION AS numeric_precision, NUMERIC_SCALE AS numeric_scale,
COLUMNPROPERTY(OBJECT_ID(QUOTENAME(TABLE_SCHEMA)+'.'+QUOTENAME(TABLE_NAME)), COLUMN_NAME, 'IsIdentity') AS is_identity,
COLUMNPROPERTY(OBJECT_ID(QUOTENAME(TABLE_SCHEMA)+'.'+QUOTENAME(TABLE_NAME)), COLUMN_NAME, 'IsComputed') AS is_computed
FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=@p1 AND TABLE_NAME=@p2 ORDER BY ORDINAL_POSITION`, schema, name)
}

func (s *webDataSession) sqlServerMetadataChildren(ctx context.Context, req webDataMetadataRequest) ([]webDataMetadataNode, error) {
	if req.NodeType != "table" || req.Name == "" {
		return nil, errors.New("invalid SQL Server metadata node")
	}
	schema := firstNonEmpty(req.Schema, "dbo")
	rows, err := s.sqlServerColumns(ctx, schema, req.Name)
	if err != nil {
		return nil, err
	}
	nodes := make([]webDataMetadataNode, 0, len(rows))
	for _, row := range rows {
		column := fmt.Sprint(row["column_name"])
		nodes = append(nodes, webDataMetadataNode{Key: "sqlserver-column-" + schema + "-" + req.Name + "-" + column, Title: column, Type: "column", Value: fmt.Sprint(row["data_type"]), Meta: map[string]string{"schema": schema, "name": req.Name, "column": column}})
	}
	return nodes, nil
}

func (s *webDataSession) sqlServerObjectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	if req.ObjectType != "table" || req.Name == "" {
		return nil, errors.New("select a SQL Server table or view")
	}
	schema := firstNonEmpty(req.Schema, "dbo")
	columns, err := s.sqlServerColumns(ctx, schema, req.Name)
	if err != nil {
		return nil, err
	}
	indexes, err := querySQLRowsAsMaps(ctx, s.sqlDB, `SELECT i.name AS indexname, i.is_primary_key, i.is_unique,
c.name AS column_name, ic.key_ordinal, ic.is_included_column
FROM sys.indexes i JOIN sys.objects o ON o.object_id=i.object_id
JOIN sys.schemas s ON s.schema_id=o.schema_id
JOIN sys.index_columns ic ON ic.object_id=i.object_id AND ic.index_id=i.index_id
JOIN sys.columns c ON c.object_id=ic.object_id AND c.column_id=ic.column_id
WHERE s.name=@p1 AND o.name=@p2 ORDER BY i.name, ic.key_ordinal`, schema, req.Name)
	if err != nil {
		return nil, err
	}
	return &webDataObjectResponse{ObjectType: "table", Database: s.database, Schema: schema, Name: req.Name, Columns: columns, Indexes: indexes, Message: schema + "." + req.Name}, nil
}
