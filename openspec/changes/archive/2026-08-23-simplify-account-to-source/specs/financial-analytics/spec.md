## ADDED Requirements

### Requirement: 按来源拆分收支
`get_financial_summary` 的返回结果 SHALL 包含按来源拆分的收入/支出小计:对统计范围内每个发生过 `income` 或 `expense` 交易的来源,给出该来源上的收入小计与支出小计。

#### Scenario: 交易分布在多个来源
- **WHEN** 统计范围内的交易分布在多个不同来源下
- **THEN** 返回结果为每个发生过交易的来源给出对应的收入/支出小计,未发生交易的来源不出现在结果中

## MODIFIED Requirements

### Requirement: 按时间范围统计财务汇总
用户 SHALL 能够通过 `get_financial_summary` 工具,指定一个可选的时间范围(`start_time`/`end_time`,unix 秒,语义与 `search_transactions` 一致),获得该范围内 `income`/`expense` 类型交易的总收入、总支出、净额(总收入减总支出),以及按分类、按来源拆分的收入/支出小计。金额按币种分组返回,不做任何汇率换算。

#### Scenario: 提供时间范围统计
- **WHEN** 调用 `get_financial_summary` 并指定 `start_time`/`end_time`
- **THEN** 返回发生时间落在该区间内的 `income`/`expense` 交易按币种分组的总收入、总支出、净额

#### Scenario: 不提供时间范围
- **WHEN** 调用 `get_financial_summary` 且不提供 `start_time`/`end_time`
- **THEN** 统计账本中全部历史 `income`/`expense` 交易,不做时间过滤

#### Scenario: 范围内没有匹配的交易
- **WHEN** 调用 `get_financial_summary`,指定的时间范围内没有任何 `income`/`expense` 交易
- **THEN** 返回总收入、总支出、净额均为空(不返回任何币种分组),而不是错误

#### Scenario: 多币种账本分别汇总
- **WHEN** 统计范围内存在多笔不同币种的 `income`/`expense` 交易
- **THEN** 返回结果按币种分别给出总收入、总支出、净额,不合并换算成单一数值

## REMOVED Requirements

### Requirement: 按账户拆分收支
**Reason**: "账户"概念被"来源"取代,见本能力新增的"按来源拆分收支"。
**Migration**: 读取 `get_financial_summary` 返回结果中的来源拆分字段替代原来的账户拆分字段,维度语义不变(仍按币种分组、不做汇率换算)。

### Requirement: 单独统计余额调整总额
**Reason**: `adjustment` 交易类型被移除,不再有"余额调整"这类交易需要单独统计。
**Migration**: 无替代——`get_financial_summary` 的输出中不再包含余额调整总额字段。
