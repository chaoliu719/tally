## Context

`SearchTransactions`(`internal/store/queries.sql`)当前已经是一条带可选过滤 + keyset 分页的查询:`source_id`/`category_id`/`start_time`/`end_time` 用 `sqlc.narg` 表达"未提供则不过滤",`after_time`/`after_id` 承载翻页位置,`ORDER BY time, id` + `LIMIT` 收尾。工具层(`internal/tools/transactions.go`)的 `searchTransactions` 负责把 `SearchTransactionsInput` 翻译成 `store.SearchTransactionsParams`,并用 `internal/tools/transactions_cursor.go` 里的 `searchTransactionsFilterFields`/`encodeSearchTransactionsCursor`/`decodeSearchTransactionsCursor` 处理不透明 cursor——cursor 内容是 `{last_time, last_id, filter_fingerprint}`,`filter_fingerprint` 是对当次查询用到的筛选字段序列化后取 SHA-256,解码时用当前请求的筛选字段重新计算并比对,不一致就拒绝(见已归档的 `add-transaction-search-pagination` change 的 design.md)。

其余细节参见 proposal.md - Why。

## Goals / Non-Goals

**Goals:**
- `keyword` 命中判定是大小写不敏感的字面子串匹配,用户输入里恰好出现的 `%`/`_` 不会被当成 SQL 通配符
- `keyword` 参与 cursor 的 `filter_fingerprint`,翻页过程中途切换 `keyword` 必须让旧 cursor 失效,行为与现有的 `source_id`/`category_id`/`start_time`/`end_time` 完全一致
- 空白 `keyword` 的处理方式与本工具其余可选字符串参数(`source_id`/`category_id` 用 `in.X != ""` 判断"是否提供")保持一致,不引入新的"报错 vs 忽略"分歧

**Non-Goals:**
- 不引入 FTS5 或任何全文索引——个人账本量级下,`comment` 上的全表 `LIKE` 扫描足够快,落在 config context 里"只有必须在全量数据上用 DB 速度算才正确的才进服务端"判据的下限,不需要为一个够用的方案预先做性能优化
- 不支持 `keyword` 之外的其他文本匹配模式(前缀匹配、正则、多关键词 AND/OR 组合)——先例检索目前只需要"这段文本历史上出现过吗",更复杂的匹配语义留给需求明确后的后续 change
- 不改变现有筛选字段(`source_id`/`category_id`/`start_time`/`end_time`)的语义或 `limit`/`cursor` 分页机制本身,只是把 `keyword` 接入同一套过滤 + fingerprint 框架

## Decisions

**LIKE 通配符转义:转义 `\`、`%`、`_` 三个字符,再用 `ESCAPE '\'` 子句**
拼接查询前对用户提供的 `keyword`做转义:先把字面反斜杠 `\` 替换成 `\\`,再把 `%` 替换成 `\%`、`_` 替换成 `\_`(顺序不能反,否则会把转义序列本身的反斜杠再转义一次)。查询用 `LOWER(comment) LIKE '%' || LOWER(escaped_keyword) || '%' ESCAPE '\'`——转义字符复用 SQL 标准写法里最常见的 `\`,`comment` 中真实出现反斜杠、百分号、下划线的场景在个人账单场景里存在(例如金额、编号里的 `_`），转义能保证这些字符仍被当作字面字符处理,不会误判命中范围。

**大小写不敏感的实现:`LOWER()` 包裹两侧,不依赖 SQLite 的 `LIKE` 默认大小写行为**
SQLite 的 `LIKE` 对 ASCII 默认已经大小写不敏感,但对超出 ASCII 范围的字符(项目里 `comment` 允许任意用户文本,包括中文商户名及非 ASCII 文字)`LIKE` 不做大小写折叠。中文场景没有大小写问题,但为避免"是否大小写不敏感"的行为在 ASCII 和非 ASCII 字符之间不一致给用户带来困惑,显式用 `LOWER()` 包裹 `comment` 与 `keyword` 双方,行为对所有输入统一、可预期,且不需要额外扩展或依赖 SQLite 的 `ICU` 扩展。

**空白 `keyword` 视为未提供,而不是报错**
`SearchTransactionsInput.SourceID`/`CategoryID` 均已经用 `in.X != ""` 判断"是否提供该过滤条件",空字符串一律等价于"不筛选"。`keyword` 沿用同一惯例:trim 后为空(空字符串或纯空白)时不加入 SQL 过滤、也不参与 fingerprint 计算差异(等价于未提供)。选择"视为未提供"而不是"报错拒绝",是因为空白输入对这个字段没有歧义——不像"必填字段缺失"那样代表调用方的错误,更接近"调用方选择不使用这个可选筛选维度",报错反而会打断 Agent 用统一模板构造请求参数的简单性(不需要额外判断 keyword 是否该被省略）。

**`keyword`(trim 后的规范值)纳入 `searchTransactionsFilterFields`,参与 `filter_fingerprint`**
在 `internal/tools/transactions_cursor.go` 的 `searchTransactionsFilterFields` 结构体中新增一个字段(建议 `Keyword string`,存储 trim 后的规范值;未提供时为空字符串,与"视为未提供"的规则天然一致,不需要额外的 `sql.NullString`),让它和 `LedgerID`/`SourceID`/`CategoryID`/`StartTime`/`EndTime` 一起参与 `searchTransactionsFilterFingerprint` 的 JSON 序列化 + SHA-256。这样翻页中途切换 `keyword`(包括从"有 keyword"切到"无 keyword",或反过来)会让 fingerprint 不再匹配,触发规范中「cursor 无效或已不匹配当前筛选条件」的既有错误路径,不需要新的错误类型或校验逻辑。

**分页条件下推到 SQL,不在 Go 里再做一次子串扫描**
`SearchTransactions` 查询新增 `AND (sqlc.narg('keyword') IS NULL OR LOWER(comment) LIKE '%' || LOWER(sqlc.narg('keyword')) || '%' ESCAPE '\')`,延续现有 `sqlc.narg` 可选过滤的写法,`keyword` 为空(未提供)时该条件恒真。转义在 Go 层完成后再把转义后的字符串传给这个查询参数,SQL 侧不需要感知转义逻辑本身。

## Risks / Trade-offs

- [风险] `comment` 上没有索引,`LIKE '%...%'` 前导通配符导致 SQLite 无法使用任何索引,查询退化为全表扫描 → 缓解:这是本变更明确接受的权衡(见 Non-Goals),个人账本量级(几千到几万笔交易)下全表扫描的耗时可忽略;如果账本规模远超预期导致这条查询变慢,是一个独立的、有明确触发信号(实测延迟)的后续优化问题,不是这次要提前解决的
- [风险] 转义规则如果遗漏边界情况(比如用户 `keyword` 以反斜杠结尾)可能产生 SQL 语法错误或匹配行为异常 → 缓解:转义函数只需处理三个固定字符且顺序固定(先转义 `\` 本身,再转义 `%`/`_`),tasks.md 里要求为这些边界情况(纯反斜杠、以反斜杠结尾、同时包含 `%`/`_`/`\`)补充单元测试
- [风险] `filter_fingerprint` 的字段集合此前只覆盖四个筛选参数,新增 `keyword` 后所有历史签发的 cursor(理论上仅存在于测试或极短时间的使用窗口内,因为这是同一批功能的迭代)会因为结构变化而在下次解码时产生不匹配 → 缓解:cursor 本身在设计上就是"不保证跨版本兼容"的不透明短生命周期令牌(见 `add-transaction-search-pagination` design.md 的 Non-Goals),旧 cursor 解码失败时返回的正是规范里已定义的「cursor 无效」错误,不是需要额外处理的新场景

## Open Questions

(无)
