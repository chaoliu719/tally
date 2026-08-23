## Why

`search_transactions` 目前不分页,不论筛选条件命中多少笔交易,都会一次性把全部结果通过 MCP 返回给 Agent。账本处于起步阶段时这没问题,但账本积累到几百上千笔交易后,一次宽泛的查询(比如"这个月的交易"、或者不带筛选条件的"全部交易")会把大量原始交易 JSON 塞进 LLM 的上下文——这不是功能缺陷,而是随着账本增长必然会撞上的 token 经济问题,且没有任何手动分批查询的手段可以绕开。需要给 `search_transactions` 加上数量上限与翻页能力。

## What Changes

- `search_transactions` 新增可选入参 `limit`(默认 50,上限 200,超过上限则拒绝而不是静默截断)与 `cursor`(不透明字符串,由上一次调用的响应给出)
- `search_transactions` 响应新增 `next_cursor`(存在更多结果时给出,否则不返回),Agent 需要下一页时把它原样传回 `cursor`
- 分页基于查询已有的稳定排序 `ORDER BY time, id`(见 `internal/store/queries.sql`)做 keyset 分页,不用 `OFFSET`——`OFFSET` 在翻页过程中如果有新交易插入会导致漏行或重复行,而 keyset 不受影响
- 不筛选、返回全部结果的用法仍然可用,只是现在默认按 50 条一页分批返回,而不是没有上限地一次性返回
- 不引入向后翻页(上一页)能力,也不支持"跳到第 N 页"——Agent 场景是持续向后翻页直到拿到需要的结果或 `next_cursor` 消失,不需要更复杂的分页语义

## Capabilities

### New Capabilities
(无)

### Modified Capabilities
- `transaction-recording`:「按条件搜索交易」这条 requirement 的 `search_transactions` 新增 `limit`/`cursor` 入参与 `next_cursor` 出参,默认行为从"一次返回全部匹配结果"变为"每页最多 50 条,需要翻页取完"

## Impact

- 仅修改 `internal/tools/transactions.go` 里 `SearchTransactionsInput`/`SearchTransactionsOutput`/`searchTransactions`,不涉及 `create_transaction`/`get_transaction`/`update_transaction`/`delete_transaction`
- `internal/store/queries.sql` 的 `SearchTransactions` 语句改为支持 keyset 过滤 + `LIMIT`,不改动其筛选条件(account_id/category_id/start_time/end_time)本身
- 不涉及数据库 schema 变更
- 与同期的 `add-financial-analytics` change 相比,两者都会读取 `internal/store/queries.sql`,但落在文件中互不相邻的不同查询语句上(一个改 `SearchTransactions`,一个新增独立的 `Summarize*` 查询),互不修改对方新增/改动的内容
