## Context

时间线面板的过滤条需要"选一个分类时，其全部子孙分类下的交易也一并算入"。`categories` 表只有 `parent_id`，无материализованный path，嵌套深度不限（`schema.sql`）。仓库里已有一条能算出某分类全部子孙 id 的递归查询 `ListCategoryDescendantIDs`（`queries.sql:108-115`，`WITH RECURSIVE`），但从未在 `SearchTransactions`/`SearchTransactionsDesc` 里被用来构造动态 IN 列表——这两条查询目前所有过滤字段都是 `sqlc.narg` 的单值等值比较，仓库里也没有 `sqlc.slice(` 的先例（`sqlc.yaml` 用的是 `sqlite` engine，其对 `sqlc.slice` 的支持历史上比 postgres engine 弱，具体版本行为未验证）。

## Goals / Non-Goals

**Goals:**
- `search_transactions` 能表达"匹配这个分类，或它的任意层级子孙分类"，且默认行为（不传新参数）与改动前逐字节一致。
- 复用已有、已在别处验证过的 `ListCategoryDescendantIDs`，不重新发明子孙查找逻辑。

**Non-Goals:**
- 不改变 `category_id` 单值精确匹配这条路径的默认语义（`plugin/skills/optimize` 的分类合并流程依赖它）。
- 不在这次改动里给 `get_financial_summary`、`CountTransactionsByCategory` 等独立查询加同样的"含子孙"能力——它们目前不需要，等真的有需求再单独提案。
- 不追求任意 N 元 IN 列表的通用基础设施；只解决"一个分类 + 它的子孙"这一种形状的过滤。

## Decisions

**D1. 新增 `include_descendants` 可选参数，而不是改默认语义或让前端合并多次查询结果。**
- 改默认语义：影响面广，`optimize` skill 的合并流程会静默改变行为（见 proposal.md Impact）。
- 前端合并多次查询：`search_transactions` 是基于 `cursor` 的单流懒加载，一个分类 + N 个子分类就是 N+1 条并行游标流，翻页时要把它们归并排序——复杂度和出 bug 的概率都明显更高，而后端加一个 bool 参数是几行 SQL + 一次已有查询调用就能解决的事。
- 结论：加参数，默认 `false`，向后兼容零风险。

**D2. 子孙 id 的取数方式：Go 侧先调 `ListCategoryDescendantIDs` 拿到 id 列表，再把列表喂回主查询，而不是把递归 CTE 直接内联进 `SearchTransactions` 的 `WHERE` 子句。**
- 内联 `WITH RECURSIVE` 到一个子查询表达式里（`category_id IN (WITH RECURSIVE ... SELECT id FROM descendants)`）在 SQLite 里对"CTE 嵌套在子查询表达式内"的支持是版本相关的，没有在这个代码库里验证过。
- `ListCategoryDescendantIDs` 已经是一条独立、可测试、被其他路径隐含验证过写法正确性的查询；两次查询（先查子孙 id，再查交易）在个人记账规模的数据量下性能上没有实际差异。
- 结论：分两步，复用已验证的查询，不引入未验证的 SQL 嵌套写法。

**D3. 把 id 列表喂回主查询的具体写法：先探路验证 `json_each`，不行再退到手写 IN 占位符。**
- 首选：把 Go 里拿到的 `[]int64`（分类自身 + 子孙）序列化成 JSON 数组字符串，作为一个新的 `sqlc.narg` TEXT 参数传入，SQL 里用 `category_id IN (SELECT value FROM json_each(sqlc.narg('category_ids_json')))`。这是 SQLite 处理动态 IN 列表的标准写法，不依赖 `sqlc.slice`（本仓库无此先例，`sqlite` engine 支持也未验证），改动只是给 sqlc 生成的 `Params` struct 多一个字段。
- 前提风险：`modernc.org/sqlite`（纯 Go、无 cgo 的驱动）默认构建是否启用了 JSON1 扩展，没有在这个代码库里验证过——如果没启用，`json_each` 直接报"no such table function"。
- 备选（如果探路失败）：绕开 sqlc，为这一条查询手写一个 Go 函数，动态拼 `IN (?, ?, ?, ...)` 占位符和对应数量的 `?` 参数。这会在"所有查询都走 sqlc 生成"这条项目约定上开一个例外，但只限这一处，且是 SQLite/database/sql 里最基础、最不会出问题的写法。
- 结论：任务列表里第一项是一个几分钟就能跑完的最小 Go 测试（`SELECT value FROM json_each('[1,2,3]')`），根据结果二选一，不预先赌哪边一定行。

**D4. 分页游标的过滤指纹继续基于请求参数本身（`category_id` + `include_descendants`），不基于展开后的 id 集合。**
- 游标翻页要求"后续页复用与首页完全相同的过滤参数"（`transactions_cursor.go`）；只要 Go handler 在每次请求时都用相同的 `category_id`/`include_descendants` 重新算一遍子孙 id（而不是把某一次算出的 id 集合缓存进游标本身），语义就是稳定的、可重放的，不需要改游标编码格式。

## Risks / Trade-offs

- [风险] `modernc.org/sqlite` 不支持 JSON1 → [缓解] D3 里的探路任务先验证，不行则退到手写 IN 占位符，两条路都已经想清楚，不会卡在一半。
- [风险] 这次改动把原本"纯前端 widget 改动"的范围扩大到后端 SQL/工具 schema，超出了最初 proposal 的 Impact 声明 → [缓解] 已经通过 `openspec-update-change` 把这个决策折回 proposal/spec/tasks，不是代码先斩后奏。
- [风险] 未来如果 `get_financial_summary` 等其他工具也想要"含子孙"的分类过滤，会不会想复制这套逻辑导致重复 → [缓解] 当前不做，等真实需求出现时再决定是否把"分类 id 集合展开"提炼成一个共享的小函数；现在提炼是过度设计。
