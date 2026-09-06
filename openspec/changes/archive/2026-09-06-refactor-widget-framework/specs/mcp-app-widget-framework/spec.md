## Purpose

约束 tally-mcp 作为「纯事实内核」破例托管交互式 `ui://` widget 资源时的通用机制:widget
如何被发现与命名、渲染产物必须满足的自包含与脚本作用域不变量、资源读取要带的缓存提示,
以及本地开发用的 preview harness 契约。让新增一个 widget 只需遵循这些约定,而不必逐个
重新实现或复制这些保证。

## ADDED Requirements

### Requirement: widget 按目录约定被发现与命名

tally-mcp SHALL 把 widget 源码组织为「每个 widget 一个目录」:目录下的 `index.html` 是该
widget 的入口,**目录名即 widget 名**。服务端 SHALL 在启动时自动发现全部这样的 widget,
无需在代码里逐个登记 widget 名。每个 widget SHALL 以 URI `ui://widgets/<name>.html`、
mime 类型 `text/html;profile=mcp-app` 对宿主暴露为一个可读资源。

#### Scenario: 新增 widget 无需改注册代码

- **WHEN** 开发者新增一个 widget 目录(含 `index.html`)并重新构建服务端
- **THEN** 该 widget 在 `ui://widgets/<目录名>.html` 处可被 `resources/read`,不需要修改
  任何集中式的 widget 名列表

#### Scenario: 读取未知 widget

- **WHEN** 宿主对一个不存在的 widget 名发起 `resources/read`
- **THEN** 服务端返回「资源不存在」错误,不返回空白或半成品 HTML

### Requirement: widget 渲染产物自包含且脚本作用域隔离

每个 widget 的 `index.html` SHALL 恰好包含一个约定的运行时占位标记;服务端在加载时 SHALL
用内联的 Apps SDK 浏览器运行时与共享前端运行时替换该标记,若标记数量不为一则 SHALL 在
启动/测试期大声失败,而非把未替换的产物发给宿主。渲染后的 widget HTML SHALL 是单文件
自包含:内联全部所需脚本、样式与运行时,不引用任何外部脚本、样式表、字体或图片 URL,
不发起任何跨域网络请求;对账本数据的读取 SHALL 只通过 `callServerTool` 走 tally-mcp
自身的只读工具。widget 自身脚本与共享前端运行时 SHALL NOT 向宿主注入环境泄漏任何顶层
词法绑定(宿主可能把 widget 脚本拼进一个已声明若干短标识符的共享作用域,泄漏的顶层
`const`/`let` 会破坏整个注入)。

#### Scenario: 资源可被宿主直接渲染

- **WHEN** 宿主对 widget 的 `ui://` URI 发起 `resources/read`
- **THEN** 返回内容的 mime 类型为 `text/html;profile=mcp-app`,且是一个不依赖任何外部
  URL 即可运行的完整 HTML 文档,其中运行时占位标记已被替换、无残留

#### Scenario: widget 只读且不触达外部

- **WHEN** 审视任一 widget 脚本发起的全部 `callServerTool` 调用与网络行为
- **THEN** 全部指向 tally-mcp 自身的只读工具,没有任何创建/修改/删除数据的调用,也没有
  任何跨域请求

#### Scenario: 脚本不污染宿主作用域

- **WHEN** 检查渲染后的 widget HTML 内联脚本与共享前端运行时
- **THEN** 二者都不引入会泄漏到宿主注入作用域的顶层词法绑定(各自以 IIFE 或命名空间
  赋值的形式封闭)

### Requirement: widget 资源读取带缓存提示

每个 widget 资源的 `resources/read` 结果 SHALL 携带一个正的新鲜度提示(缓存 TTL)与
`public` 缓存范围,使宿主可在该时段内复用已缓存的 HTML,而不必每次打开面板或刷新页面都
重新拉取整份(约数百 KB)资源。

#### Scenario: 资源结果带缓存提示

- **WHEN** 宿主对任一 widget 的 `ui://` URI 发起 `resources/read`
- **THEN** 结果的 TTL 提示为正值、缓存范围为 `public`,宿主可在 TTL 内复用缓存副本

### Requirement: 本地开发 preview harness

服务端 SHALL 提供一个默认关闭、经显式开关开启的本地 preview 通道:开启后,能在普通浏览器
标签里打开某个 widget,widget 被接到一个**假宿主**上而非真实宿主运行时——假宿主提供
widget 运行所需的全部宿主接口,并按查询参数注入主题与首屏数据,数据来自该 widget 自带的
可选 demo fixture。preview 产物 SHALL NOT 内嵌真实的 Apps SDK 浏览器运行时。widget 未提供
demo fixture 时,preview SHALL 仍能渲染(以空数据降级),不报错。

#### Scenario: preview 使用假宿主

- **WHEN** 在开启 preview 开关后于普通浏览器打开某 widget 的 preview 地址
- **THEN** widget 正常挂载并渲染,其宿主接口由假宿主提供,首屏数据与主题取自查询参数与
  该 widget 的 demo fixture,页面中不含真实 Apps SDK 浏览器运行时

#### Scenario: widget 无 demo fixture

- **WHEN** 一个未附带 demo fixture 的 widget 被 preview 打开
- **THEN** 页面以空数据状态正常渲染,不抛错

#### Scenario: preview 默认关闭

- **WHEN** 未开启 preview 开关
- **THEN** preview 地址不可用,widget 只能经正常的 MCP 资源通道被宿主加载
