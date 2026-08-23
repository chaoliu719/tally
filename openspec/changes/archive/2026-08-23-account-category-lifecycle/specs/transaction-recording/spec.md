## MODIFIED Requirements

### Requirement: 记录一笔收入或支出交易
用户 SHALL 能够通过 `create_transaction` 工具记录一笔收入或支出类型的交易,指定所属账户、分类、金额、币种与发生时间。分类可以是账本中任意已存在的分类,不要求处于某个特定层级。

#### Scenario: 提供有效信息记录交易
- **WHEN** 调用 `create_transaction`,指定的账户已存在、指定的分类已存在,金额、币种、发生时间均合法
- **THEN** 交易被记录,随后可以通过 `get_transaction` 按其 ID 查到,且所属账户的余额按交易类型(收入增加、支出减少)相应变化

#### Scenario: 引用不存在的账户或分类
- **WHEN** 调用 `create_transaction`,指定的账户 ID 或分类 ID 在当前账本中不存在
- **THEN** 请求被拒绝,返回说明引用无效的错误,不记录交易,账户余额不变

#### Scenario: 引用一级分类
- **WHEN** 调用 `create_transaction`,指定的分类是一个没有父分类的顶层分类
- **THEN** 交易照常被记录——分类不再区分层级,曾经"必须使用二级分类"的限制不再适用

#### Scenario: 缺少必填字段
- **WHEN** 调用 `create_transaction`,缺少账户、分类、金额或发生时间中的任意一项
- **THEN** 请求被拒绝,不记录交易

## ADDED Requirements

### Requirement: 记录一笔余额调整交易
用户 SHALL 能够通过 `create_transaction` 工具,以 `type="balance_adjustment"`,记录一笔余额调整交易,作为修正账户余额的正式方式。这类交易指定所属账户、带符号的调整金额与发生时间,不指定分类。

#### Scenario: 提供有效信息记录余额调整
- **WHEN** 调用 `create_transaction`(`type="balance_adjustment"`),指定的账户已存在,`amount` 是一个非零值(正数表示增加余额,负数表示减少余额),发生时间合法,且未提供分类
- **THEN** 交易被记录,随后可以通过 `get_transaction` 查到,所属账户的余额按 `amount` 的符号与大小相应变化

#### Scenario: 携带分类
- **WHEN** 调用 `create_transaction`(`type="balance_adjustment"`)时提供了 `category_id`
- **THEN** 请求被拒绝,返回说明余额调整交易不能指定分类的错误,不记录交易,账户余额不变

#### Scenario: 金额为零
- **WHEN** 调用 `create_transaction`(`type="balance_adjustment"`)时 `amount` 为 0
- **THEN** 请求被拒绝,返回说明调整金额不能为零的错误,不记录交易

#### Scenario: 引用不存在的账户
- **WHEN** 调用 `create_transaction`(`type="balance_adjustment"`),指定的账户 ID 在当前账本中不存在
- **THEN** 请求被拒绝,返回说明引用无效的错误,不记录交易
