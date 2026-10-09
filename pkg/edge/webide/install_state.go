package webide

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type installationState struct {
	Version int    `json:"version"`
	Status  string `json:"status"`
}

func (s *Service) restoreInstallStatus() error {
	f, err := os.Open(filepath.Join(s.root, "installation.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return errors.New("invalid IDE installation state size")
	}
	var state installationState
	d := json.NewDecoder(io.LimitReader(f, 4097))
	d.DisallowUnknownFields()
	if d.Decode(&state) != nil || d.Decode(new(any)) != io.EOF || state.Version != 1 {
		return errors.New("invalid IDE installation state")
	}
	switch state.Status {
	case "ok", "install_failed":
		s.installStatus = state.Status
	case "installing":
		// Recovery never repeats a download or publishes a partially extracted
		// installation. The operator explicitly retries; existing versions remain.
		s.installStatus = "install_failed"
		return s.saveInstallStatus(s.installStatus)
	default:
		return errors.New("invalid IDE installation status")
	}
	return nil
}

func (s *Service) saveInstallStatus(status string) error {
	data, err := json.Marshal(installationState{Version: 1, Status: status})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.root, ".installation-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // Only this operation's private temporary file.
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(s.root, "installation.json"))
}
