## Why

`plugin/skills/record/` 目前只覆盖"记一笔/改一笔"。三个缺口在最近的设计讨论里被确认为
值得补上:(1) 先例检索/已有分类都没有合适选项时,record 没有教怎么顺手建一个新分类,
逼得 agent 要么瞎选一个分类、要么打断对话去问;(2) 用户想要用已有的 `search_transactions`
和 `get_financial_summary` 做消费分析(同类交易比价、月环比、类别对比、多月/多年趋势),
但没有一份 skill 教这些查询该怎么拼、结果该怎么解读——`get_financial_summary` 的
`income`/`expense`/`net` 符号语义不同、多币种不做换算,读错了服务端不会报错,这块信息差
目前没人补;(3) 用户记账一段时间后,分类体系会出现该拆分(新爱好冒出来了)或该合并
(不同时间建的相近分类)的情况,但没有 skill 教这套判断与安全执行的方式。

## What Changes

- 扩展 `plugin/skills/record/`:先例检索/现有分类都没有合适选项时,在预览里向用户提议
  新建一个分类(`manage_category` create),确认后再据此记录——这是非破坏性操作,不需要
  两步确认。
- 新增 `plugin/skills/analysis/`:教如何用 `search_transactions`(同类交易比价)和
  `get_financial_summary`(月环比、类别对比、多月/多年趋势)拼出几种具体的分析角度,
  以及如何正确解读返回值(`income`/`expense`/`net` 符号语义不同、多币种分别返回不做
  换算、分类/来源拆分只列出有交易的那些)。不做图表/可视化——那部分仍按现有决定延后。
- 新增 `plugin/skills/optimize/`:教如何检测分类该拆分(某个较宽分类下最近的交易明显
  聚成一个共同主题)或该合并(不同时间建的、名字/用途接近的分类)的信号,以及安全执行
  拆分/合并的流程——合并类操作要先把受影响交易数量和示例摆给用户看、确认后再逐笔
  `update_transaction` 重新分类、最后对空分类走 `manage_category` 的 preview → apply
  删除。同时教 agent 如何在当前会话的工具箱里"顺手"发现并使用宿主(harness)已经暴露的
  定时能力去提议周期性复盘,而不是在 skill 里硬编码任何具体的定时 API 或工具名。

## Capabilities

### New Capabilities
- `plugin-record-skill`: `plugin/skills/record/` 当前已实现的记录/修正交易流程
  (先例检索、对话预览、comment 纪律)首次补齐规格,并加入"无合适分类时顺手新建"的
  新行为。
- `plugin-analysis-skill`: `plugin/skills/analysis/` 提供的消费分析查询与解读规范。
- `plugin-optimize-skill`: `plugin/skills/optimize/` 提供的分类拆分/合并检测与安全
  执行流程,以及周期性复盘的定时建议方式。

### Modified Capabilities
(无——tally-mcp 事实层不变,仅新增/扩展 plugin 实践层的 skill 内容)

## Impact

- 新增文件:`plugin/skills/analysis/SKILL.md`、`plugin/skills/optimize/SKILL.md`。
- 修改文件:`plugin/skills/record/SKILL.md`(加入顺手建分类的流程)。
- 不涉及 `mcp/` 下任何代码或 spec 变更;不新增 MCP 工具。
- `optimize` skill 依赖的分类合并操作受限于没有批量更新工具——单个分类下交易数量大时
  需要多次 `update_transaction` 调用,skill 需要在执行前把这个成本告知用户。
