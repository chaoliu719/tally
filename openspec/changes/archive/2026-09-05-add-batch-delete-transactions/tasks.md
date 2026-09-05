## 1. confirm 包：批量 token 支持

- [x] 1.1 在 `mcp/internal/confirm` 里新增一个批量 payload 类型和 `IssueBatch(secret, action string, items []Item) (token string, expiresAt int64)`，其中 `Item` 是 `{ID, Revision string}`；用单元测试验证签发出的 token 能被 `VerifyBatch` 正确校验通过
- [x] 1.2 新增 `VerifyBatch(secret, token, wantAction string, wantItems []Item, now time.Time) error`，校验签名、action、过期时间，以及确认时的 id 集合是否与 token 里的 id 集合完全一致（不区分顺序）；对逐项 revision 不匹配的情况返回逐项结果而不是直接整体失败；用单元测试覆盖：合法批次、token 过期、action 不匹配、签名被篡改、id 集合不匹配（多传/少传 id）、部分项 revision 已变化
- [x] 1.3 确认现有 `Issue`/`Verify` 及其测试未被改动且仍然通过（`go test ./mcp/internal/confirm/...`）

## 2. batch_delete_transactions 工具

- [x] 2.1 在 `mcp/internal/tools/transactions.go` 里定义 `BatchDeleteTransactionsInput`（`ledger_id`、`ids []string`、`confirmation_token,omitempty`）和 `BatchDeleteTransactionsOutput`（逐项结果：id、状态 [`deleted`/`not_found`/`revision_changed`]、对应交易信息，以及仅预览时返回的 `confirmation_token`/`expires_at`）
- [x] 2.2 实现预览路径：一开始就校验 `len(ids) > 100`，超出直接拒绝并说明上限；否则在给定账本内逐个查找（复用 `deps.Q.GetTransaction`），构建逐项预览结果，并基于找到的各项的 `transactionDeletionRevision` 生成 `IssueBatch` token；用测试验证预览一批"部分存在部分不存在"的 id 时，逐项状态正确且返回合法 token
- [x] 2.3 实现确认路径：先调用 `VerifyBatch`，然后对每一项独立地重新查询、比对 revision，并在各自独立的数据库事务内删除（照搬 `deleteTransaction` 的 begin/check/delete/commit 流程）；逐项记录结果，某一项失败不中断循环；用测试验证"预览后、确认前有一项已被删除（或 revision 已变化）"时，其余项仍能正常删除成功
- [x] 2.4 在 `registerTransactionTools` 里注册 `batch_delete_transactions` 工具，描述里说明两步流程、best-effort 语义和 100 个 id 的上限；通过列出工具列表验证它和 `delete_transaction` 一起出现

## 3. 验证

- [x] 3.1 参照 `e2e_transaction_lifecycle_test.go` 的写法新增一个 e2e 测试：预览一批 → 确认该批 → 核对每一项报告的结果，并通过 `get_transaction` 确认被删除的行确实已经不存在
- [x] 3.2 跑通 transactions/confirm 相关的完整测试套件，并对 `mcp` 模块执行 `go build ./...`，确认没有影响到 `delete_transaction` 或其他交易工具
