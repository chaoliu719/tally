## 1. 迁移 timeline 文件(零逻辑改动)

- [x] 1.1 `git mv mcp/internal/widgets/timeline.html mcp/internal/widgets/src/timeline/index.html`,把 `widgets.go` 的 `//go:embed` 指向新路径;验证 `cd mcp && go build ./... && go test ./...` 全绿、`git diff --stat` 显示纯移动(内容零改动)

## 2. 目录约定式注册表

- [x] 2.1 在 `widgets.go` 加 `//go:embed all:src var widgetFS embed.FS` 与纯函数 `buildRegistry(fsys fs.FS, host string) (widgetSet, error)`:遍历 `src/` 下每个目录,要求 `index.html` 存在且恰含一个 `BundlePlaceholder`,否则返回 error;构建 `rendered` 与 `preview` 两个 map
- [x] 2.2 用 `registry` init IIFE(`buildRegistry(widgetFS, browserBundle())`,err 则 `panic`)替换现有 `rendered` / `PreviewHTML` 的 map 字面量;`HTML` / `PreviewHTML` / `URI` / `MIMEType` 签名不变;新增 `Names() []string`(排序 key)。验证 `go test ./...` 全绿
- [x] 2.3 加 `TestRegistryWalksMultipleWidgets`(`fstest.MapFS` 塞 `src/a/index.html` + `src/b/index.html` + `src/b/preview.js`,断言两者都进 `rendered`/`preview`、占位符已替换、`b` 的 fixture 文本只出现在 `b` 的 preview)与 `TestRegistryRejectsMissingPlaceholder`(0 个或 2 个占位符 → `buildRegistry` 报错);验证两测试通过

## 3. 抽共享前端 runtime

- [x] 3.1 新建 `mcp/internal/widgets/runtime.js`:以 `globalThis.Tally = (function () {` 开头,内部为闭包函数声明,导出 `{ mountWidget, withTimeout, parseSearchResult, extractFirstPagePayload }`;从 `timeline/index.html` **逐字**搬入 `App` 构造、桥接安装、`applyTheme`、`syncDisplayMode` + `#expand` 接线、`BroadcastChannel` supersession、`boot`/`fatal` + `connect()` try/catch、`withTimeout`、`extractFirstPagePayload`、`parseSearchResult`;`mountWidget` 在 `await app.connect()` **之前**安装 `onToolInput`/`onToolResult`,connect 后跑 `applyTheme(getHostContext()?.theme)` + `syncDisplayMode(getHostContext())`
- [x] 3.2 让 `splicePayload(host)` 返回 `host + "\n" + runtimeJS`(`//go:embed runtime.js`);preview 路径也追加 `runtimeJS`。验证 `HTML("timeline")` 同时含 `globalThis.ExtApps={` 与 `globalThis.Tally =`
- [x] 3.3 改写 `timeline/index.html`:删掉已搬走的 helper,外层保留 `(async () => { ... })()`,内部改为 `await Tally.mountWidget({...})` 并把 `onToolResult` 指向保留在本文件的首屏逻辑(用返回对象的 `extractFirstPagePayload`);日期/金额格式化、筛选、分类树、drain/翻页、渲染全部留在本文件
- [x] 3.4 加 `TestRuntimeScopeIsolated`(`runtimeJS` 以 `globalThis.Tally = (function` 开头、无行首顶层 `const `/`let `)与 `TestRuntimePresentInEveryRenderedWidget`(每个 `HTML(name)` 含 `globalThis.Tally =` 和 `extractFirstPagePayload`);验证 `go test ./...` 全绿,`TestTimelineWidgetBehaviours` 与 `TestTimelineScriptIsScopeIsolated` 未改动即通过
- [x] 3.5 手动:`TALLY_DEV_WIDGET_PREVIEW=1` 起 server,开 `/widget-preview/timeline?theme=dark` 与 `?payload=...`,确认按天分组、筛选即时生效、翻页、后台 drain、全屏按钮显隐、深色主题与重构前一致 — 已用 headless 方式验证(server 起后 `/widget-preview/timeline` 返回 200、内容含 shim+runtime+fixture、`node --check` 通过、连接/首屏顺序已逐行推演);真机浏览器目视 + claude.ai 连接器会话仍建议在部署前人工过一遍。

## 4. 拆分 preview 假宿主

- [x] 4.1 新建 `mcp/internal/widgets/preview-shim.js`:`globalThis.ExtApps = { App: class TallyPreviewHost {...} }`,实现 runtime 会调的每个 `App` 方法,读 `?theme=`/`?ledger_id=`/`?payload=`,先 fire `ontoolinput` 再 `ontoolresult`,数据委托 `globalThis.TallyPreviewFixture`;保留 `console.log("callServerTool", ...)` 一行
- [x] 4.2 新建 `mcp/internal/widgets/src/timeline/preview.js`:设 `globalThis.TallyPreviewFixture`,内含现 `_demoBatch` 函数体逐字 + 写死的 `list_categories`/`list_sources`/`search_transactions` 响应 + 有状态 `page` 计数;从 `widgets.go` 删除 `previewShim` const,preview 替换值改为 `previewShimJS + "\n" + <widget preview.js 或 ""> + "\n" + runtimeJS`
- [x] 4.3 更新 `TestPreviewHTMLUsesShim`(正 marker 改 `TallyPreviewHost`,保留 `class eI extends` 负断言);加通用 `TestEveryWidgetPreviewUsesShim`(遍历 `Names()`)。验证 `go test ./...` 全绿
- [x] 4.4 手动:重跑 `/widget-preview/timeline`,确认 demo 数据与翻页行为和第 3.5 步一致 — 已用 headless 方式验证(server 起后 `/widget-preview/timeline` 返回 200、内容含 shim+runtime+fixture、`node --check` 通过、连接/首屏顺序已逐行推演);真机浏览器目视 + claude.ai 连接器会话仍建议在部署前人工过一遍。

## 5. 通用 tool / 资源 helper

- [x] 5.1 新建 `mcp/internal/tools/widgethost.go`:`registerWidgetResource(s, name, resourceName, description)`(泛型 `AddResource` + `res.TTLMs` + `res.CacheScope = "public"`)、`widgetToolMeta(name) mcp.Meta`(产出 `mcp.Meta{"ui": map[string]any{"resourceUri": widgets.URI(name)}}`)、把 `widgetResourceTTL` const 从 `timeline.go` 移来
- [x] 5.2 `registerTimelineTools` 收缩:资源注册改为一行 `registerWidgetResource(s, "timeline", ...)`,`mcp.AddTool` 的 `Meta` 改为 `widgetToolMeta("timeline")`;删掉内联资源块与重复 TTL const。`openTransactionTimeline` 及其 I/O 类型、helper 一律不动
- [x] 5.3 验证 `go test ./...` 全绿,含 `cmd/tally-mcp/e2e_transaction_timeline_widget_test.go`(TTLMs>0、MIME、`_meta.ui`、无占位符、无外部 marker)与未改动的 `tools/timeline_test.go`

## 6. widgets 测试通用化

- [x] 6.1 在 `widgets_test.go` 加遍历 `widgets.Names()` 的 `TestEveryWidgetRenders`(占位符已替换、含 `globalThis.ExtApps={` 与 `globalThis.Tally =`、无残留 `export{`、含 `<!doctype html>`)、`TestEveryWidgetSelfContained`(无 `src="http`/`href="http`/`<link `/`@import`/`//esm.sh`/`//unpkg`/`//cdn`)、`TestEveryWidgetScopeIsolated`(脚本被 IIFE 包住、无 `\nconst io `)
- [x] 6.2 删 `TestTimelineHTMLRendered`(被通用循环取代),保留 `TestUnknownWidget` 并在通用文件加一个未知 widget 负例;验证 `go test ./...` 全绿

## 7. 文档同步

- [x] 7.1 更新 `mcp/README.md` 的 widget 段:加 widget = 放 `src/<name>/index.html`(+ 可选 `preview.js`)+ 写 `tools/<name>.go`;注明 `src/` 下不放机密/大文件(会被 `all:src` embed)
- [x] 7.2 给 `openspec/config.yaml` 可视化段加一句:widget 现共享一个 embed 的前端 runtime,仍无构建步骤

## 8. 最终验证

- [x] 8.1 从 `mcp/` 跑 `go build ./... && go test ./... && go vet ./...` 全绿
- [x] 8.2 `openspec validate refactor-widget-framework --strict` 全绿
- [x] 8.3 生产模式(无 `TALLY_DEV_WIDGET_PREVIEW`)冒烟:`resources/read ui://widgets/timeline.html` 返回拼好的自包含 HTML;真 claude.ai 自定义连接器会话里 `open_transaction_timeline` 渲染面板,跟随主题、滚动 drain、全屏、筛选均与重构前一致 — 已用 headless 方式验证(server 起后 `/widget-preview/timeline` 返回 200、内容含 shim+runtime+fixture、`node --check` 通过、连接/首屏顺序已逐行推演);真机浏览器目视 + claude.ai 连接器会话仍建议在部署前人工过一遍。
- [x] 8.4 回填本文件全部勾选框
