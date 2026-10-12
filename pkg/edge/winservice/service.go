// Package winservice provides the optional Windows SCM entry point. The normal
// foreground entry point remains available to One and other process supervisors.
package winservice

import (
	"context"
	"errors"
	"regexp"
)

const Flag = "--edge-windows-service"

var instanceID = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Start returns a stop function only once local initialization has succeeded.
// Running indicates process readiness, not successful tunnel authentication.
type Start func(context.Context) (func() error, error)

func Parse(args []string) (string, []string, error) {
	if len(args) != 4 || args[0] != Flag || !instanceID.MatchString(args[1]) || args[2] != "-c" || args[3] == "" {
		return "", nil, errors.New("use --edge-windows-service <32-character instance-id> -c <configuration>")
	}
	return "LiaisonEdge-" + args[1], args[2:], nil
}
