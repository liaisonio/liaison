package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

const (
	bindingStoreVersion = 1
	maxNativeBindings   = 4096
	maxBindingStoreSize = 4 << 20
)

type nativeBinding struct {
	Owner          string `json:"owner"`
	Access         string `json:"access"`
	Session        string `json:"session"`
	Thread         string `json:"thread"`
	Project        string `json:"project"`
	AccessProject  string `json:"access_project"`
	InstallationID string `json:"installation_id"`
}

type bindingFile struct {
	Version  int             `json:"version"`
	Bindings []nativeBinding `json:"bindings"`
}

func bindingKey(owner, access, session string) string {
	return owner + "\x00" + access + "\x00" + session
}

func loadBindings(path string) (map[string]nativeBinding, error) {
	result := make(map[string]nativeBinding)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBindingStoreSize {
		return nil, errors.New("agent binding store is too large")
	}
	var file bindingFile
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil || file.Version != bindingStoreVersion || len(file.Bindings) > maxNativeBindings {
		return nil, errors.New("invalid agent binding store")
	}
	for _, item := range file.Bindings {
		if item.Owner == "" || len(item.Access) != 32 || len(item.Session) != 32 || item.Thread == "" || !filepath.IsAbs(item.Project) || !filepath.IsAbs(item.AccessProject) || len(item.InstallationID) != 32 {
			return nil, errors.New("invalid agent binding")
		}
		key := bindingKey(item.Owner, item.Access, item.Session)
		if _, exists := result[key]; exists {
			return nil, errors.New("duplicate agent binding")
		}
		result[key] = item
	}
	return result, nil
}

func saveBindings(path string, bindings map[string]nativeBinding) error {
	if len(bindings) > maxNativeBindings {
		return errors.New("agent binding store is full")
	}
	file := bindingFile{Version: bindingStoreVersion, Bindings: make([]nativeBinding, 0, len(bindings))}
	keys := make([]string, 0, len(bindings))
	for key := range bindings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		file.Bindings = append(file.Bindings, bindings[key])
	}
	raw, err := json.Marshal(file)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".agent-bindings-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(temporaryPath) // Best-effort cleanup of this exact temporary file.
		}
	}()
	if err = temporary.Chmod(0600); err == nil {
		_, err = temporary.Write(raw)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return err
	}
	keep = true
	return nil
}
