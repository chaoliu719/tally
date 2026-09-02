## ADDED Requirements

### Requirement: 多账本时先确认账本
Agent SHALL 在用户名下存在多个账本时,于查询前用 `list_ledgers` 把账本列表呈现给用户
选择,不擅自默认第一个账本;单账本时可直接查询。

#### Scenario: 用户有多个账本且未指明查哪个
- **WHEN** 用户提出查账请求但未指明账本,且 `list_ledgers` 返回多个账本
- **THEN** agent 先把账本列表摆给用户确认要查哪个,再据此传 `ledger_id` 发起查询

### Requirement: 检索某商户或关键词的历史交易
Agent SHALL 使用 `search_transactions(ledger_id, keyword=<关键词>)` 按 `comment` 子串
匹配检索历史交易;`keyword` 大小写不敏感且不支持通配符,`%`/`_` 按字面字符匹配。需要
某一笔的完整字段时 Agent SHALL 调用 `get_transaction(id)` 取全量,不只依赖搜索结果摘要。

#### Scenario: 用户问某商户/某笔的历史或明细
- **WHEN** 用户想查看某商户或某关键词相关的历史交易,或某一笔的完整信息
- **THEN** agent 用 `search_transactions` 按关键词检索并按时间排序呈现;需要单笔完整字段时用 `get_transaction(id)` 取全量

#### Scenario: 关键词含通配符字符
- **WHEN** agent 为放宽匹配而想在 `keyword` 里加入 `%` 或 `_`
- **THEN** agent 不加通配符,改用更短或更核心的词放宽、用更多字收窄,因为这些符号会被当作字面字符匹配

### Requirement: 检索一段时间的交易清单
Agent SHALL 在用户要求某段时间的交易明细时,用 `search_transactions` 传时间窗口
(`start_time`/`end_time`)取出该范围的交易并按时间逐笔列出;这是「列明细」,当用户
要的是合计/对比时 Agent SHALL 转由 `plugin-analysis-skill` 的分析角度处理。

#### Scenario: 用户要一段时间的开销明细
- **WHEN** 用户要求查看某段时间(如「这个月」「昨天」「上周」)的交易明细
- **THEN** agent 把该时间段换算成 Unix 秒窗口后调用 `search_transactions`,按时间逐笔呈现清单,不做合计汇总

#### Scenario: 用户要的是合计而非明细
- **WHEN** 用户要的是某段时间的收支合计、环比或占比
- **THEN** agent 转用 `plugin-analysis-skill` 定义的分析角度,而不是在本 skill 内自行累加明细

### Requirement: 查询前把时间窗口用 shell date 换算成 Unix 秒
Agent SHALL 在传 `start_time`/`end_time` 前,用 shell 的 `date` 命令把「本月/昨天/
最近 N 天」等表达按用户本地时区的自然边界换算成 Unix 秒,直接取命令输出,不得心算或
凭印象填写数字。本地时区 SHALL 先用 `date` 确认(`plugin-time-context-hook` 注入的
锚点亦可)。

#### Scenario: 用户用自然语言给出查询时间范围
- **WHEN** 用户用「这个月」「昨天」「上周」这类相对表达界定查询范围
- **THEN** agent 用 `date` 计算出该范围在用户本地时区下的起止时刻并转为 Unix 秒,以此作为 `start_time`/`end_time`,不凭印象估算边界

### Requirement: 查询后把返回的秒数字段用 shell date 反查再展示
Agent SHALL 把 `search_transactions` / `get_transaction` 返回的 `time` 及任何 Unix 秒
字段,用 `date -r <ts>`(或 `date -d @<ts>`)换算成用户本地时区的人类可读时间后再呈现
给用户;Agent SHALL NOT 直接把裸秒数展示给用户,也 SHALL NOT 自行脑补时区偏移做心算。

#### Scenario: 呈现查询结果里的交易时间
- **WHEN** agent 要把查询返回的交易时间展示给用户
- **THEN** agent 用 `date` 将该 Unix 秒换算到用户本地时区再展示,不呈现裸秒数、不心算偏移

### Requirement: 正确解读返回值的金额、币种与 comment 语义
Agent SHALL 按以下语义解读查询返回值:`amount` 的符号与收支方向按 tally 既有数据模型
理解,不默认为支出;多币种结果分币种分别呈现,不按汇率相加或合并;`comment` 是用户
原始凭据文本、结构化字段是归类解释,回答「这笔是什么」时先呈现 `comment` 原文。

#### Scenario: 结果含多种币种
- **WHEN** 一段时间的交易清单里存在多种币种
- **THEN** agent 分币种分别列出,不将不同币种金额相加或换算合并

#### Scenario: 用户问某笔交易是什么
- **WHEN** 用户询问某笔交易的性质
- **THEN** agent 先呈现 `comment` 原文,再补充分类/来源等结构化字段,不只用分类名作答

### Requirement: 明确 query 与其余 skill 的边界
Agent SHALL 将本 skill 的职责限定为只读查账;涉及合计/对比/趋势转 `plugin-analysis-skill`,
涉及记一笔/改一笔/拆多笔转 `plugin-record-skill`,涉及分类拆分/合并转 `plugin-optimize-skill`,
图表/可视化按项目现有决定延后不做。

#### Scenario: 请求超出只读查账范围
- **WHEN** 用户的请求实际是分析、记账、调整分类结构或生成图表
- **THEN** agent 按对应 skill 的规范处理,而不是在本 skill 内越界执行
