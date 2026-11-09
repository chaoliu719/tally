# Proposal

## Why

tally 目前没有独立的商户字段,银行账单上的商户原名只能塞进 `comment`,和自己想写的备注混在同一段文本里。结果是:商户名不可靠地结构化、备注被商户名污染、按商户做先例检索和价格对比只能依赖「原文里碰巧包含这个词」。用户需要把「钱付给了谁」和「自己想补充什么」分开记录。

## What Changes

- 交易新增可空字段 `merchant`(商户原名):由 Agent 从银行账单或用户口述中提取,原样存储、不做归一化;没有商户的现金/个人往来交易留空。
- `comment` 语义收窄为用户自己的备注,不再承担「必须同时包含商户原名与商品明细」的职责。
- `create_transaction` / `update_transaction` 新增可选入参 `merchant`;`get_transaction` / `search_transactions` 的返回新增 `merchant`。`update_transaction` 仍是全量替换语义,漏传 `merchant` 会像漏传 `comment` 一样被清空。
- `search_transactions` 的 `keyword` 从「只匹配 `comment`」扩大为「匹配 `merchant` 或 `comment`」,保证按商户名检索先例、查历史、做价格对比继续可用。
- 时间线 widget 把 `merchant` 作为主标题、`comment` 作为副行展示;两者都为空时回退到分类名。
- 更新 `record` / `query` / `export` 三个 skill 的纪律:商户进 `merchant`、备注进 `comment`;导出「共享记账」CSV 时「备注」列只写 `comment`,`merchant` 不导出。
- 历史交易不做数据迁移:`merchant` 留空,原 `comment` 原样保留,keyword 仍能通过 `comment` 命中老记录。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `transaction-recording`: 交易模型新增 `merchant`;创建/更新/查询/搜索结果包含该字段;keyword 同时匹配 `merchant` 与 `comment`。
- `plugin-record-skill`: 记录与修正交易时区分商户名与备注,更新 `update_transaction` 全量替换的漏字段纪律。
- `plugin-query-skill`: keyword 匹配范围扩大;回答「这笔是什么」时的字段呈现顺序。
- `plugin-export-skill`: 明确「备注」列只写 `comment`,`merchant` 不进入导出文件。
- `transaction-timeline-widget`: 时间线行内分别展示 `merchant` 与 `comment`。

## Impact

- `mcp/internal/store/schema.sql`、`queries.sql`、sqlc 重新生成的 `models.go` / `queries.sql.go`。
- `mcp/internal/store/conn.go`:启动时对既有数据库做幂等 `ALTER TABLE ... ADD COLUMN merchant`,不删库、不改写历史行。
- `mcp/internal/tools/transactions.go`:工具入参/出参、keyword 查询参数、删除确认 revision。
- `mcp/internal/widgets/src/timeline/index.html`:时间线行渲染。
- `plugin/skills/record|query|export/SKILL.md`。
- 测试:单测、迁移测试、e2e keyword 检索。
- 线上已有 SQLite 数据必须原地升级,发布依赖新镜像(tag 触发 GHCR)。
