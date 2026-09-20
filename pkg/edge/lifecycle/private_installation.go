package lifecycle

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// This gate narrows service discovery only for an installer-created private
// user instance. It never authorizes arbitrary manifest-supplied paths: all
// targets have already been derived from the fixed layout and checked.
func validatePrivateInstallation(id Identity, plan Plan) error {
	if plan.Legacy || id.UID <= 0 || !instancePattern.MatchString(plan.InstanceID) {
		return ErrUnsupported
	}
	for _, path := range []string{installationBase(plan), filepath.Dir(plan.Executable)} {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode().Perm() != 0700 || !ownedDirectory(info, id.UID) {
			return ErrUnsupported
		}
	}
	path := manifestPath(plan)
	if err := validateFile(path, id.UID); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > 4096 || info.Mode().Perm() != 0600 {
		return ErrUnsupported
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return validatePrivateManifest(data, id, plan)
}

func validatePrivateManifest(data []byte, id Identity, plan Plan) error {
	var manifest InstallManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil {
		return ErrUnsupported
	}
	var extra json.RawMessage
	if decoder.Decode(&extra) != io.EOF {
		return ErrUnsupported
	}
	if manifest.Version != 1 || manifest.InstanceID != plan.InstanceID || manifest.Platform != id.OS || manifest.UID != id.UID || manifest.Service != plan.Service {
		return ErrUnsupported
	}
	return nil
}
