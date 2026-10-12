//go:build !windows

package winservice

import "errors"

func Run(_ string, _ Start) error {
	return errors.New("Windows service mode is unavailable on this platform")
}
