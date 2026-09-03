## 1. search_transactions 倒序翻页

- [x] 1.1 在 `SearchTransactionsInput` 加可选 `newest_first bool`,更新工具描述说明默认行为不变;`go build ./...` 通过
- [x] 1.2 store 层加降序 keyset 查询(`queries.sql` + sqlc 生成),`newest_first=true` 时按 `time DESC, id DESC` 翻页;单测覆盖升序/降序两条路径
- [x] 1.3 游标编码(`transactions_cursor.go`)把 `newest_first` 纳入指纹;用不同 `newest_first` 复用 cursor 被拒绝,单测覆盖
- [x] 1.4 `searchTransactions` 按 `newest_first` 选查询与游标方向,`next_cursor` 语义正确;补 e2e:降序首页→续翻→到最早一条后无 `next_cursor`
- [x] 1.5 跑全量既有 `search_transactions` 测试,确认默认行为零回归(`go test ./...` 全绿)

## 2. Go 侧 Apps 资源 spike

- [x] 2.1 vendored `app-with-deps` bundle(ext-apps@1.7.5)进 `mcp/widgets/vendor/`,带来源/再生成注释;启动期把尾部 `export{}` 改写成 `globalThis.ExtApps`
- [x] 2.2 `package widgets` 用 `//go:embed` 内联 HTML+bundle;`internal/tools/timeline.go` 注册 `ui://widgets/timeline.html` 资源(mime `text/html;profile=mcp-app`)+ `open_transaction_timeline` 工具(`_meta.ui.resourceUri`);e2e `TestE2ETransactionTimelineWidgetResource` + `widgets` 单测验证 `resources/read` 内容正确
- [x] 2.3 部署到 aliyun 后,在 claude.ai 调 `open_transaction_timeline`,确认 widget 渲染成 iframe —— spike 判定点:不通过则停下与用户重新评估方案

## 3. 交易时间线 widget

- [x] 3.1 写 `mcp/widgets/transaction-timeline.html`:单文件,内联样式/脚本 + `/*__EXT_APPS_BUNDLE__*/` 占位符;system 字体、CSS 变量 + `:root.dark` 配色块
- [x] 3.2 启动期读模板、替换 bundle 占位符、缓存;资源处理器返回该字符串
- [x] 3.3 widget 脚本:`App({autoResize:true})`,`ontoolresult` 吃首屏页,按宿主本地时区分组渲染日期段(最新在上)
- [x] 3.4 滚动接近底部 → `callServerTool("search_transactions",{newest_first:true,cursor,limit})` 追加下一页;滚动位置不跳动
- [x] 3.5 无 `next_cursor` → 显示"已到最早一条";空账本 → 空状态;两种都不发多余请求
- [x] 3.6 跟随宿主主题:`getHostContext().theme` + `onhostcontextchanged` 切 `.dark`;`BroadcastChannel` 让旧实例停止自动翻页
- [x] 3.7 fullscreen 按钮(仅当 `availableDisplayModes` 含 fullscreen);`app.openLink` 兜底任何外链
- [x] 3.8 加 `/widget-preview` GET 路由 + 假 `ExtApps` shim,普通浏览器可调样式;文档记一句用法

## 4. open_transaction_timeline 工具

- [x] 4.1 实现工具:校验 `ledger_id` 存在;取最新一页(≤50)+ 算总条数、最早/最新日期
- [x] 4.2 结果 `_meta` 挂 widget 资源 URI;文本 = 摘要 + 最新页 JSON,并注明"这是最新一页,完整浏览需在支持面板的宿主打开"
- [x] 4.3 账本不存在 → 明确错误;工具描述强调"交互式、可滚动浏览全部交易",引导 Claude 正确选用
- [x] 4.4 e2e:调用返回结构正确;MCP Inspector(无 Apps 表面)下降级文本可读不报错

## 5. 集成验证与收尾

- [ ] 5.1 claude.ai 端到端:打开面板 → 滚动翻到最早一条 → 切换深浅主题 → fullscreen;记录验证结果到 change
- [ ] 5.2 确认整个浏览过程 Agent 上下文未被交易数据填充(观察对话 token 不随滚动增长)
- [x] 5.3 更新 `openspec/config.yaml` 可视化段落为已定路线(artifact 图表 / widget 历史浏览)
- [x] 5.4 部署到 aliyun,更新部署记忆(如资源/路由有变);`openspec validate add-transaction-timeline-widget --strict` 通过
