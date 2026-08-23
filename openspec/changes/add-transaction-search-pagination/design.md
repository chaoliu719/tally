## Context

`SearchTransactions` 目前的查询已经是 `ORDER BY time, id`(见 [internal/store/queries.sql:94](internal/store/queries.sql:94)),没有 `LIMIT`。`time` 不唯一(同一秒可以有多笔交易),单独按 `time` 做翻页游标会漏行/重复行,必须带上 `id` 做 tiebreaker,这也是现有排序已经这么写的原因。

参见 proposal.md - Why。

## Goals / Non-Goals

**Goals:**
- 翻页在"两次调用之间账本发生变化"时仍然正确(不漏、不重复),这是 keyset 分页相对 `OFFSET` 分页的核心优势,也是 spec 里「上一页返回后账本发生变化」场景的硬要求
- `cursor` 是不透明字符串,Agent 不需要、也不应该解析或构造它的内容

**Non-Goals:**
- 不支持向后翻页或跳页(proposal.md 已声明)
- 不对 `cursor` 做防篡改签名——这不是授权凭证,伪造或损坏的 cursor 最坏情况是查询结果不对或报错,不会绕过任何权限或产生数据变化,所以不复用 [internal/confirm](internal/confirm) 那套 HMAC 签名机制,避免为一个纯正确性问题引入安全机制的复杂度

## Decisions

**cursor 编码:base64url(JSON),内容为 `{last_time, last_id, filter_fingerprint}`**
`filter_fingerprint` 是对本次查询用到的 `account_id`/`category_id`/`start_time`/`end_time` 四个筛选参数序列化后取的 SHA-256(复用 [internal/tools/accounts.go](internal/tools/accounts.go) 里 `accountDeletionRevision` 已经用过的"序列化定长字段再哈希"模式,但不需要那套的签名/密钥,只是纯校验用途)。解码 `cursor` 时,用当前请求的筛选参数重新计算 fingerprint 并比对:不一致就是 spec 里「cursor 无效或已不匹配当前筛选条件」的错误,直接拒绝而不是静默换用新条件继续翻页(那样会返回一个两种条件杂糅、语义不清的结果集)。

**分页条件下推到 SQL,不在 Go 里做 `LIMIT`**
`SearchTransactions` 查询新增 `AND (time, id) > (sqlc.narg('after_time'), sqlc.narg('after_id'))`(SQLite 支持行值比较)以及 `LIMIT sqlc.arg('limit')`,`after_time`/`after_id` 为空(首页)时该条件恒真。继续复用现有的可选参数模式(`sqlc.narg`),不新增查询、只扩展已有的 `SearchTransactions` 语句。

**`limit` 校验在 Go 层做,不依赖 SQL**
未提供时默认为 50;大于 200 直接拒绝(而不是 clamp 到 200)——clamp 会让 Agent 以为自己拿到了 200 条完整结果,实际上被服务端悄悄改写了请求语义,拒绝更符合"Agent 需要准确知道自己请求了什么"的原则。

**"是否还有下一页"的判断:多查一条**
`LIMIT` 实际传 `limit + 1`;如果返回条数等于 `limit + 1`,说明还有下一页——丢弃第 `limit+1` 条(它只是"哨兵"),只把前 `limit` 条返回给调用方,并用其中第 `limit` 条(即实际返回的最后一条)的 `(time, id)` 作为 `next_cursor` 里的 `last_time`/`last_id`。这样只需一次查询就能同时拿到"这一页的数据"和"是否还有下一页",不需要额外的 `COUNT(*)` 查询。

## Risks / Trade-offs

- [风险] `filter_fingerprint` 用的字段列表如果未来 `search_transactions` 新增筛选条件(比如按 `type` 筛选),旧版本签发的 cursor 结构不会自动涵盖新字段 → 缓解:fingerprint 只在同一个 change 里和当前四个筛选字段绑定,新增筛选条件属于未来的独立 change,到时候一并更新 fingerprint 计算,不是这次要解决的问题
- [风险] SQLite 的行值比较 `(time, id) > (?, ?)` 语法/性能取决于 SQLite 版本与查询规划器 → 缓解:项目已经在用 `idx_transactions_time` 索引;如果行值比较在 modernc.org/sqlite 上表现不佳,等价写法 `time > ? OR (time = ? AND id > ?)` 可以达到同样效果,属于实现细节,不影响 spec 行为,留给 tasks 阶段按实测情况选择

## Open Questions

(无)
