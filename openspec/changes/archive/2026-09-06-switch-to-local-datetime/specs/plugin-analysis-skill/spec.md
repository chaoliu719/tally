## MODIFIED Requirements

### Requirement: 月度环比对比
Agent SHALL 通过分别调用两次 `get_financial_summary`(各自指定对应月份的起止本地日期时间字符串)来回答月度环比问题,自行计算月份边界并格式化为 `YYYY-MM-DD HH:MM:SS`。

#### Scenario: 用户要求本月与上月对比
- **WHEN** 用户要求比较本月与上月的收支
- **THEN** agent 以本月与上月各自的起止本地日期时间字符串分别调用一次 `get_financial_summary`,对比两次返回的 `income`/`expense`/`net` 与分类小计;服务端不提供"本月"语义,月份边界由 agent 自行计算
