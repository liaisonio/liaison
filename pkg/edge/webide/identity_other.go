//go:build !darwin

package webide

import "github.com/shirou/gopsutil/process"

func processCreated(pid int32) (int64, error) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return 0, err
	}
	return p.CreateTime()
}
func processArgs(pid int32) ([]string, error) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return nil, err
	}
	return p.CmdlineSlice()
}
