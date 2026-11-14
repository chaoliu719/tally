# Design

## Context

见 `proposal.md` 的 Why。这里只列约束设计的现状:

- 没有迁移框架。`schema.sql` 只在启动时执行 `CREATE TABLE IF NOT EXISTS`,对已存在的表不起作用。加列的先例是 `store.ensureTransactionMerchantColumn`(`mcp/internal/store/conn.go`):启动时 `PRAGMA table_info` 检查,缺列才 `ALTER TABLE ... ADD COLUMN`。
- SQLite 对已有数据的表加 `NOT NULL` 列必须带常量默认值,不能事后加外键或 CHECK。
- 跨表一致性(如 `source_id` 必须属于交易所在账本)在 Go 层校验,不靠数据库约束(`schema.sql` 里 `ledger_id` 的注释)。
- 破坏性与批量操作走 `internal/confirm` 的 preview → apply,`batch_delete_transactions` 是完整先例:token 内含每笔的 `(id, revision)`,确认时逐笔核对,变了的单独报告、不影响同批其余。
- id 过工具边界一律是十进制字符串。`categories.parent_id = "0"` 是既有的"无父级"哨兵。

## Goals / Non-Goals

**Goals:**

- 预算与来源、分类同级:一张平铺的 `budgets` 表,交易上一列 `budget_id`。
- 升级已有数据库不丢数据、不重建表。
- 批量更新复用 `batch_delete_transactions` 的确认机制,不另起一套。

**Non-Goals:**

- 见 `proposal.md` 的 Non-goals。本设计额外排除:不在数据库里建立分类与预算的任何关系(映射、默认、继承都不做),两者的对应由实践层从交易历史现算。

## Decisions

### 1. `budget_id` 用 `0` 表示"尚未分配",而不是 `NULL`

`ALTER TABLE ... ADD COLUMN budget_id INTEGER NOT NULL DEFAULT 0`。`0` 不是任何真实预算的 id(自增从 1 起),所以它天然不可能撞上真实数据。

选 `0` 而不是 `NULL`:与 `parent_id = 0` 的哨兵惯例一致,读取时不用处理空值;也满足"加 `NOT NULL` 列必须有常量默认值"的限制。

不在数据库层加"必须大于 0"的约束:SQLite 加列时加不上 CHECK,而重建整张 `transactions` 表换来的只是把一条 Go 层校验下推到数据库。跨表一致性本来就在 Go 层守,这里保持一致。`create_transaction` / `update_transaction` 拒绝 `0` 与不存在的 id;只有升级前的旧交易会带着 `0`,直到被回填。

### 2. 种子预算只建一次,用 `PRAGMA user_version` 标记

每个账本种子四条:`固定`、`专项`、`突发`、`日常`。

不能靠"预算表为空就补种"来判断:那样用户删光预算再重启,种子会自己长回来。用 `PRAGMA user_version` 从 0 升到 1 标记"已种过",只在版本为 0 时补种已有账本。`user_version` 目前无人使用,借它顺便给以后的迁移留一个版本位。

新建账本在 `manage_ledger`(`operation="create"`)里同事务种入,不走这个启动路径。

### 3. 批量更新是 patch,单笔更新保持全量替换

`update_transaction` 是全量替换,这是既有语义,不动。`batch_update_transactions` 必须是 patch:要求调用方把几百笔的金额、时间、备注全带上重发,既无意义也容易改坏。

两个工具语义不同,靠名字和描述区分,不另造 `batch_patch` 之类的新词:描述里各写一句与另一个的区别。

patch 只收 `category_id`、`source_id`、`budget_id`。这三个是引用字段,取值来自封闭集合,"这批全改成 X"成立。`amount`、`time`、`currency`、`merchant`、`comment` 不收:没有"一个值适用于多笔"的语义,且 `comment` 受不可变纪律保护。

### 4. 批量更新的目标用筛选条件或 id 列表指定,确认机制复用删除

目标二选一:一组交易 id,或一组 AND 组合的筛选条件(`category_id`、`source_id`、`budget_id`、`start_time`、`end_time`)。`budget_id` 允许传 `"0"`,这样"找出还没回填的"就是一次预览,不用另开工具。

预览把筛选条件解析成具体的 `(id, revision)` 集合再签发 token,确认时逐笔核对 revision,与 `batch_delete_transactions` 同一套 `internal/confirm`。不在 token 里只存筛选条件的哈希:那样确认时得重放筛选,期间新落入范围的交易会被误改。

批量上限沿用删除的那个数。

### 5. 汇总里"尚未分配"单独成组,不隐藏

`get_financial_summary` 的 `by_budget` 把 `budget_id = 0` 的交易作为一组返回。隐藏它会让各预算之和小于总额,而且让"还有交易没回填"变得不可见。各预算支出小计与该组之和等于总支出,是回填是否完成的核对依据。

## Risks / Trade-offs

- [升级后、回填前,新交易必须带预算而旧交易没有] → `0` 在查询和汇总里都可见,不隐藏;回填是上线步骤的一部分,见 Migration Plan。
- [批量更新的 token 内含每笔的 id 与 revision,交易量大时体积可观] → 沿用删除的批量上限封顶单批规模;超过上限分批。不在本期把 token 改成只存摘要。
- [全量替换的 `update_transaction` 与 patch 的 `batch_update_transactions` 语义不同,调用方可能混用] → 两个工具的描述里互相指向对方的差异。
- [`user_version` 被占用后,将来别的迁移不能再用 0→1 这个跃迁] → 版本只升不降,后续迁移用更高的版本号。

## Migration Plan

1. 部署新版本。启动时:`schema.sql` 建 `budgets` 表;`ensureTransactionBudgetColumn` 给已有 `transactions` 加 `budget_id`(默认 0);`user_version` 为 0 时给每个已有账本种四条预算,然后升到 1。
2. 回填。用 `batch_update_transactions` 把 `budget_id = 0` 的交易分批分配到真实预算。这一步是数据操作,不在代码里做——哪个分类或商户该进哪个预算由使用方判断,不写进服务端。
3. 核对。`search_transactions` 按 `budget_id = "0"` 筛选应为空;`get_financial_summary` 各预算支出之和应等于总支出。

回滚:加列与建表都是增量的,旧版本不读 `budget_id` 也不读 `budgets` 表,可以不删数据直接退回旧版本。已回填的预算分配会保留在列里,重新升级后仍在。

## Open Questions

无。批量上限取与 `batch_delete_transactions` 相同的值,不需要单独决定。
