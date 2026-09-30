//go:build linux || darwin

package bridge

import (
	"os"
	"syscall"
)

// A replaced FIFO must not block a tunnel worker. Regular files and directories
// ignore O_NONBLOCK. os.Root independently prevents symlink traversal escapes.
func openAgentFile(root *os.Root, path string) (*os.File, error) {
	return root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
