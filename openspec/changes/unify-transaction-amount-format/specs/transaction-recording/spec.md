## MODIFIED Requirements

### Requirement: 记录一笔收入或支出交易
用户 SHALL 能够通过 `create_transaction` 工具记录一笔收入或支出类型的交易,指定所属账本(`ledger_id`)、来源、分类、金额、币种与发生时间。来源与分类都必须属于指定的账本;分类可以是该账本中任意已存在的分类,不要求处于某个特定层级。`amount` 以十进制字符串提供,表示该 `currency` 主单位下的金额(例如 CNY 传 `"50.00"`,JPY 传 `"5000"`,BHD 传 `"5.000"`——精度因币种而异,不固定为两位小数);该字符串的小数位数不得超过该币种在 ISO 4217 下的标准精度,且解析后的数值必须为正。

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

#### Scenario: 金额格式非法
- **WHEN** 调用 `create_transaction`,提供的 `amount` 不是合法的十进制数字字符串(如包含非数字字符、空字符串、多个小数点)
- **THEN** 请求被拒绝,返回说明金额格式非法的错误,不记录交易

#### Scenario: 金额小数位数超出币种精度
- **WHEN** 调用 `create_transaction`,提供的 `amount` 小数位数超过该 `currency` 在 ISO 4217 下的标准精度(例如给 CNY 传 `"50.001"`,给 JPY 传 `"50.5"`)
- **THEN** 请求被拒绝,返回说明精度超出该币种允许范围的错误,不记录交易

#### Scenario: 金额为零或负值
- **WHEN** 调用 `create_transaction`,提供的 `amount` 解析后的数值为零或负数(如 `"0.00"`、`"-10.00"`)
- **THEN** 请求被拒绝,返回说明金额必须为正的错误,不记录交易

### Requirement: 按 ID 查询单条交易
用户 SHALL 能够通过 `get_transaction` 工具,提供交易 ID 与其所属 `ledger_id`,查询该笔交易的完整信息。

#### Scenario: 查询已存在的交易
- **WHEN** 调用 `get_transaction`,提供一个已记录交易的 ID 与其实际所属的 `ledger_id`
- **THEN** 返回该交易的来源、分类、金额(以该笔交易 `currency` 主单位的十进制字符串表示,精度由该币种决定)、币种、发生时间等完整信息

#### Scenario: 查询不存在的交易
- **WHEN** 调用 `get_transaction`,提供的 ID 不对应任何已记录的交易
- **THEN** 返回明确的"未找到"错误,而不是空结果或崩溃

#### Scenario: ledger_id 与交易实际所属账本不一致
- **WHEN** 调用 `get_transaction`,提供的 ID 对应一笔已存在的交易,但提供的 `ledger_id` 不是它实际所属的账本
- **THEN** 返回"未找到"错误,而不是该交易的信息

### Requirement: 更新一笔交易
用户 SHALL 能够通过 `update_transaction` 工具,用交易 ID 与其所属 `ledger_id`,替换一笔已存在交易的全部可变字段(`type`/`source_id`/`category_id`/`amount`/`currency`/`time`/`comment`)。这是完整字段替换语义,不支持只修改其中一部分字段;字段校验规则与 `create_transaction` 完全一致(income/expense 需要合法 `category_id`;`amount` 以十进制字符串提供,解析后必须为正,且小数位数不超过该 `currency` 的标准精度;`currency` 必须是系统支持的货币;`source_id`/`category_id` 必须属于该交易所在的账本)。更新不允许把交易挪到另一个账本——`source_id` 只能引用同一账本内的来源。更新不需要二次确认,直接执行。

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
- **WHEN** 调用 `update_transaction`,`type` 为 `income` 或 `expense`,但 `category_id` 缺失、或 `amount` 不是合法的十进制数字字符串、或其小数位数超出该 `currency` 的标准精度、或解析后的数值为零或负数
- **THEN** 请求被拒绝,交易不发生变化

#### Scenario: 更新不存在的交易
- **WHEN** 调用 `update_transaction`,提供的 `id` 不对应任何已记录的交易
- **THEN** 返回明确的"未找到"错误,不发生任何变化

#### Scenario: ledger_id 与交易实际所属账本不一致
- **WHEN** 调用 `update_transaction`,提供的 `id` 对应一笔已存在的交易,但提供的 `ledger_id` 不是它实际所属的账本
- **THEN** 返回"未找到"错误,不发生任何变化
