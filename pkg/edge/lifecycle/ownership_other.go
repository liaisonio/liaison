//go:build !linux && !darwin

package lifecycle

import "os"

func ownedFile(os.FileInfo, int, bool) bool { return false }
func ownedDirectory(os.FileInfo, int) bool  { return false }
