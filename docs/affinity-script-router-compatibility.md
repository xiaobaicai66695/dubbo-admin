# Affinity and Script Router compatibility

[简体中文](zh-cn/affinity-script-router-compatibility.md)

This document describes the Admin-to-runtime contract for Affinity Router and
Script Router. It intentionally does not change the Condition Router, Tag
Router, or Dynamic Config contracts.

## Rule identity and configuration-center keys

| Rule | Scope | Rule name | ZooKeeper key | Nacos key |
| --- | --- | --- | --- | --- |
| Affinity | `application` or `service` | `<key>.affinity-router` | `/dubbo/config/dubbo/<ruleName>` | `DataId=<ruleName>`, `Group=dubbo` |
| Script | `application` only | `<provider-application>.script-router` | `/dubbo/config/dubbo/<ruleName>` | `DataId=<ruleName>`, `Group=dubbo` |

Admin validates the name, scope, and public YAML before publishing. It reads
the same keys back through ZooKeeper and Nacos watchers, so a rule created by
another compatible control plane is represented by the same Admin resource.

## Public YAML

Affinity Router always uses `configVersion: v3.1` and the public
`affinityAware` field. The internal protobuf field is named `affinity`, but it
is never emitted as the external YAML field.

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

`ratio` is inclusive from `0` to `100`. A service-scope key follows the Dubbo
service key format, for example `org.apache.dubbo.Greeter:1.0.0:demo`.

Script Router is limited to an application rule with a non-empty registered
`javascript` script. Admin stores and publishes the script but does not
execute it and does not promise that a script is portable between Java and Go
runtimes.

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

## Dubbo-Go consumer prerequisites

Affinity is not registered by the historical one-stop `imports` package.
Consumers that need it must import the Affinity Router package so its factory
is registered:

```go
import _ "dubbo.apache.org/dubbo-go/v3/cluster/router/affinity"
```

Older Dubbo-Go Script Router builds also require the Script factory to be
registered explicitly. Current builds register it during package
initialization; an application that supports an older build can register it
explicitly:

```go
import (
    scriptrouter "dubbo.apache.org/dubbo-go/v3/cluster/router/script"
    "dubbo.apache.org/dubbo-go/v3/common/constant"
    "dubbo.apache.org/dubbo-go/v3/common/extension"
)

extension.SetRouterFactory(constant.ScriptRouterFactoryKey, scriptrouter.NewScriptRouterFactory)
```

For Script delete to remove the previously compiled router, the Dubbo-Go
runtime must process `EventTypeDel` before attempting to decode the empty
configuration payload. The compatible upstream implementation starts at
[`fd4dea4f8`](https://github.com/apache/dubbo-go/commit/fd4dea4f8ccef79a3190ba1d4a60af775d615135).
Use a release containing that change, or apply its backward-compatible fix,
when validating delete behavior with an older runtime.

## End-to-end validation

The opt-in test in
[`e2e/router-rule-chain/router_rule_chain_e2e_test.go`](../e2e/router-rule-chain/router_rule_chain_e2e_test.go)
uses a real ZooKeeper server and a Dubbo-Go `RouterChain`. It verifies:

1. Admin publishes the public YAML at `/dubbo/config/dubbo/<ruleName>`.
2. A consumer listener receives Add, Update, and Delete events.
3. Affinity and Script rule updates replace the active RouterChain state.
4. Deleting the rule restores the unfiltered invoker list.

Run it only with a ZooKeeper endpoint and a Dubbo-Go revision satisfying the
Script-delete prerequisite above:

```powershell
$env:DUBBO_ADMIN_E2E_ZK_ADDR = 'zookeeper://127.0.0.1:2181'
go test -tags=e2e ./e2e/router-rule-chain -count=1 -v
```

The Admin module deliberately remains on its existing Go 1.24 dependency
baseline. For this cross-repository test, use a local Go workspace or replace
directive that selects the compatible Dubbo-Go source; do not commit that
local override to the Admin module.
