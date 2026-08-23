## Why

`account-category-lifecycle` 交付时,`internal/tools` 层已经逐条覆盖了 account-management/category-management/write-confirmation 三份 spec 的每个分支,但复查时发现一个真实缺口:[write-confirmation/spec.md](../../specs/write-confirmation/spec.md) "用同一个令牌重复确认执行"这条场景——`confirmation_token` 成功 apply 一次之后被重复使用——在 `accounts_test.go`/`categories_test.go`/`confirm_test.go` 里都没有测试覆盖。design.md 明确把这道防线放在"apply 前活查"而不是 `confirm.Verify` 本身,所以只能在 `internal/tools` 层测,`confirm` 包单测测不出来。

同时,`cmd/tally-mcp/e2e_test.go` 只有一条最小闭环旅程(创建账户/分类/交易→搜索→查询),完全没有走过这次改动新增的 update/delete/preview-apply/balance_adjustment 能力经过 `main.go` 真实 wiring(`buildMux`、鉴权中间件、config 组装)的路径——目前这条路径唯一的验证方式是 tasks.md 里那条已勾掉但从未自动化的手工走查(8.2)。

这次改动纯粹是补测试,不改变任何工具的输入输出结构或校验规则,不涉及 spec 行为变化。

## What Changes

- `internal/tools/accounts_test.go` 新增 `TestManageAccountDeleteTokenReplay`:delete 成功 apply 之后,用同一个 `confirmation_token` 再 apply 一次,断言第二次调用报错、账户仍处于已删除状态、没有重复的副作用
- `internal/tools/categories_test.go` 新增对应的分类版本 `TestManageCategoryDeleteTokenReplay`
- `cmd/tally-mcp` 下把 `e2e_test.go` 里内联的 session/client 建立与 `call` helper 抽成共享的 `e2e_testutil_test.go`(镜像 `internal/tools/testutil_test.go` 的做法),供多个 e2e 测试文件复用
- 新增 `cmd/tally-mcp/e2e_account_lifecycle_test.go`:两个账户分别覆盖两条能力(不能合并成一条——`balance_adjustment` 会永久阻止删除,见 design.md):账户 A 走 create → update → preview delete → apply delete;账户 B 走 create → 记一笔 `balance_adjustment` → 验证余额,不删除。全程经过 `main.go` 的 `buildMux` wiring
- 新增 `cmd/tally-mcp/e2e_category_lifecycle_test.go`:三级嵌套分类创建 → 挪动节点 → 删除叶子分类,同样全程经过真实 wiring
- `e2e_test.go` 重命名为 `e2e_minimal_loop_test.go`,`TestEndToEndMinimalLoop` 重命名为 `TestE2EMinimalLoop`,与新增文件的 `e2e_<scope>_test.go`/`TestE2E<Scope>` 命名模式对齐;测试逻辑本身不变,只是改用抽出的共享 helper

## Capabilities

无 spec 行为变化——这次改动只增加测试文件,不修改任何工具的输入/输出契约或校验规则,`.openspec.yaml` 已设置 `skip_specs: true`。

## Impact

- `internal/tools/accounts_test.go`、`internal/tools/categories_test.go`:各新增一个测试函数
- `cmd/tally-mcp/e2e_testutil_test.go`(新增)、`cmd/tally-mcp/e2e_test.go` → `e2e_minimal_loop_test.go`(重命名 + 改用抽出的 helper,保留原测试逻辑)、`cmd/tally-mcp/e2e_account_lifecycle_test.go`(新增)、`cmd/tally-mcp/e2e_category_lifecycle_test.go`(新增)
- 不涉及非测试代码改动
