## MODIFIED Requirements

### Requirement: 按条件搜索交易
用户 SHALL 能够通过 `search_transactions` 工具,按时间范围、账户、分类中的一个或多个条件筛选交易列表,并通过 `limit`/`cursor` 分批翻页取完结果。结果按 `time`、`id` 稳定排序;每次调用最多返回 `limit` 条(未提供时默认为 50,超过 200 的请求被拒绝),当还有更多结果时响应中带一个 `next_cursor`,把它原样传回下一次调用的 `cursor` 入参即可取得下一页;没有更多结果时不返回 `next_cursor`。

#### Scenario: 无筛选条件
- **WHEN** 调用 `search_transactions` 且不提供任何筛选条件
- **THEN** 返回账本中最早的一页交易(最多 `limit` 条,按 `time`、`id` 排序),而不是全部交易

#### Scenario: 按时间范围筛选
- **WHEN** 调用 `search_transactions` 并指定起止时间
- **THEN** 只在发生时间落在该区间内的交易中翻页,不受时间范围外的交易影响页大小

#### Scenario: 筛选结果为空
- **WHEN** 调用 `search_transactions` 提供的筛选条件没有匹配到任何交易
- **THEN** 返回一个空列表,不返回 `next_cursor`,而不是错误

#### Scenario: 结果超过一页
- **WHEN** 调用 `search_transactions`,匹配的交易数量超过 `limit`
- **THEN** 只返回前 `limit` 条,响应中带一个 `next_cursor`

#### Scenario: 使用 cursor 翻页
- **WHEN** 调用 `search_transactions` 并提供上一次响应返回的 `next_cursor` 作为 `cursor`,其余筛选条件与上一次调用相同
- **THEN** 返回紧接上一页之后的下一页结果,不重复、不遗漏上一页已返回的交易

#### Scenario: cursor 无效或已不匹配当前筛选条件
- **WHEN** 调用 `search_transactions` 提供的 `cursor` 无法解析,或是用不同的筛选条件(`account_id`/`category_id`/`start_time`/`end_time`)签发的
- **THEN** 请求被拒绝,返回说明 cursor 无效的错误,不返回任何交易

#### Scenario: limit 超过上限
- **WHEN** 调用 `search_transactions` 提供的 `limit` 大于 200
- **THEN** 请求被拒绝,返回说明超出上限的错误,而不是静默截断为 200

#### Scenario: 上一页返回后账本发生变化
- **WHEN** 在两次翻页调用之间,有新交易被创建、或已返回过的交易被更新/删除
- **THEN** 翻页仍然基于 `cursor` 中记录的位置(`time`、`id`)继续,不因为新增/变更的交易而重复返回或跳过尚未返回的交易
