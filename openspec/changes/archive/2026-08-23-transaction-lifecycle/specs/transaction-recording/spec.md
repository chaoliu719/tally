## ADDED Requirements

### Requirement: 更新一笔交易
用户 SHALL 能够通过 `update_transaction` 工具,用交易 ID 替换一笔已存在交易的全部可变字段(`type`/`account_id`/`category_id`/`amount`/`time`/`comment`)。这是完整字段替换语义,不支持只修改其中一部分字段;字段校验规则与 `create_transaction` 完全一致(income/expense 需要合法 `category_id` 且 `amount` 为正;`balance_adjustment` 不能带 `category_id` 且 `amount` 非零)。更新不需要二次确认,直接执行。

#### Scenario: 提供完整合法字段更新交易
- **WHEN** 调用 `update_transaction`,提供一个已存在交易的 ID,以及完整、合法的 `type`/`account_id`/`category_id`/`amount`/`time`/`comment`(账户与分类均存在,金额/分类规则与指定的 `type` 相符)
- **THEN** 该交易被替换为新值,随后 `get_transaction` 返回更新后的信息;交易原本所属账户与更新后所属账户(如果 `account_id` 发生变化)的余额都相应变化

#### Scenario: 缺少必填字段
- **WHEN** 调用 `update_transaction`,缺少 `id`、`type`、`account_id`、`amount`、`time` 中的任意一项
- **THEN** 请求被拒绝,交易不发生变化

#### Scenario: 引用不存在的账户或分类
- **WHEN** 调用 `update_transaction`,指定的 `account_id` 或 `category_id` 在当前账本中不存在
- **THEN** 请求被拒绝,返回说明引用无效的错误,交易不发生变化

#### Scenario: balance_adjustment 携带分类或金额为零
- **WHEN** 调用 `update_transaction`,`type="balance_adjustment"` 但提供了 `category_id`,或 `amount` 为 0
- **THEN** 请求被拒绝,交易不发生变化

#### Scenario: income/expense 金额非正或缺少分类
- **WHEN** 调用 `update_transaction`,`type` 为 `income` 或 `expense`,但 `category_id` 缺失、或 `amount` 不是正数
- **THEN** 请求被拒绝,交易不发生变化

#### Scenario: 更新不存在的交易
- **WHEN** 调用 `update_transaction`,提供的 `id` 不对应任何已记录的交易
- **THEN** 返回明确的"未找到"错误,不发生任何变化

### Requirement: 删除一笔交易
用户 SHALL 能够通过 `delete_transaction` 工具,以 preview → apply 两步确认(复用 `write-confirmation` 机制)删除一笔已存在的交易记录。与账户、分类删除不同,删除交易没有"是否被其他数据引用"这类前置门槛——任意已存在的交易都允许被删除。

#### Scenario: 预览删除一笔存在的交易
- **WHEN** 调用 `delete_transaction`,不携带 `confirmation_token`,提供的 `id` 对应一笔已存在的交易
- **THEN** 返回该交易的完整信息、一个 `confirmation_token`、该令牌的过期时间,以及 `pending_confirmation` 状态,不产生任何数据变化

#### Scenario: 确认删除
- **WHEN** 调用 `delete_transaction`,携带上一步预览返回的、仍然有效的 `confirmation_token`
- **THEN** 该交易被删除;随后 `get_transaction` 返回"未找到",`search_transactions` 不再包含它,其所属账户的余额相应变化

#### Scenario: 预览或确认一笔不存在的交易
- **WHEN** 调用 `delete_transaction`(带或不带 `confirmation_token`),提供的 `id` 不对应任何已记录的交易
- **THEN** 请求被拒绝,返回明确的"未找到"错误,不签发或消费 `confirmation_token`

#### Scenario: 预览之后交易已被并发修改或删除
- **WHEN** 携带一个此前签发的 `confirmation_token` 调用 `delete_transaction`,但该交易自签发以来已被 `update_transaction` 修改、或已被删除
- **THEN** 请求被拒绝,返回说明状态已变化、需要重新预览的错误,不执行删除

#### Scenario: 清空账户下全部交易后可以删除该账户
- **WHEN** 一个账户名下的全部交易(含 `balance_adjustment`)都已通过 `delete_transaction` 成功删除
- **THEN** 对该账户调用 `manage_account`(`operation="delete"`)可以正常完成 preview → apply 流程并成功删除该账户

#### Scenario: 清空分类下全部交易后可以删除该分类
- **WHEN** 一个分类下的全部交易都已通过 `delete_transaction` 成功删除,且该分类没有子分类
- **THEN** 对该分类调用 `manage_category`(`operation="delete"`)可以正常完成 preview → apply 流程并成功删除该分类
