# Affinity 与 Script Router 手动 E2E 验证

本指南用于运行 Affinity Router 与 Script Router 的可选端到端测试。它面向
需要在提交或评审变更前，在本地确认真实配置传播链路的开发者。

```text
Dubbo Admin ZK governor
  -> /dubbo/config/dubbo/<ruleName>
  -> Dubbo-Go 动态配置监听器
  -> RouterChain
```

测试不会 mock 配置监听器或 `RouterChain`。它会通过真实 ZooKeeper 写入规则，
再验证已经创建好的 Dubbo-Go consumer RouterChain 会在规则创建、更新、删除后
改变其选中的 invoker。

## 本测试验证的内容

| 路由 | 创建 | 更新 | 删除 |
| --- | --- | --- | --- |
| Affinity | 发布 `<application>.affinity-router`，YAML 使用公开字段 `affinityAware`；`region=beijing` 的 consumer 只选择北京 provider。 | 将 `ratio` 从 `50` 改为 `60`，RouterChain 回退为两个 provider。 | 监听器收到 `Del`，RouterChain 保持恢复后的两个 provider。 |
| Script | 发布 `<application>.script-router`，YAML 包含 `type: javascript`；脚本选择其中一个 provider。 | 用新脚本替换旧脚本，使其选择另一个 provider。 | 监听器收到 `Del`，RouterChain 恢复为两个 provider。 |

每次执行都会生成唯一 application 名称。测试使用两个内存 provider invoker：
北京 provider 端口为 `20880`，上海 provider 端口为 `20881`。

## 范围与前置条件

这是一个运行时配置传播 E2E。它验证 Admin governor、ZooKeeper 数据路径、
Dubbo-Go 监听器和 RouterChain 行为；不启动 Admin HTTP 服务或 Vue 页面，
不发起真实网络 RPC，也不执行真实 Nacos E2E。

需要满足以下条件：

1. 使用包含 E2E 测试的分支（集成验证阶段例如
   `feat/affinity-script-router-integration`）。
2. Go `1.24.0`，与 [`go.mod`](../../go.mod) 一致。
3. Docker，或其他可访问的 ZooKeeper 服务。
4. 使用包含 Script Router 删除修复的 Dubbo-Go runtime。上游修复从
   [`fd4dea4f8`](https://github.com/apache/dubbo-go/commit/fd4dea4f8ccef79a3190ba1d4a60af775d615135)
   开始。对于当前 Admin 锁定的依赖版本，可以改用本地含回补修复的 checkout：
   `D:\environment\github\dubbo-go\dubbo-go-script-delete-fix`。

第 4 项很重要：旧版 Script Router 会在删除时尝试解析空 payload，报出 `EOF`，
并保留旧脚本路由。

## 1. 启动 ZooKeeper

在 PowerShell 终端 A 中启动一个隔离的本地 ZooKeeper：

```powershell
docker run --rm --name dubbo-admin-e2e-zk -d -p 2181:2181 zookeeper:3.9
Test-NetConnection 127.0.0.1 -Port 2181
```

只有在 `TcpTestSucceeded` 为 `True` 后才继续。若已有可用 ZooKeeper，
可跳过 Docker，并在后续命令中使用其地址。

## 2. 不修改 `go.mod`，选择兼容的 Dubbo-Go checkout

不要在本仓库的 `go.mod` 中加入本地 `replace`。应在 Admin 仓库外创建 Go
workspace。下面示例假设 Admin 仓库和包含回补修复的 Dubbo-Go checkout 同处于
`D:\environment\github\dubbo-go` 目录下。

```powershell
$routerE2EWorkspace = 'D:\environment\github\dubbo-go\.router-e2e-workspace'
New-Item -ItemType Directory -Force $routerE2EWorkspace | Out-Null

Push-Location $routerE2EWorkspace
if (-not (Test-Path 'go.work')) {
    go work init ..\dubbo-admin ..\dubbo-go-script-delete-fix
}
$env:GOWORK = Join-Path (Get-Location) 'go.work'
Pop-Location
```

如果使用的 Dubbo-Go release 已包含上述上游修复，请将 workspace 指向该模块。
测试本身已经 import Affinity，并注册 Script Router factory，因此测试进程不需要
额外 import。

## 3. 执行 E2E 测试

在 PowerShell 终端 B 中运行：

```powershell
Set-Location D:\environment\github\dubbo-go\dubbo-admin
$env:DUBBO_ADMIN_E2E_ZK_ADDR = 'zookeeper://127.0.0.1:2181'
go test -tags=e2e ./e2e/router-rule-chain -count=1 -v
```

必须使用 `-count=1`，避免 Go test cache 遮蔽失败或跳过的执行。成功时输出末尾应为：

```text
--- PASS: TestAdminRulesReachDubboGoRouterChain
PASS
```

不能只凭 exit code 为零就判定通过。未设置 `DUBBO_ADMIN_E2E_ZK_ADDR` 时，测试
会有意输出 `SKIP` 并以零退出，因为此时无法证明外部配置链路真正生效。

## 4. 判定通过的具体行为

测试会为每个配置事件和路由结果最多等待十秒。测试通过时，已经验证了下列行为：

### Affinity Router

1. Admin 将 `<application>.affinity-router` 写入
   `/dubbo/config/dubbo/<ruleName>`。
2. ZooKeeper 中的 YAML 包含 `affinityAware:`，且不会泄露内部 protobuf 字段名
   `affinity:`。
3. Dubbo-Go 监听器收到 Add 事件，RouterChain 从两个 invoker 变为只包含北京
   invoker（`20880`）。
4. 把 `ratio: 50` 更新为 `ratio: 60` 后，收到 Update 事件（旧但兼容的
   ZooKeeper listener 可能报告为 Add），并恢复两个 invoker。
5. 删除规则后收到 Delete 事件，RouterChain 保持未过滤的两个 invoker。

### Script Router

1. Admin 将 `<application>.script-router` 写入
   `/dubbo/config/dubbo/<ruleName>`，YAML 包含 `type: javascript`。
2. 监听器收到 Add 事件，初始 JavaScript 脚本选择一个 provider。
3. 更新 JavaScript 后选择另一个 provider，证明编译后的 router 被热替换，
   而非只是重新读取配置。
4. 删除规则后收到 Delete 事件，RouterChain 恢复两个 invoker。这是对
   Script Router 空 payload 删除缺陷的回归验证。

## 故障排查

| 现象 | 可能原因 | 处理方法 |
| --- | --- | --- |
| 输出 `SKIP` | 未设置 `DUBBO_ADMIN_E2E_ZK_ADDR`。 | 设置为可访问的 `zookeeper://host:port`，并使用 `-count=1` 重新执行。 |
| 无法连接 ZooKeeper | 容器未就绪、`2181` 被占用，或地址错误。 | 执行 `Test-NetConnection`，查看 `docker logs dubbo-admin-e2e-zk`，或改用其他可访问服务。 |
| Script 删除超时或报 `EOF` | 使用的 Dubbo-Go 不含删除修复。 | 使用包含 `fd4dea4f8` 的 checkout/release 或兼容回补，再重新创建 Go workspace。 |
| 十秒内没有路由更新 | 监听器未连接到同一个 ZooKeeper，或真实 consumer 未注册 router factory。 | 核对地址；然后参阅[兼容说明中的 consumer 前置条件](../../docs/zh-cn/affinity-script-router-compatibility.md#dubbo-go-consumer-前置条件)。 |
| 创建规则后仍然得到两个 invoker | 当前 fixture 的规则语义或 provider metadata 与断言不符。 | 检查测试中的 consumer/provider URL，以及 [`router_rule_chain_e2e_test.go`](router_rule_chain_e2e_test.go) 中的 `ratio`/脚本断言。 |

## 5. 清理环境

测试结束后，仅移除环境变量覆盖并停止临时 ZooKeeper 容器：

```powershell
Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
Remove-Item Env:DUBBO_ADMIN_E2E_ZK_ADDR -ErrorAction SilentlyContinue
docker stop dubbo-admin-e2e-zk
```

测试会删除它创建的 ZooKeeper 规则。仓库外的 Go workspace 可以保留，供后续执行
继续使用。

Admin 与 Dubbo-Go 间完整的 YAML 和配置中心契约，请参阅
[Affinity 与 Script Router 兼容说明](../../docs/zh-cn/affinity-script-router-compatibility.md)。
