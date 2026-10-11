package bridge

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

const maxInstallationIdentities = 1024

// 保存已发现的启动路径身份。软件升级可以替换软链接目标，但不能把绑定
// 自动迁移到另一个启动路径或另一种 Agent。旧 hash 保留为兼容别名。
type installationIdentity struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type installationFile struct {
	Version int                    `json:"version"`
	Items   []installationIdentity `json:"items"`
}

type installationStore struct {
	path  string
	gate  chan struct{}
	items []installationIdentity // 由 gate 串行管理，文件成功保存后才更新。
}

func loadInstallationStore(path string) (*installationStore, error) {
	s := &installationStore{path: path, gate: make(chan struct{}, 1)}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxBindingStoreSize+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBindingStoreSize {
		return nil, errors.New("agent installation store is too large")
	}
	var file installationFile
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&file); err != nil {
		return nil, errors.New("invalid agent installation store")
	}
	if d.Decode(new(any)) != io.EOF || file.Version != 1 || len(file.Items) > maxInstallationIdentities {
		return nil, errors.New("invalid agent installation store")
	}
	seen := map[string]bool{}
	for _, item := range file.Items {
		id, err := hex.DecodeString(item.ID)
		if err != nil || len(id) != 16 || item.Kind == "" || strings.ContainsRune(item.Kind, 0) || !filepath.IsAbs(item.Path) || filepath.Clean(item.Path) != item.Path || strings.ContainsRune(item.Path, 0) || seen[item.ID] {
			return nil, errors.New("invalid agent installation identity")
		}
		seen[item.ID] = true
	}
	s.items = file.Items
	return s, nil
}

type installationSet struct {
	byID      map[string]discovery.Installation
	canonical map[string]string
}

func installationKey(i discovery.Installation) string { return i.Agent + "\x00" + i.Path }

func (s *installationStore) resolve(ctx context.Context, found discovery.Result) (installationSet, error) {
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	case <-ctx.Done():
		return installationSet{}, ctx.Err()
	}
	items := append([]installationIdentity(nil), s.items...)
	known := map[string]installationIdentity{}
	for _, item := range items {
		known[item.ID] = item
	}
	for _, i := range found.Installations {
		if i.Wrapper {
			continue
		}
		id := installationID(i)
		if old, ok := known[id]; ok {
			if old.Kind != i.Agent || old.Path != i.Path {
				return installationSet{}, errors.New("conflicting agent installation identity")
			}
			continue
		}
		item := installationIdentity{ID: id, Kind: i.Agent, Path: i.Path}
		items = append(items, item)
		known[id] = item
	}
	if len(items) > maxInstallationIdentities {
		return installationSet{}, errors.New("agent installation store is full")
	}
	if len(items) != len(s.items) {
		raw, err := json.Marshal(installationFile{Version: 1, Items: items})
		if err != nil {
			return installationSet{}, err
		}
		if err := writePrivateStore(s.path, raw); err != nil {
			return installationSet{}, err
		}
		s.items = items
	}
	out := installationSet{byID: map[string]discovery.Installation{}, canonical: map[string]string{}}
	for _, i := range found.Installations {
		if i.Wrapper {
			continue
		}
		for _, item := range items {
			if item.Kind != i.Agent || item.Path != i.Path {
				continue
			}
			out.byID[item.ID] = i
			key := installationKey(i)
			if out.canonical[key] == "" {
				out.canonical[key] = item.ID
			}
		}
	}
	return out, nil
}
