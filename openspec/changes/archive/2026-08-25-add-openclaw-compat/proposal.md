## Why

`plugin/` 目前只按 Claude Code 的习惯写文档和 skill 措辞(`record/SKILL.md` 直接点名
`MEMORY.md`),而且完全没有安装说明。用户已经在自己的 openclaw 生产实例上手工接好了
`tally` MCP server,想让同一个 `plugin/` 也能装进 openclaw 用。调研并在用户的 openclaw
实例上实测确认:openclaw 原生认得这个仓库现在的样子(根 `marketplace.json` + `plugin/`
子目录),`openclaw plugins install tally --marketplace chaoliu719/tally` 已经能正确
解析到 `tally` 插件,不需要挪动任何文件。真正要做的只是把 skill 正文里的一处
Claude-Code-专属措辞泛化,以及把两条安装路径写成文档——目前哪条路径都没有文档。

## What Changes

- `plugin/skills/record/SKILL.md` 里"什么该写 MEMORY.md，什么不该"一节,把对
  `MEMORY.md` 的措辞泛化为 host-agnostic 表述(如"写进你当前 host 的记忆机制,例如
  Claude Code 的 MEMORY.md"),对齐 `optimize/SKILL.md` 里"提议周期性复盘"一节已经
  采用的不硬编码宿主工具名的写法,以及 `openspec/config.yaml` 里"harness 的 MEMORY.md
  等"的泛化说法。frontmatter 的 `description` 字段同样有一处提到 MEMORY.md,一并泛化。
  这是纯措辞改动,不改变任何 SHALL 级别的行为规则(该现算的仍然现算、该写 Agent 记忆
  的仍然写)。
- 新增 `plugin/README.md`,记录两条安装路径:Claude Code(marketplace add + install)
  和 openclaw(`plugins install --marketplace`,已实测跑通),以及两边都要用到的环境
  变量(`TALLY_MCP_URL`/`TALLY_MCP_TOKEN`)。openclaw 一侧额外写明两个实测发现的坑:
  `plugin/.mcp.json` 用的是 Claude Code 的 MCP schema 字段名,和 openclaw 原生
  `mcp.servers` 的字段名不同,给出手动补充配置时的字段对照;以及装完插件后三个 skill
  默认不会自动对某个 agent 生效,需要在 openclaw 侧显式启用并加进目标 agent 的
  `skills` 列表。

## Capabilities

本次改动不引入或修改任何 agent 行为契约——`plugin-record-skill` 现有 spec 里
"账本数据投影不写入 Agent 记忆"这条 Requirement 本来就写的是"不写入 Agent 侧持久
记忆",没有点名 `MEMORY.md`,措辞泛化后这条 Requirement 描述的行为不变;新增的
`plugin/README.md` 是安装/运维文档,不是对话里的 agent 行为规则。因此本次 change 在
`.openspec.yaml` 里设置 `skip_specs: true`,不新增/修改任何 capability spec。

### New Capabilities
(无)

### Modified Capabilities
(无——见上,`plugin-record-skill` 现有 spec 无需变更)

## Impact

- 修改文件:`plugin/skills/record/SKILL.md`(措辞泛化,frontmatter + 正文各一处)。
- 新增文件:`plugin/README.md`。
- 不改:`plugin/.claude-plugin/plugin.json`、`plugin/.mcp.json`、根
  `.claude-plugin/marketplace.json`、`analysis/SKILL.md`、`optimize/SKILL.md`——
  这些已经是 host-agnostic,或不在本次改动范围内。
- 不涉及 `mcp/` 下任何代码变更;不新增 MCP 工具;不改变仓库目录结构。
