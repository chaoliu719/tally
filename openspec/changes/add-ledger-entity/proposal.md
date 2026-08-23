## Why

Tally 目前把"整个 SQLite 数据库"当作隐式的唯一账本——没有账本实体，一个用户的所有账户/来源、分类、交易全部混在一起。这在只记一份账时没问题，但一旦用户想把"个人"和"公司"这类互不相关的资金往来分开记录、分开统计，就无法只用一个数据库文件做到：不同用途的收支、分类语义（同名分类在不同场景下含义不同）、来源标签会互相污染，统计汇总也无法按用途拆分。

引入"账本"(ledger) 作为显式的顶层隔离实体，让唯一用户可以在同一个数据库里维护多个互相隔离的账本，各自拥有独立的来源、分类和交易，互不可见、互不干扰。

## What Changes

- 新增"账本"(ledger) 概念：`id`/`name`/`comment`/`created_at`/`updated_at`，与现有 `categories`/`sources` 同构的极简结构。
- **BREAKING**: 新增 `list_ledgers`/`manage_ledger` MCP 工具，用于账本的查看、创建、更新、删除。
- **BREAKING**: 来源、分类、交易全部归属某个账本，完全隔离，无跨账本共享——`list_sources`/`manage_source`、`list_categories`/`manage_category`、`create_transaction`/`update_transaction`/`search_transactions`、财务分析工具全部新增必填的 `ledger_id` 参数。
- `transactions.ledger_id` 冗余存储（不通过 `source_id` 关联推导），但必须与其 `source_id` 所属账本一致；`categories.parent_id` 隐含约束——父分类必须与子分类同账本。
- 删除保护：账本非空（存在任意来源、分类或交易）时禁止删除，走现有 `write-confirmation` 的 preview → apply 两步流程，与删除来源/分类的保护逻辑同构。
- 新建账本时来源列表、分类树均完全为空，不预置任何默认数据。
- 系统不自动创建默认账本——全新数据库是"零账本"状态，是合法的初始状态；调用方必须显式创建账本后才能创建来源/分类/交易。
- 所有相关 MCP 工具调用方式为显式传参，服务端不维护"当前账本"这类隐式会话状态。
- 明确排除（Non-goals）：不支持多用户；不支持跨账本转账或跨账本汇总（转账功能本身已在 `simplify-account-to-source` 中被否决，不复活）；不做历史数据迁移（项目当前无正式存量数据）；不预置默认账本或默认分类/来源。

## Capabilities

### New Capabilities
- `ledger-management`: 让唯一用户能够通过 `list_ledgers`/`manage_ledger` 查看、创建、更新、删除"账本"——资金记录的顶层隔离容器，为来源/分类/交易提供归属边界。

### Modified Capabilities
- `source-management`: 来源新增必填的 `ledger_id` 归属字段；`list_sources`/`manage_source` 新增必填 `ledger_id` 参数；来源按账本完全隔离，同名来源可以在不同账本下并存；删除保护逻辑不变（仍是"被交易引用则不可删"），但引用范围限定在同一账本内。
- `category-management`: 分类新增必填的 `ledger_id` 归属字段；`list_categories`/`manage_category` 新增必填 `ledger_id` 参数；`parent_id` 新增约束——父分类必须与子分类属于同一账本；分类树按账本完全隔离。
- `transaction-recording`: `create_transaction`/`update_transaction`/`search_transactions` 新增必填 `ledger_id` 参数；交易的 `ledger_id` 冗余存储，须与其 `source_id` 所属账本一致；跨账本操作（如把交易的 `source_id` 改成另一账本下的来源）被拒绝。
- `financial-analytics`: 汇总(`get_financial_summary`)、趋势等分析工具新增必填 `ledger_id` 参数，统计口径按账本隔离，不做跨账本汇总。

## Impact

- **前置依赖**：本变更假定 `simplify-account-to-source`（账户→来源改名瘦身，当前由另一 agent 实施中，尚未合并）已经落地——即 `sources` 表（`id`/`name`）、`transactions.source_id`/`transactions.currency` 列、`income`/`expense`-only 的交易类型已经存在。本变更的规范增量建立在该目标状态之上，需等其合并归档后再实施/归档本变更。
- **存储层**：新增 `ledgers` 表（`id`/`name`/`comment`/`created_at`/`updated_at`）；`sources`/`categories`/`transactions` 三张表新增 `ledger_id` 列（`NOT NULL`，外键指向 `ledgers.id`）；新增按 `ledger_id` 过滤的索引（如 `idx_transactions_ledger_time`）。
- **MCP 工具层**：新增 `list_ledgers`/`manage_ledger`；`list_sources`/`manage_source`、`list_categories`/`manage_category`、交易相关工具、财务分析工具的入参全部新增必填 `ledger_id`。
- **无数据迁移**：项目当前无正式存量数据，`ledger_id` 直接设为 `NOT NULL`，不提供回填/迁移逻辑。
- `openspec/specs/` 下新增 `ledger-management/spec.md`，`source-management`、`category-management`、`transaction-recording`、`financial-analytics` 的 spec 相应增量修改。
