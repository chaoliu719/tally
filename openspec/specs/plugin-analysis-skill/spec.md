# plugin-analysis-skill Specification

## Purpose

定义 agent 使用 `search_transactions` 与 `get_financial_summary` 为用户提供消费分析的标准查询角度,以及正确解读返回值的规则,避免因符号/精度/分组语义误读产生错误结论。不涉及图表或可视化输出。

## Requirements

### Requirement: 同类交易价格对比
Agent SHALL 使用 `search_transactions` 的 `keyword` 检索同商户/同关键词的历史交易,按时间对比其 `amount` 走势来回答价格变化类问题。

#### Scenario: 用户询问某商户价格是否变化
- **WHEN** 用户想知道某个商户/关键词相关交易的价格是否随时间变化
- **THEN** agent 用 `search_transactions(keyword=<关键词>)` 取出该账本内匹配的交易并按时间排序,对比 `amount` 走势作答

### Requirement: 月度环比对比
Agent SHALL 通过分别调用两次 `get_financial_summary`(各自指定对应月份的起止时间)来回答月度环比问题,自行计算月份边界的 Unix 秒数。

#### Scenario: 用户要求本月与上月对比
- **WHEN** 用户要求比较本月与上月的收支
- **THEN** agent 以本月与上月各自的起止 Unix 秒数分别调用一次 `get_financial_summary`,对比两次返回的 `income`/`expense`/`net` 与分类小计;服务端不提供"本月"语义,月份边界由 agent 自行计算

### Requirement: 类别拆分对比
Agent SHALL 使用单次 `get_financial_summary` 调用返回的按分类拆分结果回答"哪类花销最多"这类问题,不需要为每个分类单独查询。

#### Scenario: 用户询问某段时间内哪类花销最多
- **WHEN** 用户想知道某段时间内各分类的收支占比
- **THEN** agent 使用单次 `get_financial_summary` 调用返回的按分类拆分结果作答

### Requirement: 多月/多年趋势对比
Agent SHALL 按所需粒度多次调用 `get_financial_summary` 来回答趋势类问题,并以文字或表格呈现,不生成图表。

#### Scenario: 用户要求连续几个月的变化趋势
- **WHEN** 用户要求查看连续多个月或多年的收支趋势
- **THEN** agent 按月或按年粒度分别多次调用 `get_financial_summary`,将结果整理为趋势说明或表格,不产出图表/可视化内容

### Requirement: 正确解读金额符号语义
Agent SHALL 将 `get_financial_summary` 返回的 `net` 字段按其可能带负号的语义解读,不假设它与 `income`/`expense` 一样恒为非负。

#### Scenario: 净支出场景
- **WHEN** 展示 `get_financial_summary` 返回的 `net` 字段
- **THEN** agent 按 `net` 可能带负号(净支出)的语义解读,不假设它恒为非负

### Requirement: 多币种结果不做换算
Agent SHALL 将 `get_financial_summary` 按币种分组返回的多个结果分别呈现,不将不同币种的金额相加或换算合并。

#### Scenario: 账本存在多币种交易
- **WHEN** `get_financial_summary` 按币种分组返回多个结果
- **THEN** agent 分别呈现各币种的汇总,不将不同币种的金额直接相加或换算合并

### Requirement: 分类/来源拆分仅包含有交易的项
Agent SHALL 将 `get_financial_summary` 按分类/按来源拆分结果中未出现的分类/来源理解为该范围内无收支,而非数据缺失或查询出错。

#### Scenario: 某分类在统计范围内无交易
- **WHEN** 解读 `get_financial_summary` 按分类或按来源的拆分结果
- **THEN** agent 将结果中未出现的分类/来源理解为"该范围内无收支",不当作数据缺失或查询出错来处理
