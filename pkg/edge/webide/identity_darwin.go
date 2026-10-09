//go:build darwin

package webide

import (
	"bytes"
	"encoding/binary"
	"errors"

	"golang.org/x/sys/unix"
)

func processCreated(pid int32) (int64, error) {
	p, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil {
		return 0, err
	}
	return p.Proc.P_starttime.Sec * 1000, nil
}

func processArgs(pid int32) ([]string, error) {
	data, err := unix.SysctlRaw("kern.procargs2", int(pid))
	if err != nil {
		return nil, err
	}
	return parseProcessArgs(data)
}

// KERN_PROCARGS2: argc, executable path, NUL padding, then argc NUL-delimited
// arguments. Do not split on whitespace or read the following environment.
func parseProcessArgs(data []byte) ([]string, error) {
	invalid := errors.New("invalid process arguments")
	if len(data) < 4 {
		return nil, invalid
	}
	argc := int(binary.LittleEndian.Uint32(data[:4]))
	if argc <= 0 || argc > 65536 {
		return nil, invalid
	}
	data = data[4:]
	end := bytes.IndexByte(data, 0)
	if end < 0 {
		return nil, invalid
	}
	data = bytes.TrimLeft(data[end+1:], "\x00")
	args := make([]string, 0, argc)
	for i := 0; i < argc; i++ {
		end = bytes.IndexByte(data, 0)
		if end < 0 {
			return nil, invalid
		}
		args = append(args, string(data[:end]))
		data = data[end+1:]
	}
	return args, nil
}
