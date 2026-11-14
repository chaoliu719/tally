# financial-analytics Specification

## Purpose

让用户能够通过 MCP 工具直接拿到一段时间内收入、支出、净额的聚合统计,按分类和来源拆分,不需要 Agent 自己把原始交易明细拉进上下文手动求和。

## Requirements

### Requirement: 按时间范围统计财务汇总
用户 SHALL 能够通过 `get_financial_summary` 工具,指定 `ledger_id` 与一个可选的时间范围(`start_time`/`end_time`,本地日期时间字符串,格式 `YYYY-MM-DD HH:MM:SS`,不带时区,语义与 `search_transactions` 一致),获得该账本内该范围内 `income`/`expense` 类型交易的总收入、总支出、净额(总收入减总支出),以及按分类、按来源、按预算拆分的收入/支出小计。金额按币种分组返回,不做任何汇率换算,不做跨账本汇总。`income`/`expense`/`net` 均以十进制字符串表示该分组 `currency` 主单位下的金额,精度由该币种在 ISO 4217 下的标准精度决定(与 `create_transaction` 的 `amount` 字段同一套精度规则);`net` 为负(净支出)时字符串带负号(如 `"-50.00"`),`income`/`expense` 恒为非负。

#### Scenario: 提供时间范围统计
- **WHEN** 调用 `get_financial_summary`,指定一个已存在的账本并指定 `start_time`/`end_time`
- **THEN** 返回该账本内发生时间落在该区间内的 `income`/`expense` 交易按币种分组的总收入、总支出、净额,不包含其他账本的交易

#### Scenario: 不提供时间范围
- **WHEN** 调用 `get_financial_summary`,指定一个已存在的账本且不提供 `start_time`/`end_time`
- **THEN** 统计该账本中全部历史 `income`/`expense` 交易,不做时间过滤,不包含其他账本的交易

#### Scenario: 范围内没有匹配的交易
- **WHEN** 调用 `get_financial_summary`,指定的账本在给定时间范围内没有任何 `income`/`expense` 交易
- **THEN** 返回总收入、总支出、净额均为空(不返回任何币种分组),而不是错误

#### Scenario: 多币种账本分别汇总
- **WHEN** 统计范围内存在多笔不同币种的 `income`/`expense` 交易
- **THEN** 返回结果按币种分别给出总收入、总支出、净额,不合并换算成单一数值

#### Scenario: 净支出场景净额带负号
- **WHEN** 统计范围内某币种的总支出大于总收入
- **THEN** 该币种分组的 `net` 是一个带负号的十进制字符串(如总收入 `"30.00"`、总支出 `"50.00"` 时 `net` 为 `"-20.00"`),`income`/`expense` 本身仍是非负字符串

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `get_financial_summary`,指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回说明账本不存在的错误

#### Scenario: start_time 或 end_time 格式非法
- **WHEN** 调用 `get_financial_summary` 提供的 `start_time` 或 `end_time` 不符合 `YYYY-MM-DD HH:MM:SS` 格式
- **THEN** 请求被拒绝,返回说明时间格式非法的错误

### Requirement: 按分类拆分收支
`get_financial_summary` 的返回结果 SHALL 包含按分类拆分的收入/支出小计:对指定账本统计范围内每个出现过 `income` 或 `expense` 交易的分类,给出该分类下的收入小计与支出小计(同样按币种分组),不包含其他账本的分类。`income`/`expense` 小计均以十进制字符串表示,精度规则与总收入/总支出字段一致。

#### Scenario: 交易分布在多个分类
- **WHEN** 统计范围内的交易分布在多个不同分类下
- **THEN** 返回结果为每个出现过交易的分类给出对应的收入/支出小计,未出现交易的分类不出现在结果中

### Requirement: 按来源拆分收支
`get_financial_summary` 的返回结果 SHALL 包含按来源拆分的收入/支出小计:对指定账本统计范围内每个发生过 `income` 或 `expense` 交易的来源,给出该来源上的收入小计与支出小计,不包含其他账本的来源。`income`/`expense` 小计均以十进制字符串表示,精度规则与总收入/总支出字段一致。

#### Scenario: 交易分布在多个来源
- **WHEN** 统计范围内的交易分布在多个不同来源下
- **THEN** 返回结果为每个发生过交易的来源给出对应的收入/支出小计,未发生交易的来源不出现在结果中

### Requirement: 按预算拆分收支
`get_financial_summary` 的返回结果 SHALL 包含按预算拆分的收入/支出小计:对指定账本统计范围内每个出现过 `income` 或 `expense` 交易的预算,给出该预算下的收入小计与支出小计(同样按币种分组),不包含其他账本的预算。`income`/`expense` 小计均以十进制字符串表示,精度规则与总收入/总支出字段一致。尚未分配预算的交易 SHALL 单独成组返回,而不是被并入某个预算或被省略。

#### Scenario: 交易分布在多个预算
- **WHEN** 统计范围内的交易分布在多个不同预算下
- **THEN** 返回结果为每个出现过交易的预算给出对应的收入/支出小计,未出现交易的预算不出现在结果中

#### Scenario: 存在尚未分配预算的交易
- **WHEN** 统计范围内存在一笔或多笔还没有被分配预算的交易
- **THEN** 这些交易的收入/支出小计单独成组返回,标识为尚未分配预算,不并入任何已有预算

#### Scenario: 各预算支出之和等于总支出
- **WHEN** 统计范围内某币种的交易分布在若干预算下,其中部分尚未分配预算
- **THEN** 该币种下各预算支出小计与尚未分配预算组的支出小计之和,等于该币种的总支出

### Requirement: 按预算筛选汇总
`get_financial_summary` SHALL 接受一个可选的 `budget_id` 入参,提供时只统计该预算下的交易;它 SHALL 允许取表示"尚未分配预算"的值。提供 `budget_id` 时,返回的总额与各项拆分都只基于该预算下的交易。

#### Scenario: 只统计某一个预算
- **WHEN** 调用 `get_financial_summary` 并提供一个属于该账本的 `budget_id`
- **THEN** 返回的总收入、总支出、净额以及各项拆分,都只计入预算为该值的交易

#### Scenario: 筛选一个不存在的预算
- **WHEN** 调用 `get_financial_summary`,提供的 `budget_id` 在该账本中不存在,且不是表示"尚未分配预算"的值
- **THEN** 请求被拒绝,返回说明预算不存在的错误
