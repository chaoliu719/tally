## Context

`plugin/` 已经是标准 Claude Code 插件布局(根 `.claude-plugin/marketplace.json` +
`plugin/.claude-plugin/plugin.json` + `plugin/skills/*/SKILL.md` + `plugin/.mcp.json`)。
用户已经在自己的 openclaw 生产实例(`ghcr.io/openclaw/openclaw:latest`,version
2026.7.1,docker compose 部署)上手工配置了一个 `mcp.servers.tally` 条目连到同一台机器
上跑的 `tally-mcp` 容器,并且已经建好一个专门的 `tally` agent(绑定在 feishu 频道)。
用户希望 `plugin/` 也能装进这个 openclaw 实例。调研阶段直接在这台机器上跑了 openclaw
自己的 CLI 命令验证,而不是只依赖文档(`docs.openclaw.ai` 部分措辞含糊,网上另有一批
关于"openclaw"的可疑内容站点信息不可靠)。见 proposal.md 的 Why。

## Goals / Non-Goals

**Goals:**
- 确定 `plugin/` 要不要为了兼容 openclaw 调整仓库布局,并把结论落成可执行的
  README 安装文档。
- 把 skill 正文里唯一的 Claude-Code-专属措辞(`MEMORY.md`)泛化,不引入新的行为
  规则。

**Non-Goals:**
- 不做仓库拆分或把 `plugin/` 提到仓库根——见 Decision 1。
- 不在 mac-mini 上实际执行 `plugins install`(会改动用户正在用的生产 openclaw 实例的
  状态)——本次 change 只交付文档和 skill 措辞;实际安装/联调放到 tasks.md 的验证步骤
  里,由用户确认后再执行,不作为本次代码/文档改动的一部分自动发生。
- 不新增或修改 `commands/`、`agents/`、`hooks/`——`plugin.json` 描述里提到的这几类
  组件目前都还没有内容,不属于这次要处理的范围。

## Decisions

**1. 仓库布局保持不变,openclaw 安装方式用 `--marketplace`,不做 `git:` 裸装或仓库拆分。**

原因:实测(`docker exec openclaw-openclaw-gateway-1 node dist/index.js plugins
marketplace list chaoliu719/tally --json`)证实 openclaw 的 `plugins install <name>
--marketplace <source>` 就是为"根目录放 Claude marketplace.json、插件在子目录"这种
布局设计的选项,克隆仓库后能正确解析出 `tally` 插件在 `./plugin`。备选方案(把
`plugin.json` 复制一份到仓库根让 `git:` 裸装可用,或者把 `plugin/` 拆成独立仓库)都
放弃:前者会产生两份 manifest 的版本漂移风险,且违背"根 marketplace.json + 子目录
plugin.json"这个布局本来就是为将来仓库里能放多个 plugin 留的口子;后者直接推翻
`openspec/config.yaml` 里"tally = mcp/ + plugin/ 同仓库兄弟目录"这个刚定下不久的
架构决策,没有足够理由。

**2. `plugin/.mcp.json` 保持 Claude Code 的 schema,不额外为 openclaw 写一份 MCP 配置文件,不确定的部分在 README 里用字段对照表说明,而不是猜测/假装已验证。**

原因:`plugin/.mcp.json` 用的字段名(`type: "http"`)和 openclaw 原生 `mcp.servers`
配置的字段名(`transport: "streamable-http"`)不同。openclaw 官方文档对"插件包导入器
是否会把 `.mcp.json` 里的 HTTP 类型服务器翻译进原生配置"这一点写得含糊(原文只明确
提到 stdio)。但用户的机器上已经有一条手工配置、且确认在跑的 `mcp.servers.tally`
条目,指向同一个 tally-mcp 实例——也就是说不管插件安装会不会自动做这层翻译,可用性
已经有保障(两边字段值不冲突,`install` 默认也不会覆盖已有配置)。因此不需要为了这
一个不确定点在仓库里再造一份 openclaw 专用的 MCP 配置文件,只需要在 README 里把两边
字段名的对应关系写清楚,作为"如果自动导入没生效,照着这张表手动补"的备用说明。

**3. `plugin/skills/record/SKILL.md` 的措辞泛化只处理"MEMORY.md 该不该写"一节,不
重写整份 skill,也不新建一份 openclaw 专用的 SKILL.md 副本。**

原因:三份 SKILL.md 里只有这一处点名了 Claude Code 专属的记忆文件约定;其余内容全部
只引用 tally-mcp 的协议工具名,天然可移植。`plugin-record-skill` 的现有 spec(见
`openspec/specs/plugin-record-skill/spec.md`)描述的"账本数据投影不写入 Agent 记忆 /
用户偏好可写入 Agent 记忆"这条 Requirement 本身已经是 host-agnostic 措辞,不点名
`MEMORY.md`——说明泛化只需要发生在 SKILL.md 正文这一层,不需要动 spec。为两个 host
各维护一份 SKILL.md 会立刻产生两份文本漂移的维护成本,而问题本身只是一处措辞,没有
必要引入这种复杂度。

## Risks / Trade-offs

- [`.mcp.json` 的 HTTP server 是否被 openclaw 插件导入器自动翻译成原生 schema,
  目前没有实测坐实,只是"不影响可用性"的合理推断] → 缓解:README 里明确写"未验证自动
  导入是否生效",附字段对照表作为手动兜底,不写成确定性的"会自动生效"。真正跑一次
  `plugins install` 之后应该把这处措辞更新为已验证的结论(见 tasks.md 的验证步骤,
  需要用户确认后再对生产实例执行)。
- [装了插件后,openclaw 侧新出现的 `record`/`analysis`/`optimize` skill entries 默认
  可能是禁用状态,需要额外手动启用并加进目标 agent 的 `skills` 数组——这是从这台机器
  上其他已装插件(如 feishu-doc、canvas)的真实配置结构观察到的规律,不是 openclaw
  文档写明的行为] → 缓解:README 把这两步启用操作写成安装说明的一部分,不假设装完
  插件就自动可用。
- [验证步骤需要对用户正在用的生产 openclaw 实例执行 `plugins install`,是有状态改动
  的操作] → 缓解:tasks.md 把这一步单独列出,明确标注需要用户在执行前再次确认,不在
  本次 change 的常规实施流程里自动跑。
