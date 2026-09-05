## Why

时间线 widget 每次交互都要等 3-5s:开场先并发两次 `callServerTool`(`list_categories` + `list_sources`)才补上分类/来源名字并填满筛选下拉;每次翻页越过已取回范围要一次 `search_transactions`;每次应用/重置筛选都作废本地数据、带参数重新联网分页。这些往返的耗时几乎全在 `widget iframe → 宿主 → model-context 传输通道 → 服务端` 这条桥接链路上(SQLite 查询本身是亚毫秒级),每次固定 2-5s,与数据量无关。对一个单用户、账本规模仅数千条交易的场景,把整份数据一次拿进 iframe、之后全部在本地算,是更合适的取舍。

## What Changes

- `open_transaction_timeline` 工具结果新增两组随首屏一起返回的查找表:该账本全部分类的 `id → {name, parent_id}`、全部来源的 `id → name`。widget 首屏即可显示分类/来源名字并填满筛选下拉,**开场不再发起任何 `callServerTool` 查找请求**。
- `open_transaction_timeline` 首屏返回的交易条数上调(50 → 200),减少后续按需取数的往返次数与首屏后的等待窗口。
- widget 首屏渲染后,在后台沿游标把该账本(在无筛选状态下的)**剩余交易全部取回**到本地;取回期间「下一页/页尾」保持可用并按已到数据推进,取完后翻页与「页尾」变为纯本地操作、瞬间响应。新实例仍通过既有的 supersession 机制让旧实例停止取数。
- **筛选改为纯本地**:应用/重置筛选不再作废本地数据、不再重新联网。起始/结束日期在 `time` 字符串的 `YYYY-MM-DD` 前缀上按字典序比较;分类/来源按 id 匹配;选中某分类时,用首屏返回的 `parent_id` 映射在本地展开其全部子孙分类一并匹配(等价于原 `include_descendants=true`)。筛选后翻页、分组、日合计全部基于本地过滤后的结果集。
- 存在活跃筛选时,面板顶部改为展示本地过滤后的**匹配条数**(此前只能显示"已筛选"文案,因为服务端总数不再适用);无匹配时仍显示"没有符合当前条件的交易"。
- widget 保留一条降级路径:当工具结果里没有分类/来源查找表(对接旧版服务端)时,回退到原先的 `list_categories` / `list_sources` 后台拉取。

## Capabilities

### New Capabilities
（无)

### Modified Capabilities
- `transaction-timeline-widget`: 「打开交易时间线 widget」——工具结果新增分类/来源查找表;「时间线面板按天分组、倒序、按页翻页」——首屏返回更多、首屏后后台取回全部剩余交易、筛选变更不再触发重新联网分页;「时间线面板支持时间范围/分类/来源过滤」——筛选改为在本地已取回的全量交易上进行,不再经 `search_transactions` 参数,子孙分类展开在本地完成,活跃筛选时展示本地匹配条数。

## Impact

- `mcp/internal/tools/timeline.go`:`OpenTransactionTimelineOutput` 新增 `categories` / `sources` 字段;`openTransactionTimeline` 额外查询这两张表并填充;`timelineFirstPageSize` 上调。文本降级摘要不变。
- `mcp/internal/tools` 下相关的输出结构 / jsonschema 说明,以及 `list_categories` / `list_sources` 现有返回结构的复用(尽量共用 TransactionInfo 之外的 CategoryInfo / SourceInfo 类型)。
- `mcp/internal/widgets/timeline.html`:`ontoolresult` 解析新字段并同步填充 `catById` / `srcById` / 筛选下拉;移除开场的 `loadLookups` 网络调用(保留为降级回退);新增首屏后的后台全量取数;`fetchNextChunk` 之上引入"本地过滤视图",`renderPage` / `computeDayTotals` / `pageCount` / `hasNextPage` / 翻页控件改为基于过滤后的结果集;`reloadWithFilters` 由"清空重拉"改为"本地重算";顶部统计文案在筛选态显示本地条数。
- 相关测试:`mcp/cmd/tally-mcp/e2e_transaction_timeline_widget_test.go`、`e2e_transaction_timeline_paging_test.go`、`mcp/internal/widgets/widgets_test.go` 需覆盖新结果字段与本地筛选行为。
- 主 spec `openspec/specs/transaction-timeline-widget/spec.md` 随本 change 归档时同步。
- 取舍:widget 会在每次打开时把整份账本交易拉进 iframe 内存。当前单用户、数千条规模下可接受;若将来账本量级显著变大,再回到服务端分页/筛选(记录在 design.md 的 trade-off 中)。
