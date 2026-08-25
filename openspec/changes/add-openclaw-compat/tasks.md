## 1. plugin-record-skill:泛化 MEMORY.md 措辞

- [x] 1.1 `plugin/skills/record/SKILL.md` frontmatter 的 `description` 里,把"什么该
      现算、什么才该写进 MEMORY.md"改成 host-agnostic 表述,不点名具体 harness
- [x] 1.2 正文"什么该写 MEMORY.md，什么不该"这个 H2 标题和其下两条要点,改成不点名
      `MEMORY.md` 的通用表述(如"写进你当前 host 的记忆机制,例如 Claude Code 的
      MEMORY.md"),对齐 `optimize/SKILL.md`"提议周期性复盘"一节已经采用的写法
- [x] 1.3 对照 `openspec/specs/plugin-record-skill/spec.md` 里"账本数据投影不写入
      Agent 记忆"这条 Requirement 的两个 Scenario,确认改写后的正文仍然完整覆盖
      (该现算的仍现算、该写用户偏好的仍可写),没有改变行为,只是措辞泛化

## 2. 新增 `plugin/README.md`

- [x] 2.1 写 Claude Code 安装段落:`/plugin marketplace add chaoliu719/tally` →
      `/plugin install tally@tally`,以及所需环境变量 `TALLY_MCP_URL`/
      `TALLY_MCP_TOKEN`(对照 `plugin/.mcp.json` 里实际用到的变量名,不要写错)
- [x] 2.2 写 openclaw 安装段落:`openclaw plugins install tally --marketplace
      chaoliu719/tally`(已实测 `plugins marketplace list` 能正确解析出这个插件,
      装不装得上还未实测,见第 3 节),复用同一组环境变量说明,不重复写一遍
- [x] 2.3 在 openclaw 段落里加一张字段对照表,说明如果 `.mcp.json` 里的 `tally`
      HTTP server 没有被自动导入,如何在 openclaw 原生 `mcp.servers` 配置里手动补
      (`type`→`transport`,`"http"`→`"streamable-http"`,`url`/`headers` 不变),并
      明确标注"是否自动导入未经验证"
- [x] 2.4 在 openclaw 段落里说明装完插件后,`record`/`analysis`/`optimize` 三个
      skill 可能需要在 openclaw 侧手动启用(`skills.entries` 设 `enabled: true`)并
      加进目标 agent 的 `skills` 数组才会生效,并注明这一步是从其他已装插件 skill 的
      真实配置结构观察到的规律,不是官方文档写明的行为

## 3. 在用户的 openclaw 实例上验证(需要用户确认后再执行)

- [x] 3.1 **执行前向用户确认**:接下来要在 mac-mini 上对用户正在用的生产 openclaw
      实例运行 `docker exec openclaw-openclaw-gateway-1 node dist/index.js plugins
      install tally --marketplace chaoliu719/tally`,这会修改该实例的插件注册表,
      需要用户明确同意后才执行——用户已确认同意,已执行(先装了一次未推送的旧内容,
      推送后用 `--force` 重装取到了最新内容)
- [x] 3.2 确认同意后执行安装,用 `plugins inspect tally --json` 或 `plugins doctor`
      检查三个 skill 是否被正确发现,以及 `.mcp.json` 里的 `tally` server 有没有被
      翻译进 `mcp.servers`(对照已有的手工配置条目,确认没有产生冲突;如果没有自动
      导入,保留手工那条即可,不依赖自动导入)——实测确认:三个 skill 被发现
      (`openclaw-extra` 来源);`.mcp.json` 的 HTTP server **没有**被自动翻译
      (`plugins inspect` 的 diagnostics 明确报告"stdio only today"),但手工配置的
      `mcp.servers.tally` 条目完好、无冲突
- [x] 3.3 按第 2.4 步的说明,在 `skills.entries` 里启用三个 skill,并加进
      `agents.list` 里 `id: "tally"` 那条记录的 `skills` 数组——`skills.entries`
      里未出现独立条目,是通过 `openclaw config patch --stdin` 把三个 skill 名加进
      `agents.list[id=tally].skills` 数组做到的;`openclaw skills info record
      --agent tally` 确认变成 `✓ Ready` / `Visible to model: yes`
- [ ] 3.4 重载 openclaw 后在 feishu 里跟 tally agent 实际对话一次,触发一次记账
      场景,确认能读到 SKILL.md 内容并正常调用 `tally` MCP 工具——**未完成**:用
      `openclaw agent --agent tally --message ... --session-key
      agent:tally:verify-openclaw-compat`(不加 `--deliver`,不会真发到 feishu)
      触发时遇到模型 provider(nvidia 的 deepseek 模型)rate limit,两次重试均返回
      `FailoverError: API rate limit reached`,和本次改动无关。留给用户在 provider
      恢复后自行验证(见 design.md Risks 一节)
- [x] 3.5 根据 3.2-3.4 的实测结果,把 `plugin/README.md` 里"未经验证"的措辞改成
      确定性结论(要么写"会自动生效",要么写"需要手动配置",不再用推测性语言);
      design.md 的 Risks 一节如有需要一并更新——两处均已更新为实测结论,3.4 的
      rate-limit 阻塞也如实记录在 design.md 里,没有假装已验证

## 4. 收尾

- [x] 4.1 运行 `openspec validate --change add-openclaw-compat --strict`,确认
      proposal/design/tasks 通过校验、无格式问题(本次 `skip_specs: true`,不生成
      spec delta)
- [x] 4.2 通读 `plugin/README.md` 两条安装路径的每一条命令,确认变量名和
      `mcp/README.md`/`plugin/.mcp.json` 里实际用的变量名完全一致,没有笔误
