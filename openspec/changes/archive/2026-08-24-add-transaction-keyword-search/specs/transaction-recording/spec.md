## MODIFIED Requirements

### Requirement: 按条件搜索交易
用户 SHALL 能够通过 `search_transactions` 工具,指定 `ledger_id`,并按时间范围、来源、分类、关键词中的一个或多个条件筛选该账本内的交易列表,并通过 `limit`/`cursor` 分批翻页取完结果。关键词筛选通过可选入参 `keyword` 提供,对交易的 `comment` 字段做大小写不敏感的子串匹配;非空白的 `keyword` 与时间范围、来源、分类等其他已提供的筛选条件以 AND 组合。`keyword` 为空字符串或仅由空白字符组成时,视为未提供该参数,不产生任何筛选效果。结果按 `time`、`id` 稳定排序;每次调用最多返回 `limit` 条(未提供时默认为 50,超过 200 的请求被拒绝),当还有更多结果时响应中带一个 `next_cursor`,把它原样传回下一次调用的 `cursor` 入参即可取得下一页;没有更多结果时不返回 `next_cursor`。`cursor` 的有效性校验涵盖包括 `keyword` 在内的全部筛选条件——用与签发时不同的 `keyword` 复用同一个 `cursor` 会被拒绝,而不是静默地把两种筛选条件混合在一起翻页。

#### Scenario: 无筛选条件
- **WHEN** 调用 `search_transactions`,指定一个已存在的账本,不提供任何其他筛选条件
- **THEN** 返回该账本中最早的一页交易(最多 `limit` 条,按 `time`、`id` 排序),不包含其他账本的交易

#### Scenario: 按时间范围筛选
- **WHEN** 调用 `search_transactions` 并指定起止时间
- **THEN** 只在该账本内发生时间落在该区间内的交易中翻页,不受时间范围外的交易影响页大小

#### Scenario: 按关键词筛选
- **WHEN** 调用 `search_transactions` 并提供非空白的 `keyword`,该账本内存在一笔或多笔交易的 `comment` 包含该关键词的子串(大小写不限)
- **THEN** 只返回 `comment` 命中该关键词子串的交易,按 `time`、`id` 排序分页;`comment` 中大小写与 `keyword` 不同、但忽略大小写后仍匹配的交易同样被返回

#### Scenario: 关键词与其他筛选条件组合
- **WHEN** 调用 `search_transactions` 同时提供 `keyword` 与时间范围、`source_id`、`category_id` 中的一个或多个
- **THEN** 只返回同时满足全部已提供筛选条件的交易——`comment` 命中关键词,且落在时间范围内、来源与分类均匹配

#### Scenario: 关键词包含 LIKE 通配符字符
- **WHEN** 调用 `search_transactions` 提供的 `keyword` 中包含 `%` 或 `_` 字符
- **THEN** 这些字符被当作字面字符参与子串匹配,不被解释为 SQL `LIKE` 通配符;只有 `comment` 中恰好包含这些字面字符的交易才会命中

#### Scenario: 关键词为空白
- **WHEN** 调用 `search_transactions` 提供的 `keyword` 是空字符串,或仅由空格、制表符等空白字符组成
- **THEN** 请求按未提供 `keyword` 处理,不因此拒绝请求,也不因此排除任何交易

#### Scenario: 关键词未命中任何交易
- **WHEN** 调用 `search_transactions` 提供的 `keyword` 在该账本内没有任何交易的 `comment` 匹配
- **THEN** 返回一个空列表,不返回 `next_cursor`,而不是错误

#### Scenario: 筛选结果为空
- **WHEN** 调用 `search_transactions` 提供的筛选条件没有匹配到该账本内任何交易
- **THEN** 返回一个空列表,不返回 `next_cursor`,而不是错误

#### Scenario: 结果超过一页
- **WHEN** 调用 `search_transactions`,匹配的交易数量超过 `limit`
- **THEN** 只返回前 `limit` 条,响应中带一个 `next_cursor`

#### Scenario: 使用 cursor 翻页
- **WHEN** 调用 `search_transactions` 并提供上一次响应返回的 `next_cursor` 作为 `cursor`,其余筛选条件(含 `ledger_id`、`keyword`)与上一次调用相同
- **THEN** 返回紧接上一页之后的下一页结果,不重复、不遗漏上一页已返回的交易

#### Scenario: cursor 无效或已不匹配当前筛选条件
- **WHEN** 调用 `search_transactions` 提供的 `cursor` 无法解析,或是用不同的筛选条件(`ledger_id`/`source_id`/`category_id`/`start_time`/`end_time`/`keyword`)签发的
- **THEN** 请求被拒绝,返回说明 cursor 无效的错误,不返回任何交易

#### Scenario: limit 超过上限
- **WHEN** 调用 `search_transactions` 提供的 `limit` 大于 200
- **THEN** 请求被拒绝,返回说明超出上限的错误,而不是静默截断为 200

#### Scenario: 上一页返回后账本发生变化
- **WHEN** 在两次翻页调用之间,该账本内有新交易被创建、或已返回过的交易被更新/删除
- **THEN** 翻页仍然基于 `cursor` 中记录的位置(`time`、`id`)继续,不因为新增/变更的交易而重复返回或跳过尚未返回的交易

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `search_transactions`,指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回说明账本不存在的错误
