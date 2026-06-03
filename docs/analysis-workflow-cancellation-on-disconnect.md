# Workflow 同步调用请求断开后继续执行的分析与解决方案

## 1. 问题描述

调用 `POST /v1/workflow/:workflow_id/run_by_wanwu` 接口，以 `IsAsync = false`（同步模式）运行 workflow 时，当 HTTP 请求因客户端断开连接而取消后，workflow 的实际执行并未中断，会继续运行直到完成。

---

## 2. 根因分析

### 2.1 这是设计意图，而非 Bug

**workflow 执行引擎的设计哲学是"启动即完成"（fire-and-forget）**——一旦 workflow 开始执行，就应执行到终态（成功/失败/取消），不应因外部因素（如客户端断开）而半途中止。这一设计意图在代码中有明确体现：

```go
// backend/domain/workflow/internal/compose/workflow_run.go:306
// this goroutine should not use the cancelCtx because it needs to be alive
// to receive workflow cancel events
lastEventChan <- execute.HandleExecuteEvent(ctx, executeID, eventChan, cancelFn, timeoutFn,
    repo, sw, config)
```

注释直接说明了 `HandleExecuteEvent` goroutine **故意不使用 cancelCtx**，因为它需要在请求上下文取消后仍然存活，以便接收并处理 workflow 的完成/取消事件、更新数据库状态。

### 2.2 三层解耦机制

HTTP 请求生命周期与 workflow 执行生命周期之间有三层解耦：

#### 第一层：事件处理 goroutine 与请求上下文解耦

**文件**: `backend/domain/workflow/internal/compose/workflow_run.go:284-310`

```go
// Prepare() 方法中:
cancelCtx, cancelFn := context.WithCancel(ctx)          // cancelCtx 绑定到 HTTP ctx
// ...
go func() {
    // HandleExecuteEvent 使用原始 ctx，而非 cancelCtx
    // 即使 HTTP ctx 取消，此 goroutine 继续运行
    lastEventChan <- execute.HandleExecuteEvent(ctx, executeID, eventChan, cancelFn, timeoutFn,
        repo, sw, config)
}()
```

- `cancelCtx` 传递给 `SyncRun` 用于节点执行的超时控制
- `HandleExecuteEvent` goroutine 使用原始 `ctx`，故意绕过取消传播
- 此 goroutine 负责：处理节点事件、更新数据库执行状态、发送完成信号

#### 第二层：HandleExecuteEvent 不检查 context 取消

**文件**: `backend/domain/workflow/internal/execute/event_handle.go:802-820`

```go
// Cancellable == false 时走此分支（OpenAPI 调用默认 Cancellable=false）
} else {
    defer func() {
        // ... cleanup
        cancelFn()
    }()
    for e := range eventChan {    // 纯 for-range，无 ctx.Done() 检查
        if terminalE := handler(e); terminalE != nil {
            return terminalE
        }
    }
}
```

即使 `Cancellable == true` 的路径（第 761-801 行），也仅检查 Redis 中的取消标志，**不检查 `ctx.Done()`**：

```go
if exeCfg.Cancellable {
    cancelTicker := time.NewTicker(cancelCheckInterval)
    for {
        select {
        case <-cancelTicker.C:
            isCancelled, err := repo.GetWorkflowCancelFlag(ctx, wfExeID)  // 查 Redis
            if isCancelled {
                cancelled = true
                cancelFn()
            }
        case event = <-eventChan:
            // ...
        }
    }
}
```

#### 第三层：safego.Go 启动的回调 goroutine 不响应 context 取消

**文件**: `backend/pkg/safego/safego.go`

```go
func Go(ctx context.Context, fn func()) {
    go func() {
        defer goutil.Recovery(ctx)  // ctx 仅用于 panic 日志
        fn()                         // fn() 运行到完成，不检查 ctx.Done()
    }()
}
```

Workflow 节点执行过程中的回调（LLM 流式输出、工具调用响应、节点启动/完成事件等）均通过 `safego.Go` 启动，这些 goroutine 不受 context 取消影响：

- `callback.go:487` — `safego.Go(ctx, func() { ... })` workflow OnEndWithStreamOutput
- `callback.go:835` — `safego.Go(ctx, func() { ... })` node OnStartWithStreamInput
- `callback.go:1190` — `safego.Go(ctx, func() { ... })` node OnEndWithStreamOutput (incremental)
- `callback.go:1205` — `safego.Go(ctx, func() { ... })` node OnEndWithStreamOutput (non-incremental)
- `callback.go:1312` — `safego.Go(ctx, func() { ... })` tool OnEndWithStreamOutput
- `workflow.go:162` — `safego.Go(ctx, func() { ... })` AsyncRun Invoke
- `workflow.go:168` — `safego.Go(ctx, func() { ... })` AsyncRun Stream
- `emitter.go:345` — `safego.Go(ctx, func() { ... })` emitter
- `batch.go:388` — `safego.Go(ctx, func() { ... })` batch processing

### 2.3 完整执行时序

```
T0: HTTP 请求到达
    │
    ├─ Prepare(ctx): cancelCtx = context.WithCancel(ctx)
    ├─ 启动 HandleExecuteEvent goroutine (使用 ctx, 非 cancelCtx)
    └─ SyncRun(cancelCtx, ...) → Runner.Invoke(cancelCtx, ...)
        └─ eino DAG 引擎执行节点
           └─ 回调 goroutines 通过 safego.Go 启动 (不响应 ctx 取消)

T1: HTTP 客户端断开连接
    │
    ├─ Hertz 取消 HTTP ctx → cancelCtx 也被取消
    │
    ├─ Runner.Invoke(cancelCtx) 可能返回 context.Canceled
    │   但 HandleExecuteEvent goroutine 仍在运行 ← 设计意图
    │   safego.Go 回调 goroutines 仍在运行 ← 设计意图
    │
    ├─ 节点中的飞行中操作（LLM 调用、HTTP 请求等）不会中断
    ├─ HandleExecuteEvent 继续处理 eventChan 中的事件
    └─ 最终 workflow 完整执行完毕，数据库状态更新为终态

T2: Handler 层收到 context.Canceled 错误并返回给已断开的客户端
    workflow 已经/正在完成执行，结果已持久化到数据库
```

### 2.4 小结

| 层级 | 组件 | 是否响应 ctx 取消 | 原因 |
|------|------|-------------------|------|
| Handler 层 | `OpenAPIRunWorkFlowByWanwu` | ✅ 返回错误 | HTTP 框架行为 |
| 领域层 | `SyncExecute` → `SyncRun` | ⚠️ 可能返回错误 | cancelCtx 取消传播到 Runner |
| 事件处理 | `HandleExecuteEvent` goroutine | ❌ 不响应 | **故意**：需存活以处理完成事件 |
| 回调层 | `safego.Go` 启动的 goroutines | ❌ 不响应 | **故意**：需存活以完成节点回调 |
| 取消机制 | `Cancellable=true` + Redis 标志 | ⚠️ 仅查 Redis | 设计为显式取消，非 context 取消 |

---

## 3. 解决方案

### 方案 A：利用现有 Cancel API（推荐，最小改动）

现有代码已实现了完整的取消机制：`Cancel` 方法通过 Redis 标志 + 数据库状态更新来取消运行中的 workflow。

**改动点**：在 `OpenAPIRunByWanwu` 中设置 `Cancellable: true`，并在 HTTP 请求断开时调用 `Cancel`。

```go
// backend/application/workflow/workflow_openapi_wanwu.go

func (w *ApplicationService) OpenAPIRunByWanwu(ctx context.Context, workflowID string, req *workflow.OpenAPIRunFlowRequest) (
    _ *workflow.OpenAPIRunFlowResponse, _ vo.TerminatePlan, err error,
) {
    // ... 现有逻辑 ...

    exeCfg := workflowModel.ExecuteConfig{
        // ... 现有字段 ...
        Cancellable: true,  // ← 新增：启用取消支持
    }

    if req.GetIsAsync() {
        // ... 异步逻辑不变 ...
    }

    // 同步执行
    exeCfg.SyncPattern = workflowModel.SyncPatternSync
    exeCfg.TaskType = workflowModel.TaskTypeForeground
    wfExe, tPlan, err := GetWorkflowDomainSVC().SyncExecute(ctx, exeCfg, parameters)
    if err != nil {
        // 请求断开导致的 context 取消，尝试取消 workflow
        if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
            // 尝试取消：查询刚创建的执行记录并设置取消标志
            // 注意：SyncExecute 在 cancelCtx 取消后可能尚未返回 executeID
            // 需要通过其他途径获取 executeID
        }
        return nil, "", err
    }

    // ... 现有返回逻辑 ...
}
```

**问题**：`SyncExecute` 返回后才能拿到 `executeID`，但 context 取消时 `SyncExecute` 尚未返回，无法拿到 `executeID` 来调用 `Cancel`。

**解决思路**：在 `Prepare` 阶段就已经生成了 `executeID` 并写入了数据库，可以在 `SyncExecute` 之前或期间获取它。

#### 具体实现

**步骤 1**：重构 `SyncExecute`，使其在 `Prepare` 之后通过回调/channel 暴露 `executeID`：

```go
// 新增：SyncExecuteWithCancelFunc 返回 executeID 和 cancel 函数
func (i *impl) SyncExecuteWithCancelFunc(ctx context.Context, config workflowModel.ExecuteConfig, input map[string]any) (
    executeID int64, result *entity.WorkflowExecution, tPlan vo.TerminatePlan, err error,
) {
    // ... 与 SyncExecute 相同的前置逻辑 ...

    cancelCtx, exeID, opts, lastEventChan, err := compose.NewWorkflowRunner(
        wfEntity.GetBasic(), workflowSC, config, compose.WithInput(inStr),
    ).Prepare(ctx)
    if err != nil {
        return 0, nil, "", err
    }

    // 暴露 executeID，调用方可在此后注册取消回调
    executeID = exeID

    // ... 执行和返回逻辑 ...
}
```

**步骤 2**：在 Handler 层注册请求断开时的取消回调：

```go
// backend/api/handler/coze/workflow_service_openapi_wanwu.go

func OpenAPIRunWorkFlowByWanwu(ctx context.Context, c *app.RequestContext) {
    // ... 参数解析 ...

    exeCfg.Cancellable = true  // 启用取消

    // 使用返回 executeID 的版本
    executeID, resp, tPlan, err := appworkflow.SVC.SyncExecuteWithCancelFunc(ctx, exeCfg, parameters)

    if executeID > 0 {
        // 注册请求断开时的取消回调
        go func() {
            <-ctx.Done()  // HTTP 请求断开
            if ctx.Err() != nil {
                cancelCtx := context.Background()  // 取消操作用独立 context
                if cancelErr := appworkflow.SVC.Cancel(cancelCtx, executeID, exeCfg.ID, 0); cancelErr != nil {
                    logs.CtxErrorf(ctx, "failed to cancel workflow %d on disconnect: %v", executeID, cancelErr)
                }
            }
        }()
    }

    // ... 现有返回逻辑 ...
}
```

**步骤 3**：在 `HandleExecuteEvent` 的 Cancellable 路径中增加 `ctx.Done()` 检查（增强响应速度）：

```go
// backend/domain/workflow/internal/execute/event_handle.go

if exeCfg.Cancellable {
    cancelTicker := time.NewTicker(cancelCheckInterval)
    defer func() { /* ... */ }()

    for {
        select {
        case <-cancelTicker.C:
            // ... 现有 Redis 取消检查 ...
        case <-ctx.Done():
            // 新增：HTTP context 取消也触发取消
            logs.CtxInfof(ctx, "workflow %d context cancelled (client disconnect)", wfExeID)
            cancelled = true
            cancelFn()
        case event = <-eventChan:
            if terminalE := handler(event); terminalE != nil {
                return terminalE
            }
        }
    }
}
```

**优点**：
- 利用已有的 `Cancel` 机制，状态管理一致
- 请求断开时通过 Redis 标志通知所有相关 goroutine
- 改动最小，不影响 async 执行路径

**缺点**：
- 需要修改 `SyncExecute` 的签名（或新增方法）
- 取消不是即时的，依赖 Redis 轮询间隔（`cancelCheckInterval`）

---

### 方案 B：HandleExecuteEvent 直接监听 ctx.Done()（中等改动）

在 `HandleExecuteEvent` 的两个分支中都加入 `ctx.Done()` 检查，不依赖 Redis 轮询。

```go
// backend/domain/workflow/internal/execute/event_handle.go

func HandleExecuteEvent(ctx context.Context, ...) (event *Event) {
    // ...

    if exeCfg.Cancellable {
        cancelTicker := time.NewTicker(cancelCheckInterval)
        defer func() { /* ... */ }()

        for {
            select {
            case <-cancelTicker.C:
                // ... 现有 Redis 检查 ...
            case <-ctx.Done():
                cancelled = true
                cancelFn()
            case event = <-eventChan:
                if terminalE := handler(event); terminalE != nil {
                    return terminalE
                }
            }
        }
    } else {
        defer func() { /* ... */ }()

        for {
            select {
            case <-ctx.Done():
                // 请求断开，设置取消状态
                cancelFn()
                // 返回一个 Cancel 事件
                c := GetExeCtx(ctx)
                return &Event{
                    Type:     WorkflowCancel,
                    Context:  c,
                    Duration: time.Since(time.UnixMilli(c.StartTime)),
                }
            case e, ok := <-eventChan:
                if !ok {
                    return nil
                }
                if terminalE := handler(e); terminalE != nil {
                    return terminalE
                }
            }
        }
    }
}
```

**但需要注意**：`HandleExecuteEvent` 的 goroutine 注释了 **"should not use the cancelCtx"**，因为它需要存活来接收事件。如果直接用 `ctx.Done()` 终止此 goroutine，会导致：

1. `lastEventChan` 永远不会收到最终事件
2. 数据库中的 workflow 执行状态可能停留在 `Running`，不会更新为 `Cancelled`
3. 飞行中的节点回调 goroutine（safego.Go）仍然运行

**因此，方案 B 单独使用不完整**，必须配合方案 A 的 `Cancel` 调用来确保数据库状态正确更新。

---

### 方案 C：全面改造（大改动，不推荐除非有强需求）

对整个 workflow 引擎进行 context-aware 改造：

1. **改造 `safego.Go`** — 增加 context 检查版本：
   ```go
   func GoWithContext(ctx context.Context, fn func(context.Context)) {
       go func() {
           defer goutil.Recovery(ctx)
           fn(ctx)
       }()
   }
   ```

2. **所有节点执行回调传递 context** — 在回调中检查 `ctx.Done()`

3. **LLM/HTTP 节点支持 context 取消** — eino 引擎的 `Runner.Invoke(ctx, ...)` 传播取消到 LLM 调用

4. **统一的取消信号传播** — 从 HTTP context → HandleExecuteEvent → eventChan → 各节点 goroutine

**优点**：取消是即时的、彻底的
**缺点**：
- 改动面极大，涉及数十个文件
- 破坏现有设计意图（fire-and-forget）
- 可能导致执行状态不一致（部分节点已完成、部分取消，数据库状态难以定义）
- 对 async 执行路径有副作用

---

## 4. 推荐方案

**推荐方案 A**，理由：

1. **与现有设计一致**：系统已有通过 `Cancel` API + Redis 标志取消 workflow 的完整机制，只需在 HTTP 断开时触发它
2. **改动最小**：主要改动在 Handler 层和应用服务层，不涉及引擎核心
3. **状态一致**：`Cancel` 方法会正确更新数据库状态为 `Cancelled`，并取消所有运行中的节点
4. **渐进增强**：可以先加 `Cancellable: true` + ctx.Done() 触发 Cancel，后续再优化响应速度

### 实施步骤

| 步骤 | 改动 | 文件 |
|------|------|------|
| 1 | `OpenAPIRunByWanwu` 设置 `Cancellable: true` | `backend/application/workflow/workflow_openapi_wanwu.go` |
| 2 | 新增 `SyncExecuteWithCancelFunc` 或在 `SyncExecute` 中提前返回 `executeID` | `backend/domain/workflow/service/executable_impl.go` |
| 3 | Handler 注册请求断开回调，调用 `Cancel` | `backend/api/handler/coze/workflow_service_openapi_wanwu.go` |
| 4 | `HandleExecuteEvent` Cancellable 分支增加 `ctx.Done()` 检查（加速响应） | `backend/domain/workflow/internal/execute/event_handle.go` |
| 5 | 确认 `Cancel` 方法对 foreground 任务正确处理 | `backend/domain/workflow/service/executable_impl.go:996` |

### 关于 async 执行路径

`IsAsync = true` 时，`AsyncRun` 通过 `safego.Go` 启动后立即返回 `executeID`，请求天然不会等待执行完成。客户端断开不影响执行。此路径无需改动。
