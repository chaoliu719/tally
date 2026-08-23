# transaction-recording Specification

## Purpose
让唯一用户能够通过 MCP 工具记录一笔收入或支出交易,按 ID 查询单条交易、按条件搜索交易列表,并对已存在的交易做更新或删除。这是账本从空到可用、再到可维护的核心闭环;批量操作留给后续 change。

## Requirements

### Requirement: 记录一笔收入或支出交易
用户 SHALL 能够通过 `create_transaction` 工具记录一笔收入或支出类型的交易,指定所属账本(`ledger_id`)、来源、分类、金额、币种与发生时间。来源与分类都必须属于指定的账本;分类可以是该账本中任意已存在的分类,不要求处于某个特定层级。

#### Scenario: 提供有效信息记录交易
- **WHEN** 调用 `create_transaction`,指定的 `ledger_id` 已存在,指定的来源与分类均已存在且属于该账本,金额、币种、发生时间均合法
- **THEN** 交易被记录并归属该账本,随后可以通过 `get_transaction` 按其 ID 查到

#### Scenario: 引用不存在的来源或分类
- **WHEN** 调用 `create_transaction`,指定的来源 ID 或分类 ID 在指定的账本中不存在
- **THEN** 请求被拒绝,返回说明引用无效的错误,不记录交易

#### Scenario: 来源或分类属于另一个账本
- **WHEN** 调用 `create_transaction`,指定的来源 ID 或分类 ID 实际存在,但属于另一个账本而不是提供的 `ledger_id`
- **THEN** 请求被拒绝,返回说明引用无效的错误,不记录交易

#### Scenario: 引用一级分类
- **WHEN** 调用 `create_transaction`,指定的分类是一个没有父分类的顶层分类
- **THEN** 交易照常被记录——分类不再区分层级,曾经"必须使用二级分类"的限制不再适用

#### Scenario: 缺少必填字段
- **WHEN** 调用 `create_transaction`,缺少 `ledger_id`、来源、分类、金额、币种或发生时间中的任意一项
- **THEN** 请求被拒绝,不记录交易

#### Scenario: 币种不受支持
- **WHEN** 调用 `create_transaction`,指定的币种代码不是系统支持的货币
- **THEN** 请求被拒绝,返回说明币种不受支持的错误,不记录交易

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `create_transaction`,指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回说明账本不存在的错误,不记录交易

### Requirement: 按 ID 查询单条交易
用户 SHALL 能够通过 `get_transaction` 工具,提供交易 ID 与其所属 `ledger_id`,查询该笔交易的完整信息。

#### Scenario: 查询已存在的交易
- **WHEN** 调用 `get_transaction`,提供一个已记录交易的 ID 与其实际所属的 `ledger_id`
- **THEN** 返回该交易的来源、分类、金额、币种、发生时间等完整信息

#### Scenario: 查询不存在的交易
- **WHEN** 调用 `get_transaction`,提供的 ID 不对应任何已记录的交易
- **THEN** 返回明确的"未找到"错误,而不是空结果或崩溃

#### Scenario: ledger_id 与交易实际所属账本不一致
- **WHEN** 调用 `get_transaction`,提供的 ID 对应一笔已存在的交易,但提供的 `ledger_id` 不是它实际所属的账本
- **THEN** 返回"未找到"错误,而不是该交易的信息

### Requirement: 按条件搜索交易
用户 SHALL 能够通过 `search_transactions` 工具,指定 `ledger_id`,并按时间范围、来源、分类中的一个或多个条件筛选该账本内的交易列表,并通过 `limit`/`cursor` 分批翻页取完结果。结果按 `time`、`id` 稳定排序;每次调用最多返回 `limit` 条(未提供时默认为 50,超过 200 的请求被拒绝),当还有更多结果时响应中带一个 `next_cursor`,把它原样传回下一次调用的 `cursor` 入参即可取得下一页;没有更多结果时不返回 `next_cursor`。

#### Scenario: 无筛选条件
- **WHEN** 调用 `search_transactions`,指定一个已存在的账本,不提供任何其他筛选条件
- **THEN** 返回该账本中最早的一页交易(最多 `limit` 条,按 `time`、`id` 排序),不包含其他账本的交易

#### Scenario: 按时间范围筛选
- **WHEN** 调用 `search_transactions` 并指定起止时间
- **THEN** 只在该账本内发生时间落在该区间内的交易中翻页,不受时间范围外的交易影响页大小

#### Scenario: 筛选结果为空
- **WHEN** 调用 `search_transactions` 提供的筛选条件没有匹配到该账本内任何交易
- **THEN** 返回一个空列表,不返回 `next_cursor`,而不是错误

#### Scenario: 结果超过一页
- **WHEN** 调用 `search_transactions`,匹配的交易数量超过 `limit`
- **THEN** 只返回前 `limit` 条,响应中带一个 `next_cursor`

#### Scenario: 使用 cursor 翻页
- **WHEN** 调用 `search_transactions` 并提供上一次响应返回的 `next_cursor` 作为 `cursor`,其余筛选条件(含 `ledger_id`)与上一次调用相同
- **THEN** 返回紧接上一页之后的下一页结果,不重复、不遗漏上一页已返回的交易

#### Scenario: cursor 无效或已不匹配当前筛选条件
- **WHEN** 调用 `search_transactions` 提供的 `cursor` 无法解析,或是用不同的筛选条件(`ledger_id`/`source_id`/`category_id`/`start_time`/`end_time`)签发的
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

### Requirement: 更新一笔交易
用户 SHALL 能够通过 `update_transaction` 工具,用交易 ID 与其所属 `ledger_id`,替换一笔已存在交易的全部可变字段(`type`/`source_id`/`category_id`/`amount`/`currency`/`time`/`comment`)。这是完整字段替换语义,不支持只修改其中一部分字段;字段校验规则与 `create_transaction` 完全一致(income/expense 需要合法 `category_id` 且 `amount` 为正;`currency` 必须是系统支持的货币;`source_id`/`category_id` 必须属于该交易所在的账本)。更新不允许把交易挪到另一个账本——`source_id` 只能引用同一账本内的来源。更新不需要二次确认,直接执行。

#### Scenario: 提供完整合法字段更新交易
- **WHEN** 调用 `update_transaction`,提供一笔已存在交易的 ID 与其实际所属的 `ledger_id`,以及完整、合法的 `type`/`source_id`/`category_id`/`amount`/`currency`/`time`/`comment`(来源与分类均存在且属于该账本,金额/分类规则与指定的 `type` 相符,`currency` 受支持)
- **THEN** 该交易被替换为新值,随后 `get_transaction` 返回更新后的信息

#### Scenario: 缺少必填字段
- **WHEN** 调用 `update_transaction`,缺少 `id`、`ledger_id`、`type`、`source_id`、`amount`、`currency`、`time` 中的任意一项
- **THEN** 请求被拒绝,交易不发生变化

#### Scenario: 引用不存在的来源或分类
- **WHEN** 调用 `update_transaction`,指定的 `source_id` 或 `category_id` 在交易所属账本中不存在(包括该 id 实际属于另一个账本的情况)
- **THEN** 请求被拒绝,返回说明引用无效的错误,交易不发生变化

#### Scenario: 币种不受支持
- **WHEN** 调用 `update_transaction`,指定的 `currency` 代码不是系统支持的货币
- **THEN** 请求被拒绝,返回说明币种不受支持的错误,交易不发生变化

#### Scenario: income/expense 金额非正或缺少分类
- **WHEN** 调用 `update_transaction`,`type` 为 `income` 或 `expense`,但 `category_id` 缺失、或 `amount` 不是正数
- **THEN** 请求被拒绝,交易不发生变化

#### Scenario: 更新不存在的交易
- **WHEN** 调用 `update_transaction`,提供的 `id` 不对应任何已记录的交易
- **THEN** 返回明确的"未找到"错误,不发生任何变化

#### Scenario: ledger_id 与交易实际所属账本不一致
- **WHEN** 调用 `update_transaction`,提供的 `id` 对应一笔已存在的交易,但提供的 `ledger_id` 不是它实际所属的账本
- **THEN** 返回"未找到"错误,不发生任何变化

### Requirement: 删除一笔交易
用户 SHALL 能够通过 `delete_transaction` 工具,以 preview → apply 两步确认(复用 `write-confirmation` 机制),提供交易 ID 与其所属 `ledger_id`,删除一笔已存在的交易记录。与来源、分类删除不同,删除交易没有"是否被其他数据引用"这类前置门槛——同一账本内任意已存在的交易都允许被删除。

#### Scenario: 预览删除一笔存在的交易
- **WHEN** 调用 `delete_transaction`,不携带 `confirmation_token`,提供的 `id` 与 `ledger_id` 对应一笔已存在的交易
- **THEN** 返回该交易的完整信息、一个 `confirmation_token`、该令牌的过期时间,以及 `pending_confirmation` 状态,不产生任何数据变化

#### Scenario: 确认删除
- **WHEN** 调用 `delete_transaction`,携带上一步预览返回的、仍然有效的 `confirmation_token`
- **THEN** 该交易被删除;随后 `get_transaction` 返回"未找到",`search_transactions` 不再包含它

#### Scenario: 预览或确认一笔不存在的交易
- **WHEN** 调用 `delete_transaction`(带或不带 `confirmation_token`),提供的 `id` 不对应任何已记录的交易,或提供的 `ledger_id` 与交易实际所属账本不一致
- **THEN** 请求被拒绝,返回明确的"未找到"错误,不签发或消费 `confirmation_token`

#### Scenario: 预览之后交易已被并发修改或删除
- **WHEN** 携带一个此前签发的 `confirmation_token` 调用 `delete_transaction`,但该交易自签发以来已被 `update_transaction` 修改、或已被删除
- **THEN** 请求被拒绝,返回说明状态已变化、需要重新预览的错误,不执行删除

#### Scenario: 清空来源下全部交易后可以删除该来源
- **WHEN** 一个来源名下的全部交易都已通过 `delete_transaction` 成功删除
- **THEN** 对该来源调用 `manage_source`(`operation="delete"`)可以正常完成 preview → apply 流程并成功删除该来源

#### Scenario: 清空分类下全部交易后可以删除该分类
- **WHEN** 一个分类下的全部交易都已通过 `delete_transaction` 成功删除,且该分类没有子分类
- **THEN** 对该分类调用 `manage_category`(`operation="delete"`)可以正常完成 preview → apply 流程并成功删除该分类
