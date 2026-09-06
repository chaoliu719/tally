## Why

目前只有一个交互式 widget(交易时间线),它把宿主桥接、主题、supersession、全屏、
工具结果解析等**与业务无关、每个 widget 都一样**的管道代码,和筛选/翻页/渲染等业务逻辑,
一起塞进单个 812 行的 `mcp/internal/widgets/timeline.html`;`tools/timeline.go` 又手写了
`AddResource` + TTL + `_meta.ui` 的整套注册样板。加第 2 / 第 3 个 widget 就意味着复制这
800+ 行、在 `widgets.go` 改三处 map、再抄一份注册块——而「自包含 / 不加载外部资源 /
scope 隔离」这些不变量在复制粘贴时极易被破坏。widget 路线已定(见
`add-transaction-timeline-widget`),后续必然会长出更多 widget,现在趁只有一个先把
框架层立起来,成本最低。

## What Changes

- **目录约定式 widget 注册表**:widget 源码迁到 `mcp/internal/widgets/src/<name>/index.html`,
  经 `//go:embed all:src` 自动发现;`widgets.go` 不再写死任何 widget 名。加 widget =
  放一个目录,不再改 `widgets.go`。
- **抽出共享前端 runtime** `mcp/internal/widgets/runtime.js`:手写、`go:embed`、scope 隔离
  (`globalThis.Tally`),在现有 `/*__EXT_APPS_BUNDLE__*/` 占位符处与 vendored ext-apps
  bundle 一起拼进每个 widget。**只抽 host 形状的管道**——`App` 桥接、`applyTheme`、
  `syncDisplayMode` + 全屏按钮、`BroadcastChannel` supersession、`boot`/connect 流、
  `extractFirstPagePayload`、`parseSearchResult`、`withTimeout`。业务逻辑(筛选、
  drain/翻页、渲染、分类树、日期/金额格式化)全部留在 `timeline/index.html`。
- **拆分 preview 假宿主**:`previewShim` const 拆成 `mcp/internal/widgets/preview-shim.js`
  (通用假宿主,embed)+ per-widget `src/<name>/preview.js`(demo fixture)。
- **通用注册 helper**:`tools/widgethost.go` 提供 `registerWidgetResource` 和
  `widgetToolMeta`;`registerTimelineTools` 的资源注册收缩成两行调用。tool handler
  逻辑不变。
- 加 `widgets.Names()`;widgets 单测通用化(遍历 `Names()`)+ 用 `fstest.MapFS` 覆盖
  多 widget 注册路径。
- 不含任何真正的第 2 个 widget(只做框架;首次真实使用留给后续 change)。

## Capabilities

### New Capabilities
- `mcp-app-widget-framework`: 约束 tally-mcp 托管 `ui://` widget 资源的通用机制——
  目录约定式发现、拼接与自包含不变量、scope 隔离不变量、资源缓存提示、以及本地开发
  preview harness 的契约。把原先散落在 `transaction-timeline-widget` spec「widget 资源
  自包含且不发起外部请求」requirement 里的通用规则,提升为一个独立、面向全部 widget 的
  capability。

### Modified Capabilities

无。本 change 不改交易时间线 widget 的任何可观察行为(同一个 `open_transaction_timeline`
工具、同样的 `_meta.ui`、同样的资源 MIME/TTL/CacheScope、同样的降级文本、同样的面板行为),
因此 `transaction-timeline-widget` 不带 delta。

## Impact

- **代码**:
  - `mcp/internal/widgets/widgets.go` —— 注册表重写(`embed.FS` 遍历 + `buildRegistry`)
  - `mcp/internal/widgets/timeline.html` → `mcp/internal/widgets/src/timeline/index.html`
  - 新增 `mcp/internal/widgets/runtime.js`、`preview-shim.js`、`src/timeline/preview.js`
  - 新增 `mcp/internal/tools/widgethost.go`;`mcp/internal/tools/timeline.go` 注册收缩
  - `mcp/internal/widgets/widgets_test.go` 通用化 + 新增 runtime / 注册表测试
- **规格**:新增 `openspec/specs/mcp-app-widget-framework/spec.md`(随本 change 归档时同步)
- **配置**:`mcp/README.md` widget 段更新为「丢一个目录」流程;`openspec/config.yaml`
  可视化段加一句
- **部署**:无客户端可见变化。唯一对外差异是渲染后的 widget HTML 多了 `runtime.js` 文本,
  宿主在一个 5 分钟 TTL 内重新拉取
- **breaking**:无。稳定 Go API(`HTML` / `PreviewHTML` / `URI` / `MIMEType`)与线上契约
  (`ui://widgets/timeline.html`、`_meta.ui.resourceUri`)不变
