// Windows SCM fixture. Never shipped in Edge packages; no network or credentials.
package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/liaisonio/liaison/pkg/edge/config"
	"github.com/liaisonio/liaison/pkg/edge/winservice"
)

func main() {
	name, args, err := winservice.Parse(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}
	os.Args = append([]string{os.Args[0]}, args...)
	if err := winservice.Run(name, func(context.Context) (func() error, error) {
		if err := config.Init(); err != nil {
			return nil, err
		}
		// Exercise the real service token's configuration and log permissions.
		if err := os.WriteFile(filepath.Join(filepath.Dir(config.Conf.Log.File), "fixture.log"), []byte("ready\n"), 0600); err != nil {
			return nil, err
		}
		return func() error { return nil }, nil
	}); err != nil {
		os.Exit(1)
	}
}
