# Manual E2E verification: Affinity and Script Router

This guide runs the opt-in integration test for the two router rules. It is
intended for a developer who wants to verify the real configuration propagation
path locally before submitting or reviewing a change.

```text
Dubbo Admin ZK governor
  -> /dubbo/config/dubbo/<ruleName>
  -> Dubbo-Go dynamic-configuration listener
  -> RouterChain
```

The test does not mock the configuration listener or the `RouterChain`. It
creates a real ZooKeeper-backed rule and checks that the already-created
Dubbo-Go consumer chain changes its selected invokers after create, update, and
delete operations.

## What this test proves

| Rule | Create | Update | Delete |
| --- | --- | --- | --- |
| Affinity | Publishes `<application>.affinity-router` using public `affinityAware` YAML; a `region=beijing` consumer selects only the Beijing provider. | Changes `ratio` from `50` to `60`, which makes the chain fall back to both providers. | The listener receives `Del` and the chain remains restored to both providers. |
| Script | Publishes `<application>.script-router` with `type: javascript`; the script selects one provider. | Replaces the script so that it selects the other provider. | The listener receives `Del` and the chain restores both providers. |

The generated application name is unique for each run. The test uses two
in-memory provider invokers: the Beijing provider is on port `20880`, and the
Shanghai provider is on port `20881`.

## Scope and prerequisites

This is a runtime propagation E2E test. It verifies the Admin governor, the
ZooKeeper data path, the Dubbo-Go listener, and router behavior. It does not
start the Admin HTTP server or Vue application, execute a real RPC over the
network, or run against a real Nacos server.

You need:

1. The branch containing the E2E test (for example,
   `feat/affinity-script-router-integration` during integration validation).
2. Go `1.24.0`, matching [`go.mod`](../../go.mod).
3. Docker, or another reachable ZooKeeper server.
4. A Dubbo-Go runtime that includes the Script Router delete fix. The upstream
   fix begins at
   [`fd4dea4f8`](https://github.com/apache/dubbo-go/commit/fd4dea4f8ccef79a3190ba1d4a60af775d615135).
   For the currently pinned Admin dependency, a local checkout with the
   backport can be used instead:
   `D:\environment\github\dubbo-go\dubbo-go-script-delete-fix`.

The delete prerequisite is important: an older Script Router tries to parse
the empty delete payload, reports `EOF`, and leaves the old script active.

## 1. Start ZooKeeper

Open PowerShell terminal A and start an isolated local ZooKeeper instance:

```powershell
docker run --rm --name dubbo-admin-e2e-zk -d -p 2181:2181 zookeeper:3.9
Test-NetConnection 127.0.0.1 -Port 2181
```

Continue only when the `TcpTestSucceeded` result is `True`. If you already
have ZooKeeper, skip Docker and use its address in the test command below.

## 2. Select a compatible Dubbo-Go checkout without changing `go.mod`

Do not add a `replace` directive to this repository's `go.mod`. Instead,
create a Go workspace outside the Admin repository. The following example
assumes the Admin repository and the compatible Dubbo-Go checkout are siblings
under `D:\environment\github\dubbo-go`.

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

If you use a released Dubbo-Go version that already contains the upstream fix,
point the workspace at that module instead. The test imports Affinity and
registers the Script Router factory itself, so no additional imports are needed
for the test process.

## 3. Run the E2E test

In PowerShell terminal B:

```powershell
Set-Location D:\environment\github\dubbo-go\dubbo-admin
$env:DUBBO_ADMIN_E2E_ZK_ADDR = 'zookeeper://127.0.0.1:2181'
go test -tags=e2e ./e2e/router-rule-chain -count=1 -v
```

`-count=1` is required so that Go's test cache cannot hide a failed or skipped
run. A successful result ends with:

```text
--- PASS: TestAdminRulesReachDubboGoRouterChain
PASS
```

Do not treat an exit code of zero by itself as success. Without
`DUBBO_ADMIN_E2E_ZK_ADDR`, the test deliberately reports `SKIP` and exits zero
because it cannot prove the external configuration path.

## 4. Read the expected behavior

The test waits up to ten seconds for each configuration event and routing
result. On success, it has verified all of the following:

### Affinity Router

1. Admin writes `<application>.affinity-router` to
   `/dubbo/config/dubbo/<ruleName>`.
2. The stored YAML contains `affinityAware:` and does **not** leak the internal
   protobuf field name `affinity:`.
3. The Dubbo-Go listener receives an Add event and the chain changes from two
   invokers to the single Beijing invoker (`20880`).
4. Updating `ratio: 50` to `ratio: 60` produces an Update event (or Add for an
   older compatible ZooKeeper listener) and restores both invokers.
5. Deleting the rule produces a Delete event and leaves the chain unfiltered
   with both invokers.

### Script Router

1. Admin writes `<application>.script-router` to
   `/dubbo/config/dubbo/<ruleName>` and the stored YAML contains
   `type: javascript`.
2. The listener receives an Add event; the initial JavaScript chooses one
   provider.
3. Updating the JavaScript chooses the other provider, proving that the
   compiled router is hot-replaced rather than merely re-read.
4. Deleting the rule produces a Delete event and restores both invokers. This
   is the regression check for the empty-payload Script Router delete bug.

## Failure diagnosis

| Symptom | Likely cause | Action |
| --- | --- | --- |
| Test shows `SKIP` | `DUBBO_ADMIN_E2E_ZK_ADDR` is unset. | Set it to a reachable `zookeeper://host:port` address and run again with `-count=1`. |
| Cannot connect to ZooKeeper | Container is not ready, port `2181` is occupied, or the address is wrong. | Run `Test-NetConnection`, inspect `docker logs dubbo-admin-e2e-zk`, or choose a different reachable server. |
| Script delete times out or reports `EOF` | The selected Dubbo-Go version lacks the delete fix. | Use a checkout/release containing `fd4dea4f8` or the compatible backport, then recreate the Go workspace. |
| No router update within 10 seconds | The config listener cannot observe the same ZooKeeper endpoint or a router factory is not registered in a real consumer. | Verify the address, then check the consumer prerequisites in [the compatibility document](../../docs/affinity-script-router-compatibility.md#dubbo-go-consumer-prerequisites). |
| Result still has two invokers after a create | The expected rule semantics or provider metadata differs from this fixture. | Inspect the test's consumer/provider URLs and the `ratio`/script assertions in [`router_rule_chain_e2e_test.go`](router_rule_chain_e2e_test.go). |

## 5. Clean up

After the test, remove only the environment override and stop the disposable
ZooKeeper container:

```powershell
Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
Remove-Item Env:DUBBO_ADMIN_E2E_ZK_ADDR -ErrorAction SilentlyContinue
docker stop dubbo-admin-e2e-zk
```

The test deletes the ZooKeeper rules it created. The external Go workspace is
outside this repository and can be retained for future runs.

For the complete Admin/Dubbo-Go YAML and configuration-center contract, see
[Affinity and Script Router compatibility](../../docs/affinity-script-router-compatibility.md).
