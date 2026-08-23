## 1. 实现

- [x] 1.1 修改 `internal/tools/ledgers.go` 中 `manage_ledger` 工具的 description(约第 33-38 行),去掉"operation=update replaces name and/or comment on an existing ledger (at least one must be provided)"的部分更新措辞,改为说明 `name` 与 `comment` 必须一并提供(完整替换)
- [x] 1.2 修改 `internal/tools/ledgers.go` 中 `ManageLedgerInput.Name`/`Comment` 字段的 jsonschema 描述(约第 76-77 行),去掉"required for create, optional for update"/"for update, only changed when non-empty"的措辞,改为 `name`/`comment` 对 create 和 update 均为必填
- [x] 1.3 修改 `internal/tools/ledgers.go` 中 `updateLedger` 函数(约第 121-162 行):将 `if in.Name == "" && in.Comment == ""` 的"至少一个非空"校验,改为分别校验 `in.Name == ""` 和 `in.Comment == ""`(仿照 `createLedger`/`updateSource`/`updateCategory` 的必填字段校验风格),并删除"未提供的字段保留 `existing.Name`/`existing.Comment` 原值"的合并逻辑,改为直接使用 `in.Name`/`in.Comment` 更新
- [x] 1.4 跑 `go build ./...` 确认编译通过

## 2. 测试

- [x] 2.1 修改 `internal/tools/ledgers_test.go` 中依赖"部分更新"行为的既有用例(约第 60-106 行:先只传 `Comment` 保留 `Name` 原值、再只传 `Name` 保留 `Comment` 原值那个用例),改为断言这两种调用都被拒绝并返回缺少字段的错误
- [x] 2.2 在 `internal/tools/ledgers_test.go` 新增/确认覆盖:只传 `name` 未传 `comment` 时更新被拒绝;只传 `comment` 未传 `name` 时更新被拒绝;同时提供合法 `name` 与 `comment` 时更新成功且两个字段都生效
- [x] 2.3 跑 `go test ./internal/tools/...` 确认全部通过
- [x] 2.4 `cmd/tally-mcp/` 下的 e2e 测试不需要改:已确认 `e2e_ledger_isolation_test.go` 只测了 `manage_ledger` 的 `create`/`delete`,没有测 `update`,全仓库也没有其他 e2e 用例调用过 `manage_ledger` 的 `operation=update`

## 3. 文档与收尾

- [x] 3.1 检查 `README.md` 中是否有描述 `manage_ledger` 更新语义的文字,如有则同步措辞为完整替换
- [x] 3.2 跑 `openspec validate ledger-update-full-replacement --strict` 确认这次变更本身校验通过
