//go:build !darwin && !linux

package webide

import (
	"context"
	"errors"
	"os/exec"
)

func canLaunch() bool                            { return false }
func prepareProcess(*exec.Cmd)                   {}
func privateDirectory(string) error              { return errors.New("unsupported IDE platform") }
func trustedProgram(string) error                { return errors.New("unsupported IDE platform") }
func stopProcess(context.Context, *record) error { return errors.New("unsupported IDE platform") }
