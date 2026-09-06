## REMOVED Requirements

### Requirement: 会话开始时注入当前时间上下文
**Reason**:Claude Code / claude.ai 的系统提示词本身已经会注入当前日期(`currentDate`),这份注入是宿主平台级的能力,不依赖 hook 执行所在主机的系统时钟。tally 自己的 hook 只是在重复宿主已经提供、而且提供得更可靠的信息——尤其是在 cowork / claude.ai 网页版这类托管执行环境里,hook 实际跑在服务器上,系统时区是 UTC,反而比宿主自带的注入更不可靠。
**Migration**:相对时间表达("今天/昨天/上周五")的解析,改为以宿主系统提示词自带的 `currentDate` 为锚点,用 `date` 命令计算;缺少该注入时沿用既有兜底规则,先运行 `date` 取当前时刻。

### Requirement: 每轮用户输入刷新当前时间上下文
**Reason**:同上——宿主系统提示词本身会在每轮输入时保持当前日期上下文最新,不需要 tally 自己的 hook 重复做这件事。
**Migration**:无需替代实现,依赖宿主自身对 `currentDate` 的刷新机制。

### Requirement: hook 仅注入上下文,不做拦截或判断
**Reason**:这条 Requirement 约束的是 tally 自己的 hook 实现细节;随着该 hook 整体删除,这条约束不再有对象。
**Migration**:无需替代——三份 skill(record/query/analysis)里"agent 不得跳过预览、不得凭心算判断日期"等语义判断纪律不受影响,继续由 skill 层约束。

### Requirement: 注入内容不假设服务端时区语义
**Reason**:这条 Requirement 约束的是 tally 自己 hook 注入内容与 `time` 字段语义的关系;hook 删除后不再有需要澄清的注入内容。
**Migration**:`time` 字段本身"不带时区标记的本地日期时间字符串"这一事实,已在 `transaction-recording` 能力的 delta 里明确声明,不需要靠本能力重复约束。
