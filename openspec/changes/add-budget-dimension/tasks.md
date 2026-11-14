# Tasks

## 1. 存储层:预算表与交易的预算列

- [x] 1.1 在 `schema.sql` 增加 `budgets` 表、`transactions.budget_id` 列与索引,在 `queries.sql` 增加预算的增删改查与按预算统计交易数的查询,运行 `sqlc generate`,验证生成代码编译通过
- [x] 1.2 在 `store.Open` 增加 `ensureTransactionBudgetColumn`(照 `ensureTransactionMerchantColumn` 的 `PRAGMA table_info` 范式)与一次性种子逻辑(`PRAGMA user_version` 0→1),验证旧库打开后列被加上、种子只建一次、删光预算重启后不重建

## 2. 预算管理工具

- [x] 2.1 实现 `list_budgets` 与 `manage_budget`(create/update/delete,delete 走 preview → apply 且被引用时拒绝),验证 `e2e` 覆盖创建、改名、跨账本拒绝、被引用不可删、空账本返回空列表

## 3. 交易的预算字段

- [x] 3.1 `create_transaction` 与 `update_transaction` 增加必填 `budget_id` 校验(拒绝缺失、`"0"`、不存在、跨账本),`get_transaction` 与 `search_transactions` 返回 `budget_id` 并支持按它筛选(含 `"0"`),验证缺预算被拒、收入交易同样要求预算、按未分配筛选能命中旧交易
- [x] 3.2 确认 `search_transactions` 的 cursor 把 `budget_id` 纳入有效性校验,验证用不同 `budget_id` 复用 cursor 被拒绝

## 4. 批量更新交易

- [x] 4.1 实现 `batch_update_transactions`:patch 只收 `category_id`/`source_id`/`budget_id`,目标用 id 列表或筛选条件指定,复用 `internal/confirm` 的 preview → apply 与 revision 核对,沿用批量上限,验证空 patch 被拒、金额字段被拒、新值跨账本在预览即被拒、确认时已变化的项单独报告且不影响其余、过期 token 被拒

## 5. 财务汇总与建账

- [x] 5.1 `get_financial_summary` 增加 `by_budget`(含 `budget_id = 0` 单独成组)与可选 `budget_id` 筛选,验证各预算支出小计与未分配组之和等于总支出、多币种分开、筛选不存在的预算被拒
- [x] 5.2 `manage_ledger`(`operation="create"`)在同事务内种入四个种子预算,验证新账本创建后即有 `固定`/`专项`/`突发`/`日常` 四个预算、且没有来源与分类

## 6. 实践层 skill

- [x] 6.1 更新 `plugin/skills/record/SKILL.md`:预算按性质判断、定不了查商户再查分类先例、仍定不了或金额异常就问、预览带预算与依据、修正时必带 `budget_id`,验证文中每个 `update_transaction` 示例都带 `budget_id`
- [x] 6.2 更新 `query`、`analysis`、`export` 三个 skill:明细呈现预算名称、按预算拆分时收支分开且点明单月不作结构判断、导出不携带预算且交付时说明,验证三份文档都提到预算且 export 明确写"不进导出文件"
- [x] 6.3 更新 `plugin/skills/optimize/SKILL.md`:合并与拆分改用 `batch_update_transactions`,删除"没有批量更新工具、需逐笔调用"的说法,验证文档不再要求逐笔 `update_transaction` 完成合并

## 7. 端到端核对

- [x] 7.1 跑 `mcp/` 下全部 `e2e` 与单元测试,验证无回归,特别是交易记录、搜索翻页、财务汇总、分类与来源删除这几组既有用例

## Workflow follow-up

- 部署后回填存量交易:用 `batch_update_transactions` 把 `budget_id = 0` 的交易分配到真实预算,哪个分类或商户进哪个预算由使用方判断,不在代码里做。
- 回填后核对:`search_transactions` 按 `budget_id = "0"` 筛选为空,`get_financial_summary` 各预算支出之和等于总支出。
- 归档本 change 并同步 specs。
