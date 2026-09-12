package web

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Search requests deliberately expose an index-scoped subset, not an HTTP proxy.
// In particular, remote reindex, snapshot repositories and security APIs are absent.
type searchCommand struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
}

var searchIndexName = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,254}$`)
var searchDocumentID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,512}$`)

func parseSearchCommand(statement string) (searchCommand, error) {
	var cmd searchCommand
	dec := json.NewDecoder(strings.NewReader(statement))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cmd); err != nil {
		return cmd, fmt.Errorf("expected JSON {method,path,body}: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return cmd, errors.New("expected exactly one JSON command")
	}
	cmd.Method = strings.ToUpper(cmd.Method)
	if cmd.Method != "GET" && cmd.Method != "POST" && cmd.Method != "PUT" && cmd.Method != "DELETE" {
		return cmd, errors.New("unsupported method")
	}
	// Encoded paths, redirects, query parameters and non-index APIs are not accepted.
	parts := strings.Split(strings.TrimPrefix(cmd.Path, "/"), "/")
	if !strings.HasPrefix(cmd.Path, "/") || len(parts) == 0 || !searchIndexName.MatchString(parts[0]) {
		return cmd, errors.New("use an explicit index path, not a URL or cluster API")
	}
	valid := false
	switch len(parts) {
	case 1:
		valid = cmd.Method == "GET" || cmd.Method == "PUT" || cmd.Method == "DELETE"
	case 2:
		switch parts[1] {
		case "_search", "_count":
			valid = cmd.Method == "GET" || cmd.Method == "POST"
		case "_mapping":
			valid = cmd.Method == "GET" || cmd.Method == "PUT"
		case "_doc":
			valid = cmd.Method == "POST"
		}
	case 3:
		if searchDocumentID.MatchString(parts[2]) {
			valid = parts[1] == "_doc" && (cmd.Method == "GET" || cmd.Method == "PUT" || cmd.Method == "DELETE") || parts[1] == "_update" && cmd.Method == "POST"
		}
	}
	if !valid {
		return cmd, errors.New("unsupported index operation")
	}
	if len(cmd.Body) > 0 {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(cmd.Body, &object); err != nil || object == nil {
			return cmd, errors.New("body must be a JSON object")
		}
	}
	return cmd, nil
}

func (web *web) openWebDataSearch(ctx context.Context, s *webDataSession, password string) error {
	return s.openSearch(ctx, password, web.webDataDeadlineSafeDialContext(s.proxyID, s.protocol))
}

func (s *webDataSession) openSearch(ctx context.Context, password string, dial func(context.Context, string, string) (net.Conn, error)) error {
	if strings.TrimSpace(s.connectionParams) != "" {
		return errors.New("custom connection parameters are not supported")
	}
	if s.database != "" && !searchIndexName.MatchString(s.database) {
		return errors.New("invalid default index")
	}
	scheme := "http"
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.target.TargetHost}
	switch s.tlsMode {
	case "", "disable", "false":
	case "require", "true":
		scheme = "https"
	case "skip-verify":
		scheme = "https"
		tlsConfig.InsecureSkipVerify = true // Explicit user choice, consistent with WebData TLS controls.
	default:
		return errors.New("unsupported TLS mode")
	}
	address := net.JoinHostPort(s.target.TargetHost, strconv.Itoa(s.target.TargetPort))
	transport := &http.Transport{DialContext: func(ctx context.Context, network, target string) (net.Conn, error) {
		if target != address {
			return nil, errors.New("search target override is forbidden")
		}
		return dial(ctx, network, target)
	}, TLSClientConfig: tlsConfig, ResponseHeaderTimeout: webDataExecuteTimeout, MaxIdleConnsPerHost: 2}
	s.searchClient = &http.Client{Transport: transport, Timeout: webDataExecuteTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("search redirects are disabled") }}
	s.searchURL = scheme + "://" + net.JoinHostPort(s.target.TargetHost, strconv.Itoa(s.target.TargetPort))
	s.searchPassword = password
	_, err := s.searchRequest(ctx, http.MethodGet, "/", nil)
	if err != nil {
		transport.CloseIdleConnections()
		s.searchClient = nil
		s.searchPassword = ""
	}
	return err
}

func (s *webDataSession) searchRequest(ctx context.Context, method, path string, body []byte) (map[string]any, error) {
	if s.searchClient == nil {
		return nil, errors.New("search session is not connected")
	}
	req, err := http.NewRequestWithContext(ctx, method, s.searchURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if s.username != "" || s.searchPassword != "" {
		req.SetBasicAuth(s.username, s.searchPassword)
	}
	resp, err := s.searchClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()
	const maxBytes = 4 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, errors.New("response exceeds 4 MiB; narrow the query")
	}
	var result map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("search returned HTTP %d with invalid JSON", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := json.Marshal(result["error"])
		return nil, fmt.Errorf("search HTTP %d: %.1024s", resp.StatusCode, detail)
	}
	return result, nil
}

func (s *webDataSession) executeSearch(ctx context.Context, statement string) (*webDataExecuteResponse, error) {
	cmd, err := parseSearchCommand(statement)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(cmd.Path, "/_search") {
		body := map[string]json.RawMessage{}
		if len(cmd.Body) > 0 {
			if err = json.Unmarshal(cmd.Body, &body); err != nil {
				return nil, err
			}
		}
		size := 100
		if raw, ok := body["size"]; ok {
			if err = json.Unmarshal(raw, &size); err != nil || size < 0 || size > 500 {
				return nil, errors.New("search size must be between 0 and 500")
			}
		}
		body["size"] = json.RawMessage(strconv.Itoa(size))
		cmd.Body, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	result, err := s.searchRequest(ctx, cmd.Method, cmd.Path, cmd.Body)
	if err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	response := &webDataExecuteResponse{Type: "message", Message: string(encoded)}
	if timedOut, _ := result["timed_out"].(bool); timedOut {
		return response, errors.New("search timed out; results may be incomplete")
	}
	if shards, ok := result["_shards"].(map[string]any); ok {
		if failed, ok := shards["failed"].(json.Number); ok && failed != "0" {
			return response, errors.New("some search shards failed; results may be incomplete")
		}
	}
	if hits, ok := result["hits"].(map[string]any); ok {
		if rows, ok := hits["hits"].([]any); ok {
			response.Type = "rows"
			response.Columns = []string{"_index", "_id", "_score", "_source"}
			for _, row := range rows {
				if item, ok := row.(map[string]any); ok {
					response.Rows = append(response.Rows, item)
				}
			}
		}
	}
	return response, nil
}

func (s *webDataSession) searchMetadata(ctx context.Context) ([]webDataMetadataNode, error) {
	path := "/_alias"
	if s.database != "" {
		path = "/" + url.PathEscape(s.database) + "/_alias"
	}
	result, err := s.searchRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(result))
	for name := range result {
		if searchIndexName.MatchString(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	nodes := make([]webDataMetadataNode, 0, len(names))
	for _, name := range names {
		if len(nodes) >= 200 {
			break
		}
		nodes = append(nodes, webDataMetadataNode{Key: "search-index-" + name, Title: name, Type: "index", Meta: map[string]string{"name": name}})
	}
	if len(names) > 200 {
		nodes = append(nodes, webDataMetadataNode{Key: "search-indices-truncated", Type: "info", Title: "Showing first 200 indices; set a default index to narrow the list"})
	}
	return nodes, nil
}

func (s *webDataSession) searchObjectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	if req.ObjectType != "index" || !searchIndexName.MatchString(req.Name) {
		return nil, errors.New("select a valid index")
	}
	result, err := s.searchRequest(ctx, "GET", "/"+req.Name+"/_mapping", nil)
	if err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	detail := &webDataObjectResponse{ObjectType: "index", Name: req.Name, DDL: string(encoded), Message: "Mapping"}
	if index, ok := result[req.Name].(map[string]any); ok {
		if mapping, ok := index["mappings"].(map[string]any); ok {
			var visit func(map[string]any, string, int)
			visit = func(properties map[string]any, prefix string, depth int) {
				if depth > 16 {
					return
				}
				names := make([]string, 0, len(properties))
				for name := range properties {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					if len(detail.Columns) >= 500 {
						return
					}
					field, ok := properties[name].(map[string]any)
					if !ok {
						continue
					}
					typ, _ := field["type"].(string)
					if typ == "" {
						typ = "object"
					}
					detail.Columns = append(detail.Columns, map[string]any{"field": prefix + name, "type": typ})
					for _, key := range []string{"properties", "fields"} {
						if nested, ok := field[key].(map[string]any); ok {
							visit(nested, prefix+name+".", depth+1)
						}
					}
				}
			}
			if properties, ok := mapping["properties"].(map[string]any); ok {
				visit(properties, "", 0)
			}
		}
	}
	return detail, nil
}

func searchIsQuery(statement string) bool {
	cmd, err := parseSearchCommand(statement)
	if err != nil {
		return false
	}
	return cmd.Method == "GET" || cmd.Method == "POST" && (strings.HasSuffix(cmd.Path, "/_search") || strings.HasSuffix(cmd.Path, "/_count"))
}
