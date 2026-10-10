## ADDED Requirements

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
