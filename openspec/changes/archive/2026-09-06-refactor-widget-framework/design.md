## Context

见 proposal.md - Why。当前状态:

- `mcp/internal/widgets/widgets.go` 用 `//go:embed timeline.html` + 一个 `rendered` map
  字面量 + 一个 `PreviewHTML` map 字面量,把「timeline」这个名字写死在三处;
  `browserBundle()` 把 vendored ext-apps ES module 的尾部 `export{}` 正则改写成
  `globalThis.ExtApps=`,在 `/*__EXT_APPS_BUNDLE__*/` 占位符处 `strings.Replace` 拼进去。
- `mcp/internal/widgets/timeline.html`(812 行)整段脚本包在 `(async () => { ... })()`
  里(scope 隔离,guard 测试 `TestTimelineScriptIsScopeIsolated`),混合了宿主桥接
  (`new App(...)`、`app.ontoolresult/ontoolinput/onhostcontextchanged`、`app.connect()`)、
  主题(`applyTheme`)、全屏(`syncDisplayMode` + `#expand`)、supersession
  (`BroadcastChannel`)、工具结果解析(`extractFirstPagePayload`、`parseSearchResult`)、
  以及全部业务逻辑(筛选、分类树、drain/翻页、渲染、`dayKeyOf`/`formatDay`/`fmtAmount`
  等格式化)。
- `previewShim` 是 `widgets.go` 里一个 ~40 行 const 字符串,把假宿主类和 timeline 专属的
  `_demoBatch` / 写死的 `list_categories` `list_sources` `search_transactions` 响应
  焊在一起。
- `tools/timeline.go` 手写 `s.AddResource(...)`(含 `res.TTLMs` / `res.CacheScope`)+
  `mcp.AddTool(..., Meta: mcp.Meta{"ui": map[string]any{"resourceUri": ...}})`。

硬约束(不可破):不引 npm/vite/esbuild 进 Go 构建链(见
`add-transaction-timeline-widget` design.md D6);widget 脚本 + 共享 runtime 必须 scope
隔离;widget 只读、只经 `callServerTool`、不加载任何外部资源;不做时区处理;金额按
ISO 4217 真实小数位;`widgets` 包不 import mcp SDK。

## Goals / Non-Goals

**Goals:**

- 加 widget #2 = 放一个 `src/<name>/index.html`(+ 可选 `preview.js`)+ 写一个
  `tools/<name>.go`(只有 tool handler + 两行 helper 调用)。不再动 `widgets.go` /
  `tools.go` / `main.go`。
- 宿主桥接/主题/全屏/supersession/工具结果解析这些管道代码,全 widget 只有一份实现。
- 交易时间线 widget 的可观察行为(工具、`_meta.ui`、资源 MIME/TTL/CacheScope、降级文本、
  面板交互)逐字节等价——渲染 HTML 只多出 `runtime.js` 文本这一处差异。
- 多 widget 注册路径有真实测试覆盖,但不往生产二进制里塞一个一次性 widget。

**Non-Goals:**

- 不抽日期/金额格式化进 runtime(`dayKeyOf`/`rowTimeOf`/`formatDay`/`fracDigits`/
  `fmtAmount`/`catLabel` 留在 `timeline/index.html`)——它们是记账域逻辑,不是宿主管道。
- 不引入构建步骤、不引入第三方前端运行时。
- 不做真正的第 2 个 widget。
- 不改 preview 的开关方式(仍 `TALLY_DEV_WIDGET_PREVIEW=1` + `/widget-preview/<name>`)、
  不加 preview 索引页。
- 不引入 per-widget 服务端状态或「当前 widget」概念。

## Decisions

### D-A:目录约定 `src/<name>/index.html`,而非扁平 `src/*.html`

sidecar(现在 `preview.js`,以后可能 `widget.css`)跟 widget 放同一个目录,加 sidecar
不用再动注册表。目录名即 widget 名(`HTML`/`PreviewHTML`/`URI`/`widgets.URI` 的入参)。
`//go:embed all:src` 一次性 embed 整棵树。

*Alternatives*:(a) 扁平 `src/timeline.html` + `src/timeline.preview.js` 命名约定——
sidecar 与 widget 的关系靠文件名前缀维系,加类型就要改 glob;(b) 维持逐个 `//go:embed`
+ 一个 `[]WidgetSpec` 切片——仍要每个 widget 改一处 Go 代码。目录约定最接近「零代码改动」。

### D-B:共享 runtime 作为 `globalThis.Tally = (function(){…})()`,拼进同一个占位符 payload

不新增第二个占位符标记。`BundlePlaceholder` 的替换值从 `<host-runtime>` 变成
`<host-runtime>\n<runtime.js>`。好处:(1) `timeline/index.html` 原样迁移,不用加新
占位符行;(2) runtime 在结构上保证与 `globalThis.ExtApps` 同处出现——包括 preview 模式
(preview 的替换值是 `preview-shim.js + <widget preview.js> + runtime.js`);(3) 占位符
计数校验只需继续检查「恰好一个」。

runtime.js 本身用单个顶层表达式(命名空间赋值 + IIFE)满足 scope 隔离,内部所有 helper
是闭包内的函数声明,不泄漏顶层 `const`/`let`。

*Alternative*:两个占位符(`__EXT_APPS_BUNDLE__` + `__TALLY_RUNTIME__`)——更显式,但每个
新 widget 的 `index.html` 都要记得写两行,且漏写第二行不会立刻失败(runtime 缺失,行为
诡异)。单占位符 + 拼接把「runtime 一定在」变成结构保证。

### D-C:切分线——只把宿主形状/被复制的管道 helper 移进 runtime

进 runtime:`App` 构造(`autoResize` 恒 true)、`ontoolinput/ontoolresult/
onhostcontextchanged` 的安装、`applyTheme`(含 `theme !== "light" && theme !== "dark"`
的 guard 逐字保留)、`syncDisplayMode` + `#expand` 点击 + 初次同步、`BroadcastChannel`
supersession(`seq` / `onmessage` / `postMessage`,暴露 `isSuperseded()`)、`boot`/`fatal`
+ `connect()` try/catch(「连接中…」「已连接,等待数据…」文案逐字保留)、`withTimeout`、
`extractFirstPagePayload`、`parseSearchResult`。

留 `timeline/index.html`:全部业务状态与逻辑,加日期/金额格式化(见 Non-Goals)。

runtime 公开 API:`await Tally.mountWidget({ name, version, channel, onToolInput,
onToolResult })`,返回 `{ app, isSuperseded, extractFirstPagePayload, parseSearchResult,
withTimeout }`。**`mountWidget` 必须在 `await app.connect()` 之前安装 `onToolInput` /
`onToolResult` 回调**——这是首屏渲染正确性的关键(现在 `ontoolresult` 在 765 行装、
`connect()` 在 801 行调)。connect 成功后 `mountWidget` 内部再跑
`applyTheme(getHostContext()?.theme)` + `syncDisplayMode(getHostContext())`。

`timeline/index.html` 保留自己的外层 `(async () => { ... })()`(所以
`TestTimelineScriptIsScopeIsolated` 的 `"(async () => {"` 断言不变),在其中
`await Tally.mountWidget(...)`。

*Alternative*:把格式化也一起抽进 `Tally.fmt.*`——下一个「交易类」widget 能直接复用。
用户明确否决:runtime 保持「纯宿主框架」,记账域的东西不进去;真需要时另开 `tallyfmt.js`。

### D-D:preview = 通用 `preview-shim.js` + per-widget `preview.js` fixture

`preview-shim.js`(embed):`globalThis.ExtApps = { App: class TallyPreviewHost {...} }`,
实现 runtime 会调的每个 `App` 方法,读 `?theme=` / `?ledger_id=` / `?payload=`,先 fire
`ontoolinput` 再 `ontoolresult`(顺序同现在),数据委托给 `globalThis.TallyPreviewFixture`。

`src/timeline/preview.js`(per-widget embed,**JS 不是 JSON**):设
`globalThis.TallyPreviewFixture`,内含现 `_demoBatch` 函数体逐字 + 写死的
`list_categories`/`list_sources`/`search_transactions` 响应 + 有状态的 `page` 计数
(timeline preview 需要模拟翻页,JSON 表达不了状态)。

缺 fixture 时 `buildRegistry` 给该槽拼 `""`,shim 的 fallback 返回空 payload,preview 仍
渲染。

### D-E:`buildRegistry(fsys fs.FS, host string) (widgetSet, error)` 纯函数、可注入 FS

生产用 `buildRegistry(widgetFS, browserBundle())`;测试用 `fstest.MapFS` 塞
`src/a/index.html` + `src/b/index.html` + `src/b/preview.js`,直接验证多 widget 发现、
占位符替换、fixture 只进对应 widget 的 preview。这样「填空题」路径有真实覆盖,而不必
shipping 一个一次性 widget(违反仓库「延后的不要 propose」纪律,且多一个对用户可见的
`open_xxx` 工具)。

### D-F:通用 `registerWidgetResource` / `widgetToolMeta` 放 `tools` 包

`widgets` 包继续不 import mcp SDK(现状)。新 `tools/widgethost.go` 提供:

- `registerWidgetResource(s *mcp.Server, name, resourceName, description string)` —— 泛型的
  `AddResource` + `res.TTLMs` + `res.CacheScope = "public"`,handler 只调 `widgets.HTML(name)`。
- `widgetToolMeta(name string) mcp.Meta` —— 产出正好
  `mcp.Meta{"ui": map[string]any{"resourceUri": widgets.URI(name)}}`(e2e 会读回校验)。
- `widgetResourceTTL` const 从 `timeline.go` 移来。

per-widget tool handler(输入/输出结构体 + 业务逻辑)仍各写各的——它们本来就不一样。

### D-G:`transaction-timeline-widget` 不带 spec delta

本 change 是行为保持型重构:同一个工具、同样的 `_meta.ui`、同样的资源
MIME/TTL/CacheScope、同样的降级文本、同样的面板行为。原 spec 里「widget 资源自包含且不
发起外部请求」requirement 描述的通用规则,由新 capability `mcp-app-widget-framework`
以更完整的形式承接;归档时不改动 `transaction-timeline-widget` 主 spec。

## Risks / Trade-offs

- **桥接回调 vs `connect()` 顺序搞反** → `mountWidget` 内部严格先装 `onToolInput` /
  `onToolResult` 再 `await connect()`;design 与 tasks 都点名这一条;第 4 步手动 preview
  检查首屏能出数据。
- **逐字搬动时手滑改了文案/正则** → `applyTheme` guard、`extractFirstPagePayload`、
  `parseSearchResult`、boot 文案属「一字不改」清单;`TestTimelineWidgetBehaviours` 的
  子串断言(跑在完整渲染 HTML 上)会抓到多数改动。
- **runtime.js 引入了被负断言禁掉的 token**(`filterApply`、`button.primary`、
  `src="http`、`<link `、`@import`、`//esm.sh`、`//unpkg`、`//cdn`、顶层 `const io`)
  → runtime 只写函数声明、不写 `<link>`、注释里不出现这些串;通用
  `TestEveryWidgetSelfContained` + 保留的 `TestTimelineWidgetBehaviours` 负断言把关。
- **preview 一致性无法自动测**(Go 构建里没有 JS runner)→ 第 4、5 步各有一次手动
  `/widget-preview/timeline?theme=dark` + `?payload=...` 检查:按天分组、筛选、翻页、
  drain、全屏按钮、深色主题、demo 数据翻页都与重构前一致。
- **`all:src` 把 `preview.js` 也 embed 进二进制** → 无害(永不发给宿主),review 时
  在 spec/README 注明:`src/` 下不放机密或大文件。
- **`buildRegistry` 对格式错误 widget 的处理** → 占位符数 ≠ 1、缺 `index.html` 一律
  返回 error,`registry` 的 init IIFE `panic`(和现在同样的失败风格),`go test` /
  启动即炸,不会把裸占位符发上线。`TestRegistryRejectsMissingPlaceholder` 固化这条。

## Migration Plan

按 tasks.md 的 9 步走,每步后 `cd mcp && go build ./... && go test ./...` 必须全绿;
change 目录存在后 `openspec validate refactor-widget-framework --strict` 全绿。

关键顺序:先纯文件移动(第 2 步,`git mv`,零逻辑改动)→ 再上 `embed.FS` 注册表(第 3 步,
仍单 widget)→ 再抽 runtime(第 4 步)→ 再拆 preview(第 5 步)→ 再收缩 tool 注册
(第 6 步)→ 通用化测试(第 7 步)→ 文档(第 8 步)→ 收尾全量验证(第 9 步)。

**回滚**:每步都是稳定 Go API(`HTML`/`PreviewHTML`/`URI`/`MIMEType`)与稳定线上契约
(`ui://widgets/timeline.html`、`_meta.ui.resourceUri`、资源 MIME/TTL、降级文本)背后的
内部重构。回滚 = `git revert` 对应提交,无数据迁移、无客户端可见 URI/schema 变化。上线后
若发现面板回归,revert 并重新部署二进制(aliyun),宿主在一个 5 分钟 TTL 内取回旧 HTML。
