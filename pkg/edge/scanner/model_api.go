package scanner

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/liaisonio/liaison/pkg/proto"
)

const maxModelResponse = 256 << 10

func modelPort(port int) bool {
	return port == 11434 || port == 8000 || port == 1234 || port == 30000
}

// discoverModelAPI only reads metadata on the selected IP and port. No proxy,
// redirects, credentials, generation, or model-loading requests are permitted.
func discoverModelAPI(ctx context.Context, ip string, port int) *proto.ModelAPIDiscovery {
	if net.ParseIP(ip) == nil || port < 1 || port > 65535 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, MaxResponseHeaderBytes: 16 << 10,
		DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, ResponseHeaderTimeout: time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	base := "http://" + net.JoinHostPort(ip, strconv.Itoa(port))
	read := func(path string, dst any) int {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			return 0
		}
		req.Header.Set("Accept", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return 0
		}
		defer res.Body.Close() // Read-only response; no reusable connection to retain.
		if res.StatusCode != http.StatusOK {
			return res.StatusCode
		}
		kind, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
		if err != nil || kind != "application/json" {
			return 0
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, maxModelResponse+1))
		if err != nil || len(data) > maxModelResponse || json.Unmarshal(data, dst) != nil {
			return 0
		}
		return http.StatusOK
	}
	result := &proto.ModelAPIDiscovery{State: "verified"}
	seen := map[string]bool{}
	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		if len(result.Models) < 100 {
			result.Models = append(result.Models, name)
		} else {
			result.ModelsTruncated = true
		}
	}
	var tags struct {
		Models *[]struct {
			Name   string `json:"name"`
			Model  string `json:"model"`
			Digest string `json:"digest"`
		} `json:"models"`
	}
	tagStatus := read("/api/tags", &tags)
	if tagStatus == http.StatusOK && tags.Models != nil {
		valid := true
		for _, model := range *tags.Models {
			if !safeModelID(model.Name) || (model.Model == "" && model.Digest == "") {
				valid = false
				break
			}
		}
		if len(*tags.Models) == 0 {
			var version struct {
				Version string `json:"version"`
			}
			valid = read("/api/version", &version) == http.StatusOK && safeModelID(version.Version)
		}
		if valid {
			result.Service = "ollama"
			result.Protocols = append(result.Protocols, "ollama")
			for _, m := range *tags.Models {
				add(m.Name)
			}
		}
	}
	var list struct {
		Object string `json:"object"`
		Data   *[]struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		} `json:"data"`
	}
	listStatus := read("/v1/models", &list)
	if listStatus == http.StatusOK && list.Object == "list" && list.Data != nil {
		valid := true
		for _, model := range *list.Data {
			if !safeModelID(model.ID) || model.Object != "model" {
				valid = false
				break
			}
		}
		if valid {
			result.Protocols = append(result.Protocols, "openai-compatible")
			for _, m := range *list.Data {
				add(m.ID)
			}
		}
	}
	if len(result.Protocols) > 0 {
		return result
	}
	// A familiar port plus an auth challenge is a candidate, NOT protocol proof.
	if modelAuthCandidate(port, tagStatus, listStatus) {
		return &proto.ModelAPIDiscovery{State: "auth_required"}
	}
	return nil
}

func modelAuthCandidate(port, tagStatus, listStatus int) bool {
	return modelPort(port) && (tagStatus == 401 || tagStatus == 403 || listStatus == 401 || listStatus == 403)
}

func safeModelID(id string) bool {
	return id != "" && len(id) <= 256 && strings.TrimSpace(id) == id && !strings.ContainsFunc(id, unicode.IsControl)
}
