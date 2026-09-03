## Context

见 proposal.md — Why。补充约束:

- tally-mcp 是 Go,依赖 `github.com/modelcontextprotocol/go-sdk v1.7.0`。Apps SDK 的服务端 helper(`registerAppTool`/`registerAppResource`)只有 TypeScript 版;Go 侧要手工挂 `_meta.ui.resourceUri` 到工具结果、并注册一个 mime 为 `text/html;profile=mcp-app` 的资源。
- Apps SDK 的**浏览器**运行时(`@modelcontextprotocol/ext-apps/app-with-deps`)是一段纯静态 JS,可 vendored 进仓库,构建期内联进 widget HTML 的 `/*__EXT_APPS_BUNDLE__*/` 占位符。
- iframe 受 sandbox + 严格 CSP:禁外部脚本/样式/字体/图片、禁 `window.open`、禁 `eval`、禁跨域 `fetch`。
- 宿主工具结果文本上限 ~150k 字符(claude.ai/Desktop)。
- `search_transactions` 现为 keyset 升序翻页,游标编码 `(last_time, last_id)` + 全部筛选条件的指纹(`transactions_cursor.go`)。

## Goals / Non-Goals

**Goals:**
- 一个只读交易时间线 widget,倒序、按天分组、滚动到最早一条。
- widget 翻页数据不进 Agent 上下文。
- 非 Apps 宿主下降级为工具文本,不报错。
- `search_transactions` 获得倒序翻页,默认行为零变化。

**Non-Goals:**
- 分析类图表 widget(仍走 artifact 路线)。
- widget 内编辑/删除交易、行内二次确认。
- 搜索框、按分类/来源过滤的面板内交互(v1 只做"全部交易"这一个视图;过滤仍走对话 + `search_transactions`)。
- 多账本切换器(打开时锚定一个 `ledger_id`)。
- 桌面版资源缓存的自动失效处理(靠部署时 bump 资源 URI 或 server 重启)。

## Decisions

### D1:widget 而非 artifact
无上界列表 + 滚动分页 + 零推理,是 artifact 结构性做不到的(数据要烘进 HTML ⇒ 先进 Agent 上下文 ⇒ 撞 150k 上限 / 烧 token)。widget 的 `callServerTool` 增量拉取是唯一干净解。代价:事实内核破例托管 UI 资源、每次改 UI 重新部署 server —— proposal 已显式接受。备选(artifact + 只加载最近 N 条)被否:达不到"直到最早一条"。

### D2:独立的 `open_transaction_timeline` 工具,不把 widget 挂到 `search_transactions`
`search_transactions` 是 query/analysis skill 的通用工具,给它挂 widget 会让每次查询都弹面板。新工具语义单一:"打开时间线面板"。工具描述里明确写"交互式、可滚动浏览全部交易",让 Claude 在用户说"翻看全部账目"时选它、而不是狂翻 `search_transactions` 文本页。

### D3:launcher 工具首屏内联第一页
`open_transaction_timeline` 的结果同时给:(a) `_meta.ui.resourceUri`;(b) 文本 = 摘要 + 最新一页(~50 条)的 JSON。widget 的 `ontoolresult` 直接吃这批做首屏,省一次 round trip;后续页才 `callServerTool`。同一批 JSON 也是非 Apps 宿主的降级内容。~50 条远低于 150k 上限。

### D4:`search_transactions` 加 `newest_first` 布尔,不做新工具
倒序是同一查询的排序变体,keyset 翻页方向相反而已。游标指纹已覆盖"全部筛选条件",把 `newest_first` 纳入指纹即可防止方向混翻。默认 `false` ⇒ 现有调用与测试全部不变。store 层加一条降序 keyset 查询(`WHERE (time,id) < (?,?) ORDER BY time DESC, id DESC`)。

### D5:按天分组在 widget 里做,用宿主时区
服务端坚持"不碰时区"(config 硬约束)。widget 在浏览器里用宿主本地时区把每条 `time` 归到自然日。分组、日期标题、"今天/昨天"这类措辞全是纯前端。

### D6:vendored Apps SDK bundle,构建期内联
把 `app-with-deps` 那段 JS 提交进 `mcp/widgets/vendor/`(带版本号注释),Go 在启动时读 widget HTML 模板、替换占位符、缓存成字符串,由资源处理器返回。不引 npm 到 Go 构建链。

### D7:supersession
同一工具多次调用会并排挂多个 iframe。旧面板只读、不 `sendMessage`,所以 supersession 风险低;仍加一个 `BroadcastChannel` 让旧实例在检测到更新的实例后停止自动翻页(省无谓请求),但不做视觉置灰。

### D8:autoResize + 显示模式
`new App(..., { autoResize: true })` 让 iframe 高度跟随内容。提供 `requestDisplayMode('fullscreen')` 按钮(仅当 `availableDisplayModes` 含 fullscreen 时显示),长列表浏览体验更好。

## Risks / Trade-offs

- **Go SDK 无 Apps helper,`_meta.ui` 靠手挂** → 先写一个最小 spike:注册资源 + 工具带 `_meta.ui.resourceUri`,在 claude.ai 自定义连接器里确认能渲染;spike 不通过就回到 artifact 方案并缩减本 change。
- **每次改 widget UI 要重新部署 aliyun 的 server** → 接受;widget 逻辑刻意做小做稳,减少迭代频率。
- **桌面版激进缓存 UI 资源** → 开发期靠 ⌘Q 重启;生产靠部署时资源内容变化即可(URI 可保持不变,内容 hash 变)。
- **iframe CSP 静默失败(白框)** → 开发流程里固定开 iframe 自己的 devtools 看 CSP 报错;并加一个 `/widget-preview` GET 路由 + 假 `ExtApps` shim 在普通浏览器里调样式。
- **降级文本被 Claude 当成"完整答案"继续推理** → 摘要文本里写明"这是最新一页,完整浏览需在支持面板的宿主打开",避免 Claude 误以为拿到了全量。
- **超大账本单次滚到底** → 可接受:每页 ≤200,keyset 翻页 O(1);用户想跳到某段时间仍应走对话 + `search_transactions` 时间范围。

## Migration Plan

1. 先落 D4(`search_transactions` 倒序),独立可测、独立可发。
2. Go 侧 Apps 资源 spike(见 Risks 第一条)。
3. spike 通过后实现 widget + launcher 工具。
4. 部署:server 重新构建发布到 aliyun;widget 资源随二进制走,无独立发布物。
5. 更新 `openspec/config.yaml` 可视化段落。
6. 回滚:下线 `open_transaction_timeline` 工具与资源注册即可;`newest_first` 入参无害,可留。

## Open Questions

- widget 里点某一行是否要 `sendMessage` 把该交易引用抛回对话(方便接着让 Claude 改)?v1 可先纯只读,加这个不影响 specs 与任务结构 —— 留到实现时按手感定。
- 是否需要在面板顶部显示一个跨全账本的合计/条数条?不影响分页主逻辑,视实现成本决定。
