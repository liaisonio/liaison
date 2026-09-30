//go:build !linux && !darwin

package bridge

import "os"

func openAgentFile(root *os.Root, path string) (*os.File, error) { return nil, os.ErrPermission }
