## Purpose

让唯一用户能够通过 MCP 工具记录一笔收入或支出交易,并按 ID 查询单条交易、按条件搜索交易列表。这是账本从空到可用的核心闭环;更新、删除、批量操作、transfer 类型交易留给后续 change。

## ADDED Requirements

### Requirement: 记录一笔收入或支出交易
用户 SHALL 能够通过 `create_transaction` 工具记录一笔收入或支出类型的交易,指定所属账户、分类、金额、币种与发生时间。

#### Scenario: 提供有效信息记录交易
- **WHEN** 调用 `create_transaction`,指定的账户与分类均已存在,金额、币种、发生时间均合法
- **THEN** 交易被记录,随后可以通过 `get_transaction` 按其 ID 查到,且所属账户的余额按交易类型(收入增加、支出减少)相应变化

#### Scenario: 引用不存在的账户或分类
- **WHEN** 调用 `create_transaction`,指定的账户 ID 或分类 ID 在当前账本中不存在
- **THEN** 请求被拒绝,返回说明引用无效的错误,不记录交易,账户余额不变

#### Scenario: 缺少必填字段
- **WHEN** 调用 `create_transaction`,缺少账户、分类、金额或发生时间中的任意一项
- **THEN** 请求被拒绝,不记录交易

### Requirement: 按 ID 查询单条交易
用户 SHALL 能够通过 `get_transaction` 工具,用交易 ID 查询该笔交易的完整信息。

#### Scenario: 查询已存在的交易
- **WHEN** 调用 `get_transaction`,提供一个已记录交易的 ID
- **THEN** 返回该交易的账户、分类、金额、币种、发生时间等完整信息

#### Scenario: 查询不存在的交易
- **WHEN** 调用 `get_transaction`,提供的 ID 不对应任何已记录的交易
- **THEN** 返回明确的"未找到"错误,而不是空结果或崩溃

### Requirement: 按条件搜索交易
用户 SHALL 能够通过 `search_transactions` 工具,按时间范围、账户、分类中的一个或多个条件筛选交易列表。

#### Scenario: 无筛选条件
- **WHEN** 调用 `search_transactions` 且不提供任何筛选条件
- **THEN** 返回账本中的全部交易

#### Scenario: 按时间范围筛选
- **WHEN** 调用 `search_transactions` 并指定起止时间
- **THEN** 只返回发生时间落在该区间内的交易

#### Scenario: 筛选结果为空
- **WHEN** 调用 `search_transactions` 提供的筛选条件没有匹配到任何交易
- **THEN** 返回一个空列表,而不是错误
