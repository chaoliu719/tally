## Context

见 [proposal.md](proposal.md) - Why。本设计假定 `simplify-account-to-source`(账户→来源改名瘦身,当前另有 agent 实施中,尚未合并归档)已经落地:`sources` 表只剩 `id`/`name`,`transactions` 表通过 `source_id` 关联来源、自带必填的 `currency` 列,交易类型只剩 `income`/`expense`,不再有 `adjustment`,也不存在账户间转账。本变更的三份能力增量(`source-management`、`transaction-recording`、`financial-analytics`)都是在这个目标状态之上再叠加账本隔离,因此必须晚于 `simplify-account-to-source` 归档才能实施/归档。

项目本身没有独立的迁移框架——`internal/store/schema.sql` 用 `CREATE TABLE IF NOT EXISTS` 一次性建表,没有正式存量数据(见项目 context:"数据:全新账本,全新 SQLite 数据库文件")。

## Goals / Non-Goals

**Goals:**
- 引入 `ledgers` 表(`id`/`name`/`comment`/`created_at`/`updated_at`)作为顶层隔离容器。
- `sources`/`categories`/`transactions` 三张表新增 `ledger_id`(`NOT NULL`,外键指向 `ledgers.id`),实现来源、分类、交易按账本完全隔离。
- 所有涉及来源/分类/交易/分析的 MCP 工具新增必填 `ledger_id` 参数,包括仅凭主键 id 就能定位记录的操作(如 `get_transaction`/`delete_transaction`/来源与分类的 update/delete)——`ledger_id` 与记录实际所属账本不一致时,统一按"未找到"处理,不泄露记录存在于别的账本这一事实。
- 新增 `list_ledgers`/`manage_ledger` 工具,`manage_ledger` 的删除操作走 `write-confirmation` 的 preview → apply 两步流程。

**Non-Goals:**
- 不支持多用户——账本隔离的是"用途"(个人/公司等),不是"人"。
- 不支持跨账本操作——不做跨账本转账、不做跨账本聚合统计、不允许把交易的 `source_id`/`category_id` 指向另一个账本。
- 不做历史数据迁移——项目当前没有需要保留的正式数据,`ledger_id` 直接设为 `NOT NULL`。
- 不自动创建默认账本,也不在新建账本时预置任何默认来源或分类。
- 不实现账本归档/停用(软删除)语义——项目全局没有软删除概念,这里也不引入例外。

## Decisions

**账本字段取最小集合(`id`/`name`/`comment`/`created_at`/`updated_at`),不加 `is_default`、不加 `archived`。**
和 `categories`/`sources` 保持同构,遵循项目一贯的极简风格。`is_default` 不需要,因为所有工具都强制显式传 `ledger_id`,没有"缺省账本"的语义空间;`archived` 目前没有对应需求,真要加时可以作为独立的小改动补上。

**`sources`/`categories`/`transactions` 全部按账本完全隔离,不设跨账本共享(如全局来源标签池)。**
考虑过的替代方案:让 `sources` 脱离账本、作为全局标签池共享——因为 `simplify-account-to-source` 已经把来源简化成纯标签(不再带 currency/type/balance),这个方案初看更省事。但否决了它,理由和"分类"完全隔离的既有论证一致:同名标签(如"支付宝")在不同账本下往往指代不同的资金用途,共享会引入"删除一个来源要检查它是否被其他账本引用"之类的跨账本耦合,增加而不是减少复杂度。

**所有 MCP 工具(包括仅凭 id 即可定位记录的 `get_transaction`/`delete_transaction`/来源与分类的 update/delete)都强制要求传 `ledger_id`,并在其与记录实际所属账本不一致时统一返回"未找到"。**
考虑过只在 `create`/`list`/`search` 这类"需要新建立范围"的操作上强制 `ledger_id`,id 已经唯一定位记录的操作(update/delete/get)可以不强制。否决:一是不一致的参数契约会让调用方(Agent)难以记住"哪些工具要传、哪些不用";二是"一致返回未找到"这种防御性设计,能在 Agent 传错 `ledger_id`(比如记混了两个账本)时提前暴露问题,而不是意外地跨账本改到了同名记录。

**`transactions.ledger_id` 冗余存储,不通过 `source_id` 关联 `sources.ledger_id` 推导。**
与 `simplify-account-to-source` 把 `currency` 从"账户"搬到"交易"自身的设计取向一致——"这笔交易属于哪个账本"由交易自己的字段决定,不依赖 JOIN 到主数据表。代价是需要在写入时校验 `ledger_id` 与 `source_id`/`category_id` 实际所属账本一致,换来的是所有按账本过滤的查询(尤其是 `search_transactions`/`get_financial_summary`)可以直接走 `WHERE ledger_id = ?` 加专用索引,不必每次都 JOIN `sources`/`categories`。

**服务端不维护"当前账本"这类会话状态,所有工具调用显式传参。**
与项目既有的"无用户、无会话"哲学一致;HTTP 传输下的隐式全局状态在并发调用场景中本身就是风险点。代价是调用方每次都要显式传 `ledger_id`,但对 Agent 调用方而言,显式反而更可靠——不用担心上一轮对话切换了账本却忘记切回来导致误操作。

**删除账本要求账本非空(存在来源、分类或交易)时拒绝,不提供级联删除或强制覆盖。**
账本是新引入的顶层容器,一次级联删除的影响范围比现有"删来源/删分类"大一个数量级,即使配合 preview 也容易低估影响。代价是清空一个账本需要先手动删光其下的来源、分类、交易(多轮 preview → apply),但换来的是"删除账本"这个操作本身不可能在一次误操作里丢掉大量历史数据。

## Risks / Trade-offs

- [实施顺序依赖 `simplify-account-to-source`] → 本变更的三份修改型规范增量(`source-management`/`transaction-recording`/`financial-analytics`)都建立在该变更的目标状态之上,如果它的字段/工具名最终调整,这里也要同步调整。缓解:两个变更保持独立、顺序归档,`simplify-account-to-source` 先落地并归档,再实施本变更。
- [所有工具新增必填参数,调用体量增加] → 每次调用来源/分类/交易/分析相关工具都要多传一个 `ledger_id`,对调用方(Agent)是额外的负担,且现有测试/文档里所有相关工具的入参 schema 都要同步更新。这是"完全隔离 + 无会话状态"这两条已确认决策的直接代价,不是遗留风险。
- [无级联删除,清空账本手续多] → 见上文 Decisions,是刻意的取舍,换取避免误删的安全性。

## Migration Plan

项目没有正式用户数据,不提供旧数据迁移。实施步骤(前提:`simplify-account-to-source` 已合并归档):
1. 改 `internal/store/schema.sql`:新增 `ledgers` 表;`sources`/`categories`/`transactions` 新增 `ledger_id INTEGER NOT NULL REFERENCES ledgers(id)`;新增按 `ledger_id` 过滤的索引(如 `idx_sources_ledger`、`idx_categories_ledger`、`idx_transactions_ledger_time`)。
2. 改 `internal/store/queries.sql`,所有相关查询新增 `ledger_id` 过滤条件与校验查询(如"某 id 是否属于指定账本"),跑 sqlc 重新生成 `internal/store/queries.sql.go`/`querier.go`。
3. 改 `internal/tools` 下新增 `ledgers.go`(`list_ledgers`/`manage_ledger`),并给来源/分类/交易/分析相关工具的输入结构体新增必填 `ledger_id` 字段及相应校验逻辑。
4. 改 `internal/tools` 下对应的单元测试与 `cmd/tally-mcp` 下的 e2e 测试,覆盖跨账本引用被拒绝、账本非空不可删等场景。
5. 改 `README.md` 里提到工具入参的地方,补充 `ledger_id`。
6. 本地已有的测试用 SQLite 文件如果还在用旧 schema,直接删除重建即可,不需要额外迁移脚本。

回滚策略:改动集中在这一个 change 内,没有对外发布过,直接回退这个 change 的提交即可。
