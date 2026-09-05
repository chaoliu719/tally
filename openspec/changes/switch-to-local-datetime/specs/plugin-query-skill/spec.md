## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: 查询前把时间窗口用 shell date 换算成 Unix 秒
**Reason**:`time`/`start_time`/`end_time` 不再是无时区语义的 Unix 秒数,而是直接以本地日期时间字符串表示,不需要再换算成秒数。
**Migration**:改用新增的「查询前把时间窗口用 shell date 换算成本地日期时间字符串」。

### Requirement: 查询后把返回的秒数字段用 shell date 反查再展示
**Reason**:返回值本身已是人类可读的本地日期时间字符串,不再是需要反查换算的裸 Unix 秒数。
**Migration**:改用新增的「查询后直接展示返回的日期时间字符串」。

## ADDED Requirements

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
