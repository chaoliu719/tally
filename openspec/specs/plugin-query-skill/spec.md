# plugin-query-skill Specification

## Purpose

定义 agent 通过 `search_transactions` / `get_transaction` / `list_ledgers` 为用户提供
只读查账(单笔明细、某商户历史、某段时间清单)的规范:检索方式、查询前用 `date` 把相对
时间窗口换算成 `YYYY-MM-DD HH:MM:SS` 本地日期时间字符串、查询后直接展示返回的日期时间
字符串(不做时区换算),以及返回值中金额符号、多币种、`comment` 语义的解读规则。不做汇总/对比/趋势(归
`plugin-analysis-skill`),不做增改(归 `plugin-record-skill`),不做图表。

## Requirements

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
- **THEN** agent 把该时间段换算成 `YYYY-MM-DD HH:MM:SS` 格式的本地日期时间窗口后调用 `search_transactions`,按时间逐笔呈现清单,不做合计汇总

#### Scenario: 用户要的是合计而非明细
- **WHEN** 用户要的是某段时间的收支合计、环比或占比
- **THEN** agent 转用 `plugin-analysis-skill` 定义的分析角度,而不是在本 skill 内自行累加明细

### Requirement: 查询前把时间窗口用 shell date 换算成本地日期时间字符串
Agent SHALL 在传 `start_time`/`end_time` 前,用 shell 的 `date` 命令把「本月/昨天/
最近 N 天」等表达换算成 `YYYY-MM-DD HH:MM:SS` 格式的日期时间字符串,直接取命令输出
格式化后的结果,不得心算或凭印象填写。当前日期 SHALL 以宿主(Claude Code / claude.ai)
系统提示词自带的当前日期上下文为锚点;缺少该上下文时,agent SHALL 先运行 `date` 取
当前时刻,再据此计算。

#### Scenario: 用户用自然语言给出查询时间范围
- **WHEN** 用户用「这个月」「昨天」「上周」这类相对表达界定查询范围
- **THEN** agent 以宿主注入的当前日期为锚点,用 `date` 计算出该范围的起止时刻,格式化为 `YYYY-MM-DD HH:MM:SS` 字符串,以此作为 `start_time`/`end_time`,不凭印象估算边界

### Requirement: 查询后直接展示返回的日期时间字符串
Agent SHALL 将 `search_transactions` / `get_transaction` 返回的 `time` 字段(`YYYY-MM-DD
HH:MM:SS` 格式的本地日期时间字符串)直接呈现给用户,不做任何时区换算,也不假设它代表
查看者当前所在时区的时刻——它就是记账当时的墙上钟时刻。

#### Scenario: 呈现查询结果里的交易时间
- **WHEN** agent 要把查询返回的交易时间展示给用户
- **THEN** agent 直接展示该 `time` 字符串所表示的日期与时间(可补充星期等易读信息),不做时区换算、不心算偏移

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

### Requirement: 用时间线面板浏览交易时先划定范围
当用户想翻看、浏览某段交易(而不是让 Agent 念出一份清单)时,Agent SHALL 用 `open_transaction_timeline` 打开时间线面板,并 SHALL 在打开时用 `start_date` / `end_date` / `category_id` / `source_id` 划定范围,只加载用户要看的部分。用户没有指明时间范围时,Agent SHALL 默认传最近 30 天(`start_date` 为今天往前数 30 天),并在回答里告诉用户所用范围。该默认只存在于本 skill,工具本身不施加任何默认范围。

#### Scenario: 用户想翻看本月的账
- **WHEN** 用户说"打开这个月的账""我想翻翻 9 月的交易"
- **THEN** agent 把该时间段换算成 `YYYY-MM-DD` 的 `start_date` / `end_date` 后调用 `open_transaction_timeline`,不打开未限定范围的全部历史

#### Scenario: 用户没有指明时间范围
- **WHEN** 用户只说"看看我的账""打开时间线",没有给出时间范围
- **THEN** agent 以今天往前数 30 天作为 `start_date` 调用工具,并在回答里说明"当前显示最近 30 天",不无范围地打开全部历史

#### Scenario: 用户明确要全部历史
- **WHEN** 用户明确要求看全部历史
- **THEN** agent 不传时间范围参数,并告知用户这会加载整个账本

#### Scenario: 用户要某个分类或来源的账
- **WHEN** 用户要看某个分类(如"餐饮")或某个来源的账
- **THEN** agent 先用 `list_categories` / `list_sources` 查到对应 id 再传 `category_id` / `source_id`,不凭名称猜 id;同时按本节规则确定时间范围

#### Scenario: 用户想看范围之外的数据
- **WHEN** 用户看完面板后想看更早或另一段时间的数据
- **THEN** agent 用新的范围重新调用 `open_transaction_timeline`,而不是让用户在面板里自行查找,因为面板只包含打开时划定的范围

### Requirement: 浏览面板的日期用 shell date 换算
Agent SHALL 在传 `start_date` / `end_date` 前,与查账时间窗口的换算纪律一致,以宿主注入的当前日期为锚点、用 shell 的 `date` 命令算出 `YYYY-MM-DD` 格式的日期并直接取输出,不得心算或凭印象填写;日期不带时分秒,不做时区换算。

#### Scenario: 换算最近 30 天
- **WHEN** agent 需要以最近 30 天为范围打开时间线
- **THEN** agent 用 `date` 取得今天往前数 30 天的 `YYYY-MM-DD` 作为 `start_date`,`end_date` 可以不传
