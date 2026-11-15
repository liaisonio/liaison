package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// decodeAgentRequest 在 HTTP 边界兼容 protobuf 的 uint64 字符串，不改变内部 RPC 类型。
// dst 是本包已知的请求 DTO；未知字段仍被拒绝，不能注入身份字段。
func decodeAgentRequest(body io.Reader, dst any, requireEdge bool) error {
	d := json.NewDecoder(body)
	var fields map[string]json.RawMessage
	if err := d.Decode(&fields); err != nil {
		return fmt.Errorf("decode agent request: %w", err)
	}
	if fields == nil {
		return fmt.Errorf("invalid agent request")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected trailing body")
	}
	raw, exists := fields["edge_id"]
	if requireEdge && !exists {
		return fmt.Errorf("missing connector")
	}
	if exists {
		value := string(raw)
		if len(raw) > 0 && raw[0] == '"' {
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode connector: %w", err)
			}
		}
		if len(value) == 0 || value[0] < '1' || value[0] > '9' {
			return fmt.Errorf("invalid connector")
		}
		for _, digit := range value {
			if digit < '0' || digit > '9' {
				return fmt.Errorf("invalid connector")
			}
		}
		if _, err := strconv.ParseUint(value, 10, 64); err != nil {
			return fmt.Errorf("invalid connector: %w", err)
		}
		fields["edge_id"] = json.RawMessage(value)
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("encode agent request: %w", err)
	}
	strict := json.NewDecoder(bytes.NewReader(encoded))
	strict.DisallowUnknownFields()
	if err := strict.Decode(dst); err != nil {
		return fmt.Errorf("decode agent fields: %w", err)
	}
	return nil
}
