// Windows SCM fixture. Never shipped in Edge packages; no network or credentials.
package main

import (
	"context"
	"github.com/liaisonio/liaison/pkg/edge/winservice"
	"os"
)

func main() {
	name, _, err := winservice.Parse(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}
	if err := winservice.Run(name, func(context.Context) (func() error, error) {
		return func() error { return nil }, nil
	}); err != nil {
		os.Exit(1)
	}
}
