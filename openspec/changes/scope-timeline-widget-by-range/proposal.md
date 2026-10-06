## Why

`open_transaction_timeline` 只接受 `ledger_id`,打开后只能看整个账本:工具结果里内联了 200 条交易明细(连同 JSON 一起进了 Agent 的对话上下文),面板还会在后台把整份历史沿游标全部拉进 iframe。用户常见的需求其实是"看本月""看 9 月的账""看某个分类的账",要的是一个**精确划定范围的视图**,而不是"全部历史 + 事后筛选"。结果是数据量过大、范围不精确,Agent 想让用户只看一段时间也做不到。

## What Changes

- `open_transaction_timeline` 新增可选的范围参数:`start_date`、`end_date`(`YYYY-MM-DD`,含当天)、`category_id`(单值,默认连同全部子孙分类)、`source_id`(单值)。参数都是**可选且无隐含默认**:不传某项就是不限制该项,全不传等同于现在的"看全部"。
- 范围在打开时固定,**范围外的数据不会被加载**:首屏与面板后台取数都只在范围内进行(每一页取数都带同样的范围参数);想看别的范围,由 Agent 重新打开工具。
- 参数校验:日期格式非法、开始日期晚于结束日期、分类/来源不属于该账本,均返回明确错误,不渲染面板。
- 工具结果不再把交易明细写进文本:文本只放可独立阅读的摘要(范围、范围内条数、按币种的收入与支出合计);交易数据只走结构化结果供面板使用。结果同时返回生效的范围,供面板顶部与筛选条使用。
- 面板顶部始终显示当前范围与范围内条数(取代账本总数)。筛选条里被范围预置的分类/来源下拉改为只读标签;未被预置的保留下拉,继续在已加载数据里本地筛选;日期输入保留,但可选区间限制在范围内,筛选只能收窄、不能扩大范围。范围内没有交易时,空状态说明"该范围内没有交易"并带上范围信息。
- `plugin/skills/query` 新增一节教 Agent 使用时间线:什么时候该开面板而不是 `search_transactions`;用户没指明时间范围时**默认传最近 30 天**并在回答里说明;先用 `date` 算好日期再传;分类/来源先用 `list_categories` / `list_sources` 查 id;想看更早数据就重新打开并传新范围。默认值只存在于 skill,工具本身没有默认。

## Capabilities

### New Capabilities
（无)

### Modified Capabilities
- `transaction-timeline-widget`: 「打开交易时间线 widget」——文本摘要改为只含范围/条数/按币种收支合计、不含明细,结果携带生效范围;新增「按范围打开」要求(范围参数语义、校验、范围外不加载);「时间线面板按天分组、倒序、按页翻页」——取数只在范围内、顶部展示范围;「时间线面板支持时间范围/分类/来源过滤」——预置的分类/来源变为只读标签、日期输入受范围约束、筛选只能收窄。
- `plugin-query-skill`: 新增「用时间线面板浏览交易」要求——使用时机、默认最近 30 天、日期换算、id 查询、换范围需重新打开。

## Impact

- `mcp/internal/tools/timeline.go`:`OpenTransactionTimelineInput` 新增四个可选字段;参数解析与校验;首屏查询与游标指纹带上范围;文本摘要重写;结果新增 `scope` 字段;`TransactionStats` 的调用换成带范围的统计。
- `mcp/internal/store/queries.sql`(及 sqlc 生成的 `queries.sql.go` / `querier.go`):新增一条带范围过滤的统计查询,返回范围内条数与按币种分组的收入/支出合计。
- `mcp/internal/widgets/src/timeline/index.html`:读取 `scope`;后台取数的每页参数带上范围;顶部统计改为范围 + 范围内条数;预置分类/来源显示为只读标签;日期输入 `min`/`max` 限制;范围内空状态文案。
- 测试:`mcp/internal/tools/timeline_test.go`、`mcp/cmd/tally-mcp/e2e_transaction_timeline_widget_test.go`、`e2e_transaction_timeline_paging_test.go`、`mcp/internal/widgets/widgets_test.go`。
- 文档:`mcp/README.md` 中的工具说明;`plugin/skills/query/SKILL.md`。
- 主 spec:`openspec/specs/transaction-timeline-widget/spec.md` 的 Purpose 仍写着"浏览某个账本的全部交易——直到最早一条",归档时需手工改成"浏览账本在所选范围内的交易"(delta 不能改 Purpose)。
- 兼容性:新增字段与参数均为可选;不传参数的调用行为不变(看全部),旧面板忽略未知字段;文本摘要不再带 JSON 明细是对 Agent 可见内容的变化(见 design.md 的验证门槛)。
