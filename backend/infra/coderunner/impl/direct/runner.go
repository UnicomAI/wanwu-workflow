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

package direct

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/coze-dev/coze-studio/backend/bizpkg/fileutil"
	"github.com/coze-dev/coze-studio/backend/infra/coderunner"
	"github.com/coze-dev/coze-studio/backend/pkg/sonic"
	"github.com/coze-dev/coze-studio/backend/types/consts"
)

var pythonCode = `
import asyncio
import json
import sys

class Args:
    def __init__(self, params):
        self.params = params

class Output(dict):
    pass

%s

params = json.loads(sys.stdin.read())
try:
    result = asyncio.run(main(Args(params)))
    print(json.dumps(result))
except Exception as  e:
    print(f"{type(e).__name__}: {str(e)}", file=sys.stderr)
    sys.exit(1)

`

func NewRunner() coderunner.Runner {
	return &runner{}
}

type runner struct{}

func (r *runner) Run(ctx context.Context, request *coderunner.RunRequest) (*coderunner.RunResponse, error) {
	var (
		params = request.Params
		c      = request.Code
	)
	if request.Language == coderunner.Python {
		ret, err := r.pythonCmdRun(ctx, c, params)
		if err != nil {
			return nil, err
		}
		return &coderunner.RunResponse{
			Result: ret,
		}, nil
	}
	return nil, fmt.Errorf("unsupported language: %s", request.Language)
}

func (r *runner) pythonCmdRun(_ context.Context, code string, params map[string]any) (map[string]any, error) {
	bs, err := sonic.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params to json, err: %w", err)
	}
	cmd := exec.Command(fileutil.GetPython3Path(), "-c", fmt.Sprintf(pythonCode, code)) // ignore_security_alert RCE
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe, err: %w", err)
	}
	// 安全加固（非 root 执行）：
	// 1) 将代码执行子进程降权为非 root（默认 nobody），防止以容器 root 身份运行任意代码；
	// 2) 按 CODE_RUNNER_ALLOW_ENV 白名单裁剪子进程环境变量，防止继承 JWT/DB 等敏感凭据。
	// 仅在当前进程为 root 时才执行降权，避免非 root 容器内重复降权报错。
	applyDropPrivileges(cmd)
	cmd.Env = buildChildEnv()
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start python process, err: %w", err)
	}
	if _, err = io.Copy(stdin, bytes.NewReader(bs)); err != nil {
		return nil, fmt.Errorf("failed to write to stdin, err: %w", err)
	}
	if err = stdin.Close(); err != nil {
		return nil, fmt.Errorf("failed to close stdin, err: %w", err)
	}
	err = cmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to run python script err: %s, std err: %s", err.Error(), stderr.String())
	}

	ret := make(map[string]any)
	err = sonic.Unmarshal(stdout.Bytes(), &ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// buildChildEnv 构造代码执行子进程的环境变量：
//   - 始终附带最小必需的基础变量（PATH/HOME/TMPDIR/LANG），保证 python 解释器与用户代码可正常运行；
//   - 业务变量仅额外允许 CODE_RUNNER_ALLOW_ENV（逗号分隔的变量名）里显式白名单的项；
//     JWT/DB 等敏感凭据不在白名单内时不会传入子进程。
func buildChildEnv() []string {
	envs := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/tmp",
		"TMPDIR=/tmp",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
	}
	allow := os.Getenv(consts.CodeRunnerAllowEnv)
	if allow == "" {
		return envs
	}
	allowSet := make(map[string]struct{})
	for _, key := range strings.Split(allow, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			allowSet[key] = struct{}{}
		}
	}
	for _, kv := range os.Environ() {
		key := kv
		if idx := strings.IndexByte(kv, '='); idx > 0 {
			key = kv[:idx]
		}
		if _, ok := allowSet[key]; ok {
			envs = append(envs, kv)
		}
	}
	return envs
}

// applyDropPrivileges 让代码执行子进程以非 root 身份运行（platform-specific，见 runner_linux.go / runner_other.go）。
func applyDropPrivileges(cmd *exec.Cmd) {
	applyDropPrivilegesImpl(cmd)
}
