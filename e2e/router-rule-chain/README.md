# Router rule chain E2E

This opt-in integration test exercises the complete ZooKeeper path:

```text
Dubbo Admin governor
  -> /dubbo/config/dubbo/<ruleName>
  -> Dubbo-Go configuration listener
  -> RouterChain
```

It needs a reachable ZooKeeper server and a Dubbo-Go runtime that contains the
Script Router delete fix described in
[`docs/affinity-script-router-compatibility.md`](../../docs/affinity-script-router-compatibility.md).

```powershell
$env:DUBBO_ADMIN_E2E_ZK_ADDR = 'zookeeper://127.0.0.1:2181'
go test -tags=e2e ./e2e/router-rule-chain -count=1 -v
```

Do not add a local `replace` directive to `go.mod` for this test. Use a local
Go workspace or an appropriate released Dubbo-Go dependency instead.
