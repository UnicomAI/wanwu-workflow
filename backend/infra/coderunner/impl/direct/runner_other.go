//go:build !linux

package direct

import "os/exec"

func applyDropPrivilegesImpl(_ *exec.Cmd) {}
