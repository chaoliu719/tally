## Why

`open_transaction_timeline` 目前只能按 `ledger_id` 浏览一个账本的**全部**交易，从最新一条开始翻页。账本交易一多，用户想看的往往是某段时间、某个分类或某个来源的子集，现在只能一页页翻到目标区间附近，体验差、也没有办法直接定位。底层 `search_transactions` 早已支持 `source_id`/`category_id`/`start_time`/`end_time` 过滤，只是时间线面板从未把这些参数暴露给用户。

## What Changes

- 时间线面板 header 新增「筛选」开关按钮，点击展开/收起一条过滤条。
- 过滤条提供：起始日期、结束日期（`<input type=date>`）、分类下拉（父类平铺、子类缩进显示，从 `list_categories` 现有查表结果生成，选中父类时其全部子孙分类下的交易一并算入）、来源下拉（从 `list_sources` 现有查表结果生成）。
- 「应用」按钮：把当前过滤条件提交为面板的活跃过滤器，清空已加载的交易数据与翻页游标，回到第 1 页，用与现有分页相同的懒加载机制（`callServerTool` 调 `search_transactions`）重新按需拉取，只是这次带上过滤参数。
- 「重置」按钮：清空所有过滤条件，回到未过滤状态，同样触发一次干净的重新拉取。
- 过滤条件生效期间，页面顶部的统计文案从「共 N 笔」切换为「已筛选」，因为该总数来自 `open_transaction_timeline` 对整个账本的统计，过滤后不再准确；空结果的提示文案区分「这个账本还没有交易」与「没有符合条件的交易」两种情况。
- `open_transaction_timeline` 工具本身的输入 schema、返回内容不变，仍然只接受 `ledger_id`；过滤能力主体是面板内的客户端行为，通过已有的 `search_transactions` 工具实现，不新增 MCP 工具。
- `search_transactions` 新增一个可选参数 `include_descendants`(bool，默认 `false`，保持现有精确匹配语义不变)。为 `true` 时，`category_id` 的匹配范围扩大到该分类及其全部子孙分类，服务端复用已有的递归查询 `ListCategoryDescendantIDs` 算出子孙 id 集合。面板选中分类下拉时固定传 `include_descendants: true`；该参数默认关闭，不改变其他调用方（包括 `plugin/skills/optimize` 依赖精确匹配的分类合并流程）的既有行为。

## Capabilities

### New Capabilities
（无）

### Modified Capabilities
- `transaction-timeline-widget`：新增"时间线面板支持时间范围/分类/来源过滤"的需求，并说明它与现有"按天分组、倒序、按页翻页"需求的交互——过滤条件变更时视为一次全新的浏览起点，已取回数据与游标作废、重新按新条件分页。

## Impact

- 前端：`mcp/internal/widgets/timeline.html`（`go:embed` 进 tally-mcp 二进制）——时间/来源两个过滤维度是纯参数透传，无需后端改动。
- 后端：为支持"选父类连带子孙分类"，需要改动 `mcp/internal/store/queries.sql` 的 `SearchTransactions`/`SearchTransactionsDesc`、`mcp/internal/tools/transactions.go`（`search_transactions` 工具 schema 新增 `include_descendants`），并重新运行 `sqlc generate` 更新生成代码。具体查询方案见 design.md（含 JSON1/`json_each` 可用性的探路验证与备选方案）。
- 兼容性：`include_descendants` 默认 `false`，`search_transactions` 现有调用方（包括 `plugin/skills/optimize` 的分类合并流程、`tally:query`/`tally:analysis` 等 skill）行为不变，无需同步修改。
- 不影响 `open_transaction_timeline` 的输出契约。
