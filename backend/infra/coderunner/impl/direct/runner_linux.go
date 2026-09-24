//go:build linux

package direct

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/coze-dev/coze-studio/backend/types/consts"
)

const defaultDropUID = 65534 // nobody
const defaultDropGID = 65534 // nogroup

func applyDropPrivilegesImpl(cmd *exec.Cmd) {
	if os.Geteuid() != 0 {
		return
	}
	uid := envUint(consts.CodeRunnerUID, defaultDropUID)
	gid := envUint(consts.CodeRunnerGID, defaultDropGID)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uid,
			Gid: gid,
		},
	}
}

func envUint(key string, def uint32) uint32 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return def
	}
	return uint32(n)
}
