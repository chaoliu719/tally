# Tasks

## 1. 存储层与迁移

- [ ] 1.1 在 `mcp/internal/store/schema.sql` 的 transactions 表新增 `merchant TEXT NOT NULL DEFAULT ''`,并同步更新 `queries.sql` 中 Create/Get/Search(升序+降序)/Update 的列清单;验证:运行 `sqlc generate` 成功,`go build ./...` 通过
- [ ] 1.2 把 `SearchTransactions` / `SearchTransactionsDesc` 的 keyword 条件改为同时匹配 `comment` 与 `merchant`(保留 `%`/`_` 的字面转义);验证:新增的查询层测试里,只命中 merchant、只命中 comment、两列都命中的交易都各只返回一次
- [ ] 1.3 在 `mcp/internal/store/conn.go` 的 `Open()` 中增加幂等迁移:用 `PRAGMA table_info(transactions)` 判断,缺列才执行 `ALTER TABLE transactions ADD COLUMN merchant TEXT NOT NULL DEFAULT ''`;验证:新增迁移测试用旧 schema 建库、插入一行、调用 `Open()`,断言该行仍在、`merchant` 为空字符串
- [ ] 1.4 运行 `sqlc generate` 并检查生成结果没有遗漏手工改动的列;验证:`git diff` 中 `models.go` / `queries.sql.go` 只包含 merchant 相关变化,`go test ./internal/store/...` 通过

## 2. MCP 工具层

- [ ] 2.1 在 `TransactionInfo`、`CreateTransactionInput`、`UpdateTransactionInput` 增加 `merchant`(可选,`omitempty`),并在 `createTransaction` / `updateTransaction` / `toTransactionInfo` 中透传;验证:`go test ./internal/tools/...` 覆盖创建后 `get_transaction` 返回的 merchant 与写入一致
- [ ] 2.2 把 `merchant` 纳入 `transactionDeletionRevision` 的哈希字段;验证:新增测试修改一笔交易的 merchant 后,旧删除令牌被判定为 `revision_changed`
- [ ] 2.3 更新 `create_transaction` / `update_transaction` / `search_transactions` 的工具描述,明确 merchant 是可选商户原名、comment 是用户备注、更新为全量替换;验证:`grep` 工具注册处描述与实际参数一致,`go vet ./...` 通过
- [ ] 2.4 补工具层测试:keyword 命中 merchant、keyword 命中 comment、同一笔两列都命中不重复;更新时漏传 merchant 会清空;不传 merchant 仍可正常创建;验证:`go test ./internal/tools/... -run 'Merchant|Keyword'` 全部通过

## 3. 时间线 widget

- [ ] 3.1 修改 `mcp/internal/widgets/src/timeline/index.html` 的行渲染:主标题按 `merchant` > `comment` > 分类名 > `#id` 回退,备注与分类名作为副行且可同时存在;验证:`go test ./internal/widgets/...` 通过,并用 `preview.js` 的样例数据在 preview 中确认四种回退组合
- [ ] 3.2 在 `mcp/internal/widgets/src/timeline/preview.js` 的样例交易里加入 `merchant` 字段,覆盖「有商户有备注」「只有备注」两种展示;验证:本地打开 preview 页面,肉眼确认主标题/副行没有合并或空白

## 4. Plugin skill 纪律

- [ ] 4.1 重写 `plugin/skills/record/SKILL.md`:商户原名写 `merchant`、用户备注写 `comment`,删除「comment 必须同时承载商户名和明细」与「comment 是唯一可检索字段」的表述,并补充 `update_transaction` 漏传 merchant 会被清空的陷阱;验证:通读 skill 不再出现旧纪律,且先例检索、预览、拆分、修正四节都同时提到两个字段
- [ ] 4.2 更新 `plugin/skills/query/SKILL.md`:keyword 同时匹配 merchant 与 comment,回答「这笔是什么」时按 merchant → comment → 分类/来源的顺序;验证:grep 不到「按 comment 子串命中」这类旧表述
- [ ] 4.3 更新 `plugin/skills/export/SKILL.md`:`备注` 列只写 `comment`,明确 merchant 不拼接、不新增列;验证:grep 确认导出映射里没有把 merchant 写进备注的指令
- [ ] 4.4 更新 `plugin/skills/optimize/SKILL.md` 的合并/拆分步骤:调用 `update_transaction` 时除 comment 外也要原样带上 merchant;验证:通读该 skill,所有「保持原样」的字段列表都包含 merchant
- [ ] 4.5 检查 `plugin/skills/analysis/SKILL.md`:keyword 语义变化后其价格对比步骤仍然成立,必要时只补一句「keyword 同时匹配商户名与备注」;验证:该 skill 不出现与新字段矛盾的表述

## 5. 集成验证与发布

- [ ] 5.1 补一条 e2e:通过 MCP 创建带 merchant 的交易,再用 keyword 搜商户名命中;验证:`go test ./cmd/tally-mcp/... -run Merchant` 通过
- [ ] 5.2 运行完整检查:`gofmt -l .` 无输出、`go vet ./...`、`go build ./...`、`go test ./... -race -count=1` 全部通过
- [ ] 5.3 更新 `openspec/changes/add-transaction-merchant` 的 tasks 状态并运行 `openspec validate add-transaction-merchant`;验证:输出 `Change ... is valid`

## Workflow follow-up

- 合并到 `main` 后打 `v*` tag,等 GHCR 镜像发布。
- 线上 `docker compose pull && docker compose up -d`,确认启动日志无 schema 错误。
- 用一笔新交易验证 `get_transaction` 分别返回 merchant/comment,且 `search_transactions(keyword=<商户名>)` 能命中;再读一笔老交易确认 merchant 为空、comment 未变。
- 归档 change 并 sync 主 specs。
