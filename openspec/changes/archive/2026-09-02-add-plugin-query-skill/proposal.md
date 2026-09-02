## Why

`plugin/skills/` 目前三个 skill 的触发面之间有一道缝:`record` 只管「记一笔/改一笔」,
`analysis` 收窄在「合计/对比/趋势」,`optimize` 管分类结构。用户最高频的一类意图——
**把已有交易原样查出来看**(「上次在星巴克花了多少」「昨天买了啥」「把这个月的账
列出来」「这笔备注写的啥」)——不属于其中任何一个,实际运行里只会勉强蹭到 `analysis`,
于是要么不触发 skill、要么被按「分析」处理。

后果集中在时间上:纯查询没有任何 skill 约束 agent 走 shell `date` 换算,agent 常常
(1) 凭印象心算时间窗口边界,漏掉或多算跨天交易;(2) 把返回的 Unix 秒数直接念给用户,
或自己脑补一个时区偏移去心算,导致清单里每一笔的时间都是错的。`plugin-time-context-hook`
只提供「今天」锚点,不覆盖「查询返回值的秒数如何展示」。

## What Changes

- **新增 `plugin/skills/query/`(第四个实践 skill)**
  - 触发面覆盖「查某一笔/某商户历史」与「查一段时间清单」两类只读意图。
  - `search_transactions` 的 `keyword` 子串语义、不支持通配符,与 `record` 一致;
    看单笔完整字段走 `get_transaction`。
  - **时间换算纪律(本 skill 的重点):查询前把时间窗口用 shell `date` 算成 Unix 秒
    再传 `start_time`/`end_time`;查询后把返回的每个秒数字段用 `date -r`/`date -d @`
    反查成用户本地时区再展示。全程禁止心算、禁止脑补时区偏移。**
  - 返回值解读三条:`amount` 符号/方向按既有数据模型理解不默认支出;多币种分别呈现
    不换算合并;`comment` 是原文、结构化字段是解释。
  - 明确边界:合计/对比/趋势 → `analysis`;记一笔/改一笔 → `record`;分类拆并 →
    `optimize`;图表 → 按现有决定延后。
- **收窄 `plugin/skills/analysis/SKILL.md` 的 `description`**,把「原样列明细」明确指向
  `query`,与本 skill 划清触发边界。这是 skill 触发描述的调整,`plugin-analysis-skill`
  spec 的 Requirement 不变(spec 不覆盖 `description` 文案)。

## Capabilities

### New Capabilities
- `plugin-query-skill`: `plugin/skills/query/` 提供的只读查账规范——检索方式、查询前后
  两次 `date` 换算的纪律、返回值字段语义的解读规则,以及与其余三个 skill 的边界。

### Modified Capabilities
(无——tally-mcp 事实层不变;`plugin-analysis-skill` spec 的 Requirement 不变,仅 skill
`description` 文案收窄。)

## Impact

- 新增文件:`plugin/skills/query/SKILL.md`。
- 修改文件:`plugin/skills/analysis/SKILL.md`(仅 `description` 与「不在范围内」一节
  补一句边界指引)。
- `openspec/specs/plugin-query-skill/spec.md`:新建。
- 不涉及 `mcp/` 下任何代码或 spec 变更;不新增 MCP 工具;无破坏性变更。
