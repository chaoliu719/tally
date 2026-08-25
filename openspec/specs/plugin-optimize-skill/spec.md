# plugin-optimize-skill Specification

## Purpose

定义 agent 检测分类体系该拆分或合并的信号、安全执行这类结构性调整的流程,以及在当前会话可用能力范围内提议周期性复盘的方式。

## Requirements

### Requirement: 检测分类拆分信号
Agent SHALL 在检视某个较宽分类近期的交易时,识别其中是否有一部分交易的 `comment` 明显反复指向同一个具体主题,并据此向用户提议拆分。

#### Scenario: 宽泛分类下出现主题聚集
- **WHEN** agent 检视某个较宽分类近期的交易,发现其中一部分交易的 `comment` 明显反复指向同一个具体主题
- **THEN** agent 向用户提议从该分类下拆出一个更细的子分类,并给出建议名称与命中该主题的交易示例

### Requirement: 检测分类合并信号
Agent SHALL 在检视账本全部分类时,识别是否存在不同时间创建、名称或实际记录内容明显指向同一含义的分类,并据此向用户提议合并。

#### Scenario: 名称或用途相近的分类
- **WHEN** agent 检视账本全部分类,发现两个在不同时间创建、名称或实际记录内容明显指向同一含义的分类
- **THEN** agent 向用户提议将两者合并为一个,并说明各自当前的交易数量

### Requirement: 合并前必须先呈现影响范围
Agent SHALL 在执行分类合并之前,先用 `search_transactions` 统计受影响的交易数量并展示示例,不得在未展示影响范围前直接开始批量修改。

#### Scenario: 用户同意合并两个分类
- **WHEN** 用户同意将分类 A 合并入分类 B
- **THEN** agent 先用 `search_transactions(category_id=A)` 统计受影响的交易数量并展示若干示例,再执行重新分类

### Requirement: 合并通过逐笔重新分类加安全删除完成
Agent SHALL 通过对分类 A 下每笔交易调用 `update_transaction` 改为分类 B,待分类 A 不再被任何交易引用后,再对分类 A 执行 `manage_category` 的 delete 操作(遵循 `write-confirmation` 的 preview → apply 两步确认)来完成合并。

#### Scenario: 执行合并
- **WHEN** 用户确认合并分类 A 到分类 B
- **THEN** agent 对分类 A 下的每笔交易依次调用 `update_transaction` 将其 `category_id` 改为 B,待分类 A 不再被任何交易引用后,对分类 A 调用 `manage_category`(`operation="delete"`)并完成其 preview → apply 两步确认

### Requirement: 交易量较大时提前告知成本
Agent SHALL 在待合并分类下的交易数量较多、需要多次 `update_transaction` 调用才能完成重新分类时,在执行前告知用户大致的调用量或预期耗时。

#### Scenario: 待合并分类交易数量较多
- **WHEN** 待合并分类下的交易数量较多,需要多次 `update_transaction` 调用才能完成重新分类
- **THEN** agent 在执行前告知用户大致需要多少次调用/预期耗时,由用户决定是否值得现在执行

### Requirement: 拆分同样遵循先呈现后执行
Agent SHALL 在拆分分类时,先创建新分类并展示将被移动的交易列表或数量,再逐笔重新分类。

#### Scenario: 用户同意拆分
- **WHEN** 用户同意从一个分类中拆出新的子分类
- **THEN** agent 先调用 `manage_category`(`operation="create"`)建立新分类,展示将被移动的交易列表或数量,再逐笔将匹配的交易通过 `update_transaction` 改为引用新分类

### Requirement: 周期性复盘建议不硬编码具体调度机制
Agent SHALL 在判断适合向用户提议周期性复盘时,检查当前会话可用的工具/skill 列表中是否存在宿主(harness)提供的定时/计划任务能力,存在则使用该能力,不存在则明确告知用户当前环境无法自动定时;skill 内容本身不得假设或硬编码某个具体的定时工具名称或调用方式。

#### Scenario: 检测到值得建立周期性复盘
- **WHEN** agent 判断适合向用户提议定期(如每周/每月)检查分类拆分/合并信号
- **THEN** agent 检查当前会话可用的工具与 skill 列表中是否存在宿主提供的定时/计划任务能力;若存在则用该能力设置提醒,若不存在则明确告知用户当前环境无法自动定时

#### Scenario: 不同宿主环境的定时能力不同
- **WHEN** 同一份 skill 内容在不同的宿主环境中被使用
- **THEN** agent 依据当次会话实际暴露的工具/skill 判断如何设置提醒,skill 内容不预先假设某个具体的定时工具存在
