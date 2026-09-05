# Affinity 与 Script Router 兼容性

[English](../affinity-script-router-compatibility.md)

本文说明 Affinity Router 和 Script Router 从 Admin 到 runtime 的契约。它有意
不修改 Condition Router、Tag Router 或 Dynamic Config 的既有契约。

## 规则标识与配置中心键

| 规则 | 作用域 | 规则名 | ZooKeeper 键 | Nacos 键 |
| --- | --- | --- | --- | --- |
| Affinity | `application` 或 `service` | `<key>.affinity-router` | `/dubbo/config/dubbo/<ruleName>` | `DataId=<ruleName>`，`Group=dubbo` |
| Script | 仅 `application` | `<provider-application>.script-router` | `/dubbo/config/dubbo/<ruleName>` | `DataId=<ruleName>`，`Group=dubbo` |

Admin 会在发布前校验规则名、作用域和公开 YAML。它也通过 ZooKeeper 和 Nacos
watcher 回读相同的键，因此任何兼容 control plane 创建的规则都会表示为相同的
Admin resource。

## 公开 YAML

Affinity Router 固定使用 `configVersion: v3.1` 和公开字段 `affinityAware`。
内部 protobuf 字段名为 `affinity`，但它绝不会作为外部 YAML 字段输出。

```yaml
configVersion: v3.1
scope: application
key: demo-provider
runtime: true
enabled: true
affinityAware:
  key: region
  ratio: 80
```

`ratio` 的有效范围包含 `0` 和 `100`。服务作用域的 key 遵循 Dubbo service key
格式，例如 `org.apache.dubbo.Greeter:1.0.0:demo`。

Script Router 仅支持 application 作用域；它要求非空且已注册的 `javascript`
脚本。Admin 存储并发布脚本，但不执行脚本，也不保证一个脚本在 Java 和 Go
runtime 之间可移植。

```yaml
configVersion: v3.0
scope: application
key: demo-provider
enabled: true
type: javascript
script: |-
  (function route(invokers, invocation, context) {
    return invokers;
  })(invokers, invocation, context)
```

## Dubbo-Go Consumer 前置条件

历史的一站式 `imports` 包不会注册 Affinity。需要 Affinity 的 consumer 必须
import Affinity Router 包，以注册它的 factory：

```go
import _ "dubbo.apache.org/dubbo-go/v3/cluster/router/affinity"
```

较旧的 Dubbo-Go Script Router 版本也要求显式注册 Script factory。当前版本会在
包初始化阶段完成注册；如需支持旧版本，可以在 application 中显式注册：

```go
import (
    scriptrouter "dubbo.apache.org/dubbo-go/v3/cluster/router/script"
    "dubbo.apache.org/dubbo-go/v3/common/constant"
    "dubbo.apache.org/dubbo-go/v3/common/extension"
)

extension.SetRouterFactory(constant.ScriptRouterFactoryKey, scriptrouter.NewScriptRouterFactory)
```

要使删除 Script 规则能够移除先前编译的 router，Dubbo-Go runtime 必须先处理
`EventTypeDel`，再尝试解析空配置 payload。兼容的上游实现在
[`fd4dea4f8`](https://github.com/apache/dubbo-go/commit/fd4dea4f8ccef79a3190ba1d4a60af775d615135)
开始提供。若使用较旧 runtime 验证删除行为，请使用包含该改动的 release，或应用
其向后兼容的修复。

## 端到端验证

可选测试
[`e2e/router-rule-chain/router_rule_chain_e2e_test.go`](../../e2e/router-rule-chain/router_rule_chain_e2e_test.go)
使用真实 ZooKeeper 服务和 Dubbo-Go `RouterChain`，验证以下内容：

1. Admin 在 `/dubbo/config/dubbo/<ruleName>` 发布公开 YAML。
2. consumer listener 收到 Add、Update 和 Delete 事件。
3. Affinity 与 Script 规则更新会替换 RouterChain 的活动状态。
4. 删除规则会恢复未过滤的 invoker 列表。

只有在 ZooKeeper endpoint 可访问、且 Dubbo-Go revision 满足上述 Script 删除前置
条件时才运行该测试：

```powershell
$env:DUBBO_ADMIN_E2E_ZK_ADDR = 'zookeeper://127.0.0.1:2181'
go test -tags=e2e ./e2e/router-rule-chain -count=1 -v
```

Admin module 有意保持当前 Go 1.24 依赖基线。对于此跨仓库测试，应使用选择了
兼容 Dubbo-Go 源码的本地 Go workspace 或本地 replace；不要将该本地覆盖提交到
Admin module。

完整的本地启动、执行、通过条件和故障排查步骤，请参阅
[Affinity 与 Script Router 手动 E2E 验证](../../e2e/router-rule-chain/README.md)。
