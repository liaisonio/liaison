//go:build !darwin && !linux

package process

import "os/exec"

// Fail closed until account validation, wrapper launch and process-tree cleanup
// have a native implementation and have been verified on this platform.
func validateProgram(string) error {
	return protocolError("native process supervision is not supported on this platform yet")
}
func prepare(*exec.Cmd) (func() error, error) {
	return nil, protocolError("unsupported process supervision")
}
func afterStart(*exec.Cmd) error { return protocolError("unsupported process supervision") }
