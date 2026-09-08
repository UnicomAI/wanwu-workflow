/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/coze-dev/coze-studio/backend/bizpkg/fileutil"
	"github.com/coze-dev/coze-studio/backend/infra/coderunner"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
)

func NewRunner(config *Config) coderunner.Runner {
	return &runner{
		pyPath:     fileutil.GetPython3Path(),
		scriptPath: fileutil.GetPythonFilePath("sandbox.py"),
		config:     config,
	}
}

type Config struct {
	AllowEnv       []string `json:"allow_env,omitempty"`
	AllowRead      []string `json:"allow_read,omitempty"`
	AllowWrite     []string `json:"allow_write,omitempty"`
	AllowNet       []string `json:"allow_net,omitempty"`
	AllowRun       []string `json:"allow_run,omitempty"`
	AllowFFI       []string `json:"allow_ffi,omitempty"`
	NodeModulesDir string   `json:"node_modules_dir,omitempty"`
	TimeoutSeconds float64  `json:"timeout_seconds,omitempty"`
	MemoryLimitMB  int64    `json:"memory_limit_mb,omitempty"`
}

type runner struct {
	pyPath, scriptPath string
	config             *Config
}

func (runner *runner) Run(ctx context.Context, request *coderunner.RunRequest) (*coderunner.RunResponse, error) {
	if request.Language == coderunner.JavaScript {
		return nil, fmt.Errorf("js not supported yet")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(req{
		Config: runner.config,
		Code:   request.Code,
		Params: request.Params,
	})
	if err != nil {
		return nil, err
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer closeFile(pr)
	defer closeFile(pw)

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer closeFile(r)
	defer closeFile(w)

	if _, err = pw.Write(b); err != nil {
		return nil, err
	}
	if err = pw.Close(); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, runner.pyPath, runner.scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return killProcessGroup(cmd)
	}
	cmd.WaitDelay = 2 * time.Second
	cmd.ExtraFiles = []*os.File{w, pr}

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	var (
		waited  bool
		waitErr error
	)
	wait := func() error {
		if waited {
			return waitErr
		}
		waited = true
		waitErr = cmd.Wait()
		return waitErr
	}
	defer func() { _ = wait() }()

	// Parent must drop its copy of the child's write end so Decode sees EOF
	// when the child (or the process group) exits.
	if err = w.Close(); err != nil {
		_ = killProcessGroup(cmd)
		return nil, err
	}
	_ = pr.Close()

	result := &resp{}
	d := json.NewDecoder(r)
	d.UseNumber()

	decodeErrCh := make(chan error, 1)
	go func() {
		decodeErrCh <- d.Decode(result)
	}()

	var decodeErr error
	select {
	case decodeErr = <-decodeErrCh:
		if decodeErr != nil || ctx.Err() != nil {
			_ = killProcessGroup(cmd)
		}
	case <-ctx.Done():
		_ = killProcessGroup(cmd)
		_ = r.Close()
		decodeErr = <-decodeErrCh
	}

	_ = wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if decodeErr != nil {
		return nil, decodeErr
	}
	if waitErr != nil {
		return nil, waitErr
	}

	logs.CtxDebugf(ctx, "resp=%v\n", result)
	if result.Status != "success" {
		return nil, fmt.Errorf("exec failed, stdout=%s, stderr=%s, sandbox_err=%s", result.Stdout, result.Stderr, result.SandboxError)
	}
	return &coderunner.RunResponse{Result: result.Result}, nil
}

func closeFile(f *os.File) {
	if f == nil {
		return
	}
	_ = f.Close()
}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	_ = cmd.Process.Kill()
	return err
}

type req struct {
	Config *Config        `json:"config"`
	Code   string         `json:"code"`
	Params map[string]any `json:"params"`
}

type resp struct {
	Result        map[string]any `json:"result"`
	Stdout        string         `json:"stdout"`
	Stderr        string         `json:"stderr"`
	Status        string         `json:"status"`
	ExecutionTime float64        `json:"execution_time"`
	SandboxError  string         `json:"sandbox_error"`
}
