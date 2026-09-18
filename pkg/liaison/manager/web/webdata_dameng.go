package web

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/liaisonio/liaison/pkg/dameng"
)

func safeDamengError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Reconnect errors can contain native connection details. Do not expose
	// driver error strings through query, metadata or Agent responses.
	return errors.New("Dameng operation failed; check the statement, schema and database permissions")
}

func (web *web) openWebDataDameng(ctx context.Context, s *webDataSession, password string) error {
	if s.tlsMode != "" && s.tlsMode != "disable" {
		return errors.New("Dameng database TLS is not yet supported; connector encryption is separate")
	}
	if s.connectionParams != "" || s.database != "" {
		return errors.New("Dameng uses Schema; database names and custom connection parameters are not supported")
	}
	db, revoke, err := dameng.Open(ctx, dameng.Options{Host: s.target.TargetHost, Port: s.target.TargetPort, Username: s.username, Password: password}, web.webDataDeadlineSafeDialContext(s.proxyID, "dameng"))
	if err != nil {
		return err
	}
	s.sqlDB, s.sqlRevoke = db, revoke
	if s.schema != "" {
		_, err = db.ExecContext(ctx, "SET SCHEMA "+dameng.QuoteIdentifier(s.schema))
	} else {
		err = db.QueryRowContext(ctx, "SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')").Scan(&s.schema)
	}
	if err != nil {
		s.close()
		return errors.New("Dameng schema could not be selected; check the schema and database permissions")
	}
	return nil
}

func (s *webDataSession) damengMetadata(ctx context.Context) ([]webDataMetadataNode, error) {
	rows, err := s.sqlDB.QueryContext(ctx, `SELECT owner, object_name, object_type FROM all_objects WHERE owner=? AND object_type IN ('TABLE','VIEW') ORDER BY object_name`, s.schema)
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

func (s *webDataSession) damengColumns(ctx context.Context, schema, name string) ([]map[string]any, error) {
	return querySQLRowsAsMaps(ctx, s.sqlDB, `SELECT column_name AS "column_name", data_type AS "data_type", CASE nullable WHEN 'Y' THEN 'YES' ELSE 'NO' END AS "is_nullable", data_default AS "column_default", data_length AS "character_maximum_length", data_precision AS "numeric_precision", data_scale AS "numeric_scale" FROM all_tab_columns WHERE owner=? AND table_name=? ORDER BY column_id`, schema, name)
}

func (s *webDataSession) damengMetadataChildren(ctx context.Context, req webDataMetadataRequest) ([]webDataMetadataNode, error) {
	if req.NodeType != "table" || req.Name == "" {
		return nil, errors.New("select a Dameng table or view")
	}
	schema := firstNonEmpty(req.Schema, s.schema)
	columns, err := s.damengColumns(ctx, schema, req.Name)
	if err != nil {
		return nil, err
	}
	nodes := make([]webDataMetadataNode, 0, len(columns))
	for _, column := range columns {
		name := fmt.Sprint(column["column_name"])
		nodes = append(nodes, webDataMetadataNode{Key: "dameng-column-" + schema + "-" + req.Name + "-" + name, Title: name, Type: "column", Value: fmt.Sprint(column["data_type"]), Meta: map[string]string{"schema": schema, "name": req.Name, "column": name}})
	}
	return nodes, nil
}

func (s *webDataSession) damengObjectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	if req.ObjectType != "table" || req.Name == "" {
		return nil, errors.New("select a Dameng table or view")
	}
	schema := firstNonEmpty(req.Schema, s.schema)
	columns, err := s.damengColumns(ctx, schema, req.Name)
	if err != nil {
		return nil, err
	}
	return &webDataObjectResponse{ObjectType: "table", Schema: schema, Name: req.Name, Columns: columns, Message: schema + "." + req.Name}, nil
}

func damengStatement(statement string) string {
	statement = strings.TrimSpace(statement)
	if !oraclePLSQLBlock.MatchString(statement) {
		statement = strings.TrimSpace(strings.TrimSuffix(statement, ";"))
	}
	return statement
}
