## 1. Store 层聚合查询

- [x] 1.1 在 `internal/store/queries.sql` 末尾追加 `SummarizeTransactionsByCurrency`(按 `accounts.currency` 分组,`SUM(CASE WHEN type='income' THEN amount ELSE 0 END)`/`SUM(CASE WHEN type='expense' THEN -amount ELSE 0 END)`/`SUM(CASE WHEN type='balance_adjustment' THEN amount ELSE 0 END)`,支持可选 `start_time`/`end_time`),不修改已有查询语句
- [x] 1.2 追加 `SummarizeTransactionsByCategory`(按 `category_id` + `accounts.currency` 分组,只统计 `type IN ('income','expense')`,同样支持可选时间范围)
- [x] 1.3 追加 `SummarizeTransactionsByAccount`(按 `account_id` + `accounts.currency` 分组,只统计 `type IN ('income','expense')`,同样支持可选时间范围)
- [x] 1.4 运行 sqlc 生成代码(参照仓库现有生成命令),确认 `internal/store` 下生成了对应的 `SummarizeTransactionsByCurrency`/`ByCategory`/`ByAccount` 方法与参数类型,且 `go build ./...` 通过

## 2. MCP 工具

- [x] 2.1 新建 `internal/tools/analytics.go`:注册只读工具 `get_financial_summary`,输入 `start_time`/`end_time`(可选,unix 秒,语义与 `search_transactions` 一致),复用 [transactions.go](internal/tools/transactions.go) 里 `SearchTransactionsInput` 对可选时间范围的处理方式(参考,不导入依赖)
- [x] 2.2 实现 handler:调用 1.1-1.3 的三条 store 查询,组装为响应结构 `TotalsByCurrency`(收入/支出/净额)、`ByCategory`(分类拆分列表)、`ByAccount`(账户拆分列表)、`BalanceAdjustmentByCurrency`(余额调整总额),字段命名与 jsonschema 描述参照 [accounts.go](internal/tools/accounts.go)/[transactions.go](internal/tools/transactions.go) 现有工具的风格(每个字段带 `jsonschema` tag 说明含义)
- [x] 2.3 在 `internal/tools` 的工具注册入口里挂上 `registerAnalyticsTools`(沿用 `init()` + `register(...)` 的既有模式,参照 [categories.go](internal/tools/categories.go) 顶部写法),确认 `tools.RegisterAll` 会注册到新工具

## 3. 测试

- [x] 3.1 新建 `internal/tools/analytics_test.go`,复用 `newTestSession` 建立测试会话:覆盖「指定时间范围统计」「不提供时间范围统计全部历史」两个场景,验证 `go test ./internal/tools/...` 通过
- [x] 3.2 补充「范围内无匹配交易返回空聚合而非错误」「多币种账本分别按币种返回」两个场景的测试用例
- [x] 3.3 补充「按分类拆分」「按账户拆分」的测试用例:构造分布在多个分类/账户下的交易,验证返回列表只包含实际发生过交易的分类/账户
- [x] 3.4 补充「balance_adjustment 单独统计、不计入总收入/支出/净额/分类/账户拆分」的测试用例,覆盖 spec 中「范围内存在」与「范围内不存在」两个场景

## 4. 收尾

- [x] 4.1 运行完整测试套件(`go test ./...`)确认无回归
- [x] 4.2 更新 README(若其中列出了 MCP 工具清单)加入 `get_financial_summary`
