## 1. 前置条件

- [x] 1.1 确认 `simplify-account-to-source` 已合并归档(`sources` 表为 `id`/`name`,`transactions` 有 `source_id`/`currency`,交易类型只剩 `income`/`expense`);若尚未归档,暂停本变更的实施

## 2. Schema 与生成代码

- [x] 2.1 在 `internal/store/schema.sql` 新增 `ledgers` 表(`id`/`name`/`comment`/`created_at`/`updated_at`),`CREATE TABLE IF NOT EXISTS` 建表并可通过 `sqlite3 <db> .schema ledgers` 核对字段
- [x] 2.2 在 `internal/store/schema.sql` 给 `sources`/`categories`/`transactions` 新增 `ledger_id INTEGER NOT NULL REFERENCES ledgers(id)` 列,新增按 `ledger_id` 过滤的索引(如 `idx_sources_ledger`、`idx_categories_ledger`、`idx_transactions_ledger_time`),用 `.schema` 核对列与索引均已生效
- [x] 2.3 在 `internal/store/queries.sql` 新增账本相关查询(`ListLedgers`/`GetLedger`/`CreateLedger`/`UpdateLedger`/`DeleteLedger`,以及判断账本是否为空的查询),并给来源/分类/交易/分析相关查询加上 `ledger_id` 过滤条件与"记录是否属于指定账本"的校验查询
- [x] 2.4 运行 sqlc 重新生成 `internal/store/queries.sql.go`/`querier.go`/`models.go`,确认生成命令(如 `sqlc generate`)成功退出且无手工改动残留

## 3. 工具层:账本管理

- [x] 3.1 新建 `internal/tools/ledgers.go`,实现 `list_ledgers` 工具,单元测试覆盖"零账本返回空列表"与"已有账本返回全部"两个场景
- [x] 3.2 在 `internal/tools/ledgers.go` 实现 `manage_ledger` 的 `create`/`update` 操作,单元测试覆盖创建成功、缺少名称被拒绝、更新不存在账本返回未找到
- [x] 3.3 在 `internal/tools/ledgers.go` 实现 `manage_ledger` 的 `delete` 操作(复用 `write-confirmation` 的 preview → apply),单元测试覆盖:空账本可删除、非空账本(存在来源/分类/交易任一)被拒绝且不签发 `confirmation_token`、目标账本不存在

## 4. 工具层:来源与分类的账本隔离

- [x] 4.1 给 `internal/tools/sources.go` 的 `list_sources`/`manage_source` 全部操作新增必填 `ledger_id` 参数及校验(账本不存在、来源存在但不属于该账本均按规范返回相应错误),更新/补充单元测试覆盖 [source-management/spec.md](specs/source-management/spec.md) 中新增的场景
- [x] 4.2 给 `internal/tools/categories.go` 的 `list_categories`/`manage_category` 全部操作新增必填 `ledger_id` 参数,`parent_id` 校验逻辑扩展为"必须属于同一账本",更新/补充单元测试覆盖 [category-management/spec.md](specs/category-management/spec.md) 中新增的场景(含 parent_id 指向另一账本被拒绝)

## 5. 工具层:交易的账本隔离

- [x] 5.1 给 `internal/tools/transactions.go` 的 `create_transaction`/`update_transaction` 新增必填 `ledger_id` 参数,写入前校验 `source_id`/`category_id` 均属于该账本,单元测试覆盖跨账本引用被拒绝、账本不存在被拒绝
- [x] 5.2 给 `get_transaction`/`delete_transaction` 新增必填 `ledger_id` 参数,`ledger_id` 与交易实际所属账本不一致时统一返回"未找到",单元测试覆盖该场景
- [x] 5.3 给 `search_transactions` 新增必填 `ledger_id` 参数并作为查询的强制过滤条件,`cursor` 编解码逻辑纳入 `ledger_id`(与其他筛选条件同样触发"cursor 不匹配"校验),单元测试覆盖跨账本翻页/cursor 不匹配场景

## 6. 工具层:财务分析的账本隔离

- [x] 6.1 给 `internal/tools/analytics.go` 的 `get_financial_summary` 新增必填 `ledger_id` 参数,汇总、按分类拆分、按来源拆分均按该账本过滤,单元测试覆盖多账本数据互不混入同一次汇总结果

## 7. 集成测试与文档

- [x] 7.1 更新 `cmd/tally-mcp` 下的 e2e 测试,新增一条跨账本隔离的端到端场景(在两个账本下分别创建同名来源/分类/交易,确认互不可见、互不影响汇总)
- [x] 7.2 更新 `README.md` 中列出的工具入参说明,补充 `ledger_id` 及新增的 `list_ledgers`/`manage_ledger` 工具
- [x] 7.3 运行完整测试套件(`go test ./...`)确认全部通过
