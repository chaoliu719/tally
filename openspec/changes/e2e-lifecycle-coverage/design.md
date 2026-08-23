## Context

见 [proposal.md](proposal.md)。这里定两件事:`cmd/tally-mcp` 下新增测试文件怎么拆,以及 replay-token 测试具体断言什么。

现状:`cmd/tally-mcp/e2e_test.go` 只有一个 `TestEndToEndMinimalLoop`,`session`/`client`/`call` 的建立逻辑内联在测试函数里;`internal/tools/testutil_test.go` 已经为同样的问题(多个测试函数需要复用同一套 MCP session 建立逻辑)建立了先例(`newTestSession`/`callTool`/`callToolExpectError`)。

## Goals / Non-Goals

**Goals:**
- 补上 write-confirmation 的"令牌重复使用"场景在 `internal/tools` 层的测试
- 把 `cmd/tally-mcp` 的 e2e 测试从"一条最小闭环"扩展到"覆盖这次生命周期能力的贯穿式旅程",且不重复 `internal/tools` 已经测过的校验分支矩阵

**Non-Goals:**
- 不在 e2e 层重新穷举 update/delete 的每种拒绝分支(过期 token、drifted revision、cycle 检测等)——那是 `internal/tools` 的活,已经测过
- 不引入新的测试框架或断言库

## Decisions

### `cmd/tally-mcp` 测试文件不能拆子目录,只能拆多个同包文件

`cmd/tally-mcp` 下所有测试文件都是 `package main`(而非外部测试包 `main_test`),因为要用到未导出的 `buildMux`。Go 不允许 import `main` 包,所以没法把新测试挪到子目录用独立包组织。拆分粒度是"同目录多文件",镜像 `internal/tools` 现在"每个工具一个文件"的做法:

```
cmd/tally-mcp/
├── e2e_testutil_test.go            # 新增:抽取共享的 session/client 建立逻辑
├── e2e_minimal_loop_test.go        # 重命名自 e2e_test.go:TestE2EMinimalLoop(原 TestEndToEndMinimalLoop)
├── e2e_account_lifecycle_test.go   # 新增:账户 create→update→preview delete→apply delete
└── e2e_category_lifecycle_test.go  # 新增:分类嵌套创建→挪动→删除叶子
```

`e2e_testutil_test.go` 提供一个 `newE2ESession(t) (session *mcp.ClientSession, cleanup func())`(或直接用 `t.Cleanup`),内部做 `bootstrap.Config` 构造 + `InitDataStore` + `buildMux` + `httptest.NewServer` + MCP client connect,和现有 `e2e_test.go` 里内联的那段逻辑等价,只是抽出来给多个文件共用;`call` helper 一并挪过去。

`e2e_test.go`/`TestEndToEndMinimalLoop` 顺带改名为 `e2e_minimal_loop_test.go`/`TestE2EMinimalLoop`——这次新增的两个文件都是 `e2e_<scope>_test.go` / `TestE2E<Scope>` 这个模式,原文件的 `e2e_test.go`(没有 scope 后缀)、`TestEndToEndMinimalLoop`(`EndToEnd` 而非 `E2E`)两处都不跟这个模式对齐,顺手统一,行为不变,纯改名 + 换用抽出的 helper。

### balance_adjustment 折进账户旅程,但不能和 delete 作用在同一个账户上

`create_transaction(type="balance_adjustment")` 概念上是账户余额的一部分,折进 `e2e_account_lifecycle_test.go` 的旅程里断言,不单开文件。

但不能像最初设想的那样接在同一个账户的 update 之后、delete 之前:[account-management/spec.md](../../specs/account-management/spec.md) 的删除规则是"被任意交易(含 `balance_adjustment`)引用就永久拒绝删除,不提供强制覆盖",而系统里目前没有 `delete_transaction`——交易一旦记下就删不掉,所以"记一笔 balance_adjustment 之后还能把账户删掉"这条路径在当前实现下**根本走不通**,不是测试写法的问题。(这是补测试过程中发现的一个更大的产品缺口——账户/分类删除的"先清空引用再删除主体"这条路径,因为整个系统都没有 update/delete_transaction 而永久走不通——已经记录为独立的后续 change,不在这次范围内。)

所以 `TestE2EAccountLifecycle` 用两个账户体现两条能力,互不干扰:
- **账户 A**:完整走 create → update → preview delete → apply delete,全程不记任何交易,验证的是"管理一个从未使用过的账户"这条链路
- **账户 B**:create → 记一笔 `balance_adjustment` → `list_accounts` 验证余额变化,不删除(删除会因为已有引用被拒绝,这是预期行为,不是这条旅程要测的东西)

### replay-token 测试测什么

`TestManageAccountDeleteTokenReplay`(及分类版本)结构:
1. 走一次完整的 delete preview → apply,断言 `Status == "deleted"`
2. 用同一个 `confirmation_token`(第 1 步返回的)再调用一次 `operation="delete"`
3. 断言:调用返回工具错误(`callToolExpectError`);`list_accounts`/`list_categories` 中账户/分类数量与第 2 步之前一致(没有 panic、没有二次删除的副作用)

这条路径命中的是 design.md(account-category-lifecycle 那次)里说的"活查兜底"——第二次调用时资源已经不存在,`GetAccount`/`GetCategory` 直接返回 not-found,和 `TestManageAccountDeleteNotFound` 走的是同一段代码,但这次是通过"先删除再重放"这个更贴近真实误用场景的路径触发,而不是构造一个从未存在过的 id。

## Risks / Trade-offs

- **[trade-off] e2e 层旅程测试之间共享 `newE2ESession`,如果 helper 写得不好会让失败信息定位变难。** → 保持 helper 薄(只做 wiring,不做业务断言),断言留在各测试函数里,和 `internal/tools/testutil_test.go` 的风格一致。
