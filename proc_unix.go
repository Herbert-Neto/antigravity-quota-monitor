//go:build !windows

package main

import "os/exec"

func configureHideWindow(_ *exec.Cmd) {}
