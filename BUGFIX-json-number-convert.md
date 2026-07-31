# Bugfix: 代码节点输出 Number/Integer 类型字段被置为 null

## 问题描述

在工作流的代码节点（CodeRunner）中，当 Python 代码返回的输出字段类型声明为 `Number` 或 `Integer` 时，节点输出会被错误地置为 `null`，并附带警告信息：

```json
{
  "total_duration": null,
  "$warning": "node output parse fail: field total_duration is not number"
}
```

## 复现步骤

1. 创建一个工作流，添加代码节点
2. 节点输出声明字段 `total_duration` 类型为 `Number`
3. 代码示例：

```python
import json

async def main(args):
    durations = args.params.get("durations", [])
    total_duration = sum(float(d) for d in durations)
    # 通过 json 序列化/反序列化确保类型是纯数字
    result = json.loads(json.dumps({"total_duration": float(total_duration)}))
    return result
```

4. 输入 `{"durations": [1.2, 6]}`
5. 运行后节点输出为 `{"total_duration": null, "$warning": "node output parse fail: field total_duration is not number"}`

## 根因分析

### 错误链路

```
Python 代码返回 {"total_duration": 7.2}            （正确）
  ↓
Python 沙箱序列化为 JSON 字符串                      （正确）
  ↓
Go 引擎用 sonic + UseNumber() 反序列化              （产生 json.Number 类型）
  ↓
convert.go 的 convertToFloat64 类型校验              （json.Number 不匹配 → default → warning）
  ↓
字段值被置为 nil，添加 $warning
```

### 源码级证据

问题出在 [convert.go](backend/domain/workflow/internal/nodes/convert.go) 的三个类型转换函数：

#### 1. `convertToFloat64`（Number 类型字段）

```go
func convertToFloat64(_ context.Context, in any, path string, options *convertOptions) (any, *ConversionWarnings, error) {
    switch in.(type) {
    case int64:
        return float64(in.(int64)), nil, nil
    case float64:
        return in.(float64), nil, nil
    case string:
        f, err := strconv.ParseFloat(in.(string), 64)
        // ...
    default:  // ← json.Number 走到了这里！
        return nil, newWarnings(path, vo.DataTypeNumber,
            fmt.Errorf("unsupported type to convert to float64: %T", in)), nil
    }
}
```

#### 2. `convertToInt64`（Integer 类型字段）— 同样的问题

#### 3. `convertToString`（String 类型字段）— 同样的问题

### 根本原因

代码运行结果的 JSON 反序列化使用了字节跳动 sonic JSON 解析器的 `UseNumber()` 模式（见 `pkg/sonic` 包引用），导致 JSON 数字被解析为 `json.Number` 类型（`type Number string`），而非 `float64`。

`json.Number` 在 Go 中是字符串类型，不匹配 `case float64:` / `case int64:`，直接走到 `default` 分支，生成 `"field xxx is not number"` 警告。

### 错误信息生成位置

```go
// convert.go
func (e *ConversionWarning) Error() string {
    return fmt.Sprintf("field %s is not %s", e.Path, e.Type)
    // → "field total_duration is not number"
}
```

### 日志证据

docker logs 中可以观察到完整链路：

```
runner.go:99  [Debug] resp=&{map[total_duration:7.2]   success 2.27s}     ← 代码输出正确
code.go:239   [Warn] convert inputs warnings: field total_duration is not number  ← 类型校验失败
event_handle.go:463 [Warn] node ... end with warning: code=720712023
              message=node output parse fail: field total_duration is not number
```

数据库 `node_execution` 表记录：

```sql
raw_output = '{"total_duration":7.2}'    -- 代码原始输出正确
output     = '{"total_duration":null}'   -- 节点输出被置 null
error_info = 'node output parse fail: field total_duration is not number'
```

## 修复方案

在 `convertToFloat64`、`convertToInt64`、`convertToString` 三个函数的 type switch 中增加 `json.Number` 的处理分支。

### 修改文件

- `backend/domain/workflow/internal/nodes/convert.go`

### 修改内容

#### 1. 新增 import

```go
import (
    "context"
    "encoding/json"   // ← 新增
    "errors"
    // ...
)
```

#### 2. `convertToString` 新增 `json.Number` 分支

```go
case bool:
    return strconv.FormatBool(in.(bool)), nil, nil
case json.Number:                                    // ← 新增
    return in.(json.Number).String(), nil, nil
case []any, map[string]any:
```

#### 3. `convertToInt64` 新增 `json.Number` 分支

```go
case string:
    i, err := strconv.ParseInt(in.(string), 10, 64)
    // ...
    return i, nil, nil
case json.Number:                                    // ← 新增
    i, err := in.(json.Number).Int64()
    if err != nil {
        if options.failFast {
            return nil, nil, vo.WrapError(errno.ErrInvalidParameter, err)
        }
        return nil, newWarnings(path, vo.DataTypeInteger, err), nil
    }
    return i, nil, nil
default:
```

#### 4. `convertToFloat64` 新增 `json.Number` 分支

```go
case string:
    f, err := strconv.ParseFloat(in.(string), 64)
    // ...
    return f, nil, nil
case json.Number:                                    // ← 新增
    f, err := in.(json.Number).Float64()
    if err != nil {
        if options.failFast {
            return nil, nil, vo.WrapError(errno.ErrInvalidParameter, err)
        }
        return nil, newWarnings(path, vo.DataTypeNumber, err), nil
    }
    return f, nil, nil
default:
```

### 修复原理

```
修改前：json.Number → type switch 不匹配 → default → "is not number" warning → null
修改后：json.Number → case json.Number: → .Float64()/.Int64()/.String() → 正确转换
```

每个新增 case 都遵循了已有 case 的错误处理模式（`failFast` 选项 + `newWarnings` 生成），保持风格一致。

## 影响范围

### 受影响的场景

- 代码节点（CodeRunner）输出字段类型为 `Number` / `Integer` / `String`，且代码返回数字类型值时
- 任何经过 sonic `UseNumber()` 反序列化后进入 `ConvertInputs` 的数据流

### 不受影响的场景

- 输出字段类型为 `Boolean` / `Object` / `Array`（这些类型不经过 `convertToInt64`/`convertToFloat64`/`convertToString`）
- 代码返回纯字符串类型的 Number/Integer 字段（已有 `case string:` 处理）

## 验证结果

### 修复前

```
raw_output: {"total_duration":7.2}
output:     {"total_duration":null}
error_info: node output parse fail: field total_duration is not number
```

### 修复后

```
output: {"total_duration":7.2}    （正确转换为 float64）
无 $warning 字段
```

## 完整 Diff

```diff
--- a/backend/domain/workflow/internal/nodes/convert.go
+++ b/backend/domain/workflow/internal/nodes/convert.go
@@ -18,6 +18,7 @@ package nodes

 import (
 	"context"
+	"encoding/json"
 	"errors"
 	"fmt"
 	"net/url"
@@ -253,6 +254,8 @@ func convertToString(_ context.Context, in any, path string, options *convertOpt
 		return strconv.FormatFloat(in.(float64), 'f', -1, 64), nil, nil
 	case bool:
 		return strconv.FormatBool(in.(bool)), nil, nil
+	case json.Number:
+		return in.(json.Number).String(), nil, nil
 	case []any, map[string]any:
 		s, err := sonic.MarshalString(in)
 		if err != nil {
@@ -285,6 +288,15 @@ func convertToInt64(_ context.Context, in any, path string, options *convertOpti
 			return nil, newWarnings(path, vo.DataTypeInteger, err), nil
 		}
 		return i, nil, nil
+	case json.Number:
+		i, err := in.(json.Number).Int64()
+		if err != nil {
+			if options.failFast {
+				return nil, nil, vo.WrapError(errno.ErrInvalidParameter, err)
+			}
+			return nil, newWarnings(path, vo.DataTypeInteger, err), nil
+		}
+		return i, nil, nil
 	default:
 		if options.failFast {
 			return nil, nil, vo.WrapError(errno.ErrInvalidParameter, fmt.Errorf("unsupported type to convert to int64: %T", in))
@@ -308,6 +320,15 @@ func convertToFloat64(_ context.Context, in any, path string, options *convertOp
 			return nil, newWarnings(path, vo.DataTypeNumber, err), nil
 		}
 		return f, nil, nil
+	case json.Number:
+		f, err := in.(json.Number).Float64()
+		if err != nil {
+			if options.failFast {
+				return nil, nil, vo.WrapError(errno.ErrInvalidParameter, err)
+			}
+			return nil, newWarnings(path, vo.DataTypeNumber, err), nil
+		}
+		return f, nil, nil
 	default:
 		if options.failFast {
 			return nil, nil, vo.WrapError(errno.ErrInvalidParameter, fmt.Errorf("unsupported type to convert to float64: %T", in))
```

## 测试用例

### 测试代码

```python
import json

async def main(args):
    durations = args.params.get("durations", [])
    total_duration = sum(float(d) for d in durations)
    result = json.loads(json.dumps({"total_duration": float(total_duration)}))
    return result
```

### 测试输入

```json
{"durations": [1.2, 6]}
```

### 期望输出

```json
{"total_duration": 7.2}
```

## 备注

- 此问题在扣子（Coze）官方平台不会出现，推测官方平台使用了标准 `encoding/json` 或在类型检查中正确处理了 `json.Number`
- 万悟部署版本使用了 sonic `UseNumber()` 模式，暴露了 `convert.go` 类型检查的遗漏
- 修复保持了与已有 case 完全一致的错误处理风格，未引入新的依赖或行为变更
