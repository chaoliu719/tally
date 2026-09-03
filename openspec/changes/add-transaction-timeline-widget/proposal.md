## Why

现在要"从新到旧翻看全部交易、按天分隔、一直滑到最早一条",对话里没有合适的形态:纯文字清单翻几十页会烧光上下文,一次性 artifact 又受宿主工具结果 ~150k 字符上限约束,账目一多就装不下。这类"无上界列表 + 滚动分页 + 零推理"的浏览,正是 MCP widget(Apps SDK)唯一能干净解决的场景——iframe 里滚动到底就 `callServerTool` 拉下一页,数据全程不进 Claude 上下文。

项目此前已把宿主锁定为 Claude app,`app 无关`这条对实践层不再是硬约束;可视化路线因此从"仅 artifact"扩展为"分析类图表用 artifact,交易历史浏览用 widget"。

## What Changes

- **新增 widget:交易时间线。** 一个 `open_transaction_timeline` 工具(声明 `_meta.ui.resourceUri`),宿主渲染成内联 iframe:交易按天分组、最新在上、向下滚动无限加载直到最早一条。只读浏览,不触发任何写操作。
- **`search_transactions` 增加倒序翻页。** 新增可选入参 `newest_first`(默认 `false`,保持现有"最旧在前"行为不变);为 `true` 时结果按 `time`/`id` 降序,`next_cursor` 沿时间轴向更早的方向翻页。widget 用这个模式增量拉取。
- **tally-mcp 开始托管 UI 资源。** 事实内核新增一类 `ui://` 资源(mime `text/html;profile=mcp-app`),内联 vendored 的 Apps SDK 浏览器 bundle。这是对"纯 MCP server,不做前端 UI;对话就是唯一的 UI"的一次显式让步,边界见下。
- **`openspec/config.yaml` 更新可视化决定:** 从"skill + templates 动态生成 artifacts(何种可视化尚未研究)"改为已定路线——分析类图表走 artifact;交易历史这类无上界列表浏览走 widget,数据经 `callServerTool` 增量拉取、不进 Agent 上下文。

### 边界(仍然不做)

- widget 不做写操作、不做二次确认;点某一行至多把该交易的引用 `sendMessage` 回对话交给 Claude(v1 甚至可以完全只读)。
- 不引入前端框架、不加载任何外部资源(CSP 全禁),widget 是自包含单文件。
- 不为分析类图表做 widget——那条路线仍是 artifact。

## Capabilities

### New Capabilities

- `transaction-timeline-widget`: 通过 MCP Apps SDK 暴露一个只读的交易时间线 UI 资源——按天分组、倒序、滚动分页浏览一个账本的全部交易;以及 tally-mcp 托管此类 `ui://` widget 资源的通用约束(自包含、无外部请求、宿主无此能力时优雅降级为工具文本)。

### Modified Capabilities

- `transaction-recording`: `search_transactions` 增加 `newest_first` 入参与对应的降序 keyset 游标翻页;默认行为不变。

## Impact

- **代码:** `mcp/internal/tools/`(新 `open_transaction_timeline` 工具、`search_transactions` 降序游标)、`mcp/internal/store/`(降序 keyset 查询)、`mcp/internal/tools/transactions_cursor.go`(游标编码带方向)、MCP 资源注册处新增 widget 资源、`mcp/` 内新增 `widgets/transaction-timeline.html` 与 vendored Apps SDK bundle。
- **规格:** 新增 `openspec/specs/transaction-timeline-widget/spec.md`;修改 `openspec/specs/transaction-recording/spec.md`。
- **配置:** `openspec/config.yaml` 的可视化段落。
- **部署:** aliyun 上的 tally-mcp 现在还要版本化 HTML/JS 资产;每次改 widget UI 需重新部署 server。
- **无 breaking:** 新入参有默认值;新工具/资源是增量;不支持 Apps 表面的宿主忽略 `_meta.ui` 后仍能看到工具文本。
