# category-management Specification

## Purpose
让唯一用户能够通过 MCP 工具查看、创建、更新、删除交易分类,为记录交易提供必要的分类主数据。分类可以任意嵌套——`parent_id` 可以指向账本中任意已存在的分类,不限层级深度;`create_transaction` 可以引用账本中任意分类。排序等能力留给后续 change。

## Requirements

### Requirement: 查看全部交易分类
用户 SHALL 能够通过 `list_categories` 工具,指定 `ledger_id`,获取该账本下的全部交易分类及其名称与父分类 id。分类不再区分固定的层级——`parent_id` 可以指向同一账本内任意已存在的分类,嵌套深度不限;`parent_id` 为空/0 表示这是一个顶层分类。

#### Scenario: 账本为空
- **WHEN** 调用 `list_categories` 指定一个已存在的账本,但账本中还没有任何分类
- **THEN** 返回一个空列表,而不是错误

#### Scenario: 账本已有分类
- **WHEN** 调用 `list_categories` 指定一个已存在的账本,账本中已存在分类
- **THEN** 返回该账本下的全部分类,每个分类包含名称与父分类 id(顶层分类的父分类 id 为空/0),不包含其他账本的分类

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `list_categories`,指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回"未找到"错误

### Requirement: 创建新交易分类
用户 SHALL 能够通过 `manage_category` 工具,以 `operation="create"`,在指定的 `ledger_id` 下创建一个新的交易分类,指定名称,以及可选的 `parent_id`。不提供 `parent_id`(或提供 0)时创建一个顶层分类;提供该账本中已存在的分类 id 时,在其下创建子分类,不限制嵌套深度。

#### Scenario: 创建一级分类
- **WHEN** 调用 `manage_category`(`operation="create"`)创建分类,指定的 `ledger_id` 已存在,不提供 `parent_id`(或提供 0),名称合法
- **THEN** 分类被创建为该账本下的顶层分类(`parent_id` 为 0),随后出现在该账本 `list_categories` 的结果中,且可以直接被同一账本内的 `create_transaction` 引用

#### Scenario: 在一级分类下创建二级分类
- **WHEN** 调用 `manage_category`(`operation="create"`)创建分类,提供的 `parent_id` 是同一账本中已存在的任意一个分类(不要求它自己是顶层分类),名称合法
- **THEN** 分类被创建为该分类的子分类,随后出现在该账本 `list_categories` 的结果中

#### Scenario: parent_id 指向的分类已经是二级分类
- **WHEN** 调用 `manage_category`(`operation="create"`)创建分类,提供的 `parent_id` 对应一个自身已经有父分类的分类
- **THEN** 分类照常被创建为它的子分类——分类不再限制只能两层嵌套,这种情况不再被拒绝

#### Scenario: parent_id 指向不存在的分类
- **WHEN** 调用 `manage_category`(`operation="create"`)创建分类,提供的 `parent_id` 在当前账本中不存在(包括该 id 实际属于另一个账本的情况)
- **THEN** 请求被拒绝,返回说明父分类不存在的错误,不创建任何分类

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_category`(`operation="create"`)创建分类,但缺少名称或 `ledger_id`
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不创建任何分类

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `manage_category`(`operation="create"`),指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回说明账本不存在的错误,不创建任何分类

### Requirement: 更新交易分类
用户 SHALL 能够通过 `manage_category` 工具,以 `operation="update"` 并指定分类 `id` 与其所属 `ledger_id`,更新一个已存在分类的 `name` 与 `parent_id`。这两个字段必须一并提供(完整替换)。`parent_id` 可以指向同一账本中除该分类自身及其所有子孙分类之外的任意其他分类,也可以是 0(把该分类变为顶层分类)。

#### Scenario: 修改名称
- **WHEN** 调用 `manage_category`(`operation="update"`),指定一个已存在的分类 `id` 与其所属的 `ledger_id`,提供新的 `name` 与保持不变的 `parent_id`
- **THEN** 分类的名称被更新,随后 `list_categories` 反映新值

#### Scenario: 挪动到另一个分类下
- **WHEN** 调用 `manage_category`(`operation="update"`),`parent_id` 指向同一账本中另一个已存在、且不是该分类自身子孙的分类
- **THEN** 该分类的父分类被更新为新指定的分类,随后 `list_categories` 反映新的 `parent_id`

#### Scenario: 目标分类不存在
- **WHEN** 调用 `manage_category`(`operation="update"`),指定的 `id` 不对应任何已存在的分类
- **THEN** 请求被拒绝,返回"未找到"错误,不修改任何分类

#### Scenario: ledger_id 与分类实际所属账本不一致
- **WHEN** 调用 `manage_category`(`operation="update"`),指定的分类 `id` 确实存在,但提供的 `ledger_id` 不是它实际所属的账本
- **THEN** 请求被拒绝,返回"未找到"错误,不修改任何分类

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_category`(`operation="update"`),但 `name`、`parent_id` 或 `ledger_id` 未提供
- **THEN** 请求被拒绝,不修改任何分类

#### Scenario: 挪给自己
- **WHEN** 调用 `manage_category`(`operation="update"`),`parent_id` 等于该分类自己的 `id`
- **THEN** 请求被拒绝,返回说明不能把分类挂在自己下面的错误,不修改任何分类

#### Scenario: 挪到自己的子孙分类下,形成环
- **WHEN** 调用 `manage_category`(`operation="update"`),`parent_id` 指向该分类的某个子分类、子分类的子分类,或更深层的子孙分类
- **THEN** 请求被拒绝,返回说明这会形成环状引用的错误,不修改任何分类

#### Scenario: parent_id 指向不存在的分类
- **WHEN** 调用 `manage_category`(`operation="update"`),`parent_id`(非 0)在当前账本中不存在(包括该 id 实际属于另一个账本的情况)
- **THEN** 请求被拒绝,返回说明父分类不存在的错误,不修改任何分类

### Requirement: 删除交易分类
用户 SHALL 能够通过 `manage_category` 工具,以 `operation="delete"` 并指定分类 `id` 与其所属 `ledger_id`,删除一个分类。这是一个破坏性操作,遵循 `write-confirmation` 能力定义的 preview → apply 两步流程。一个分类只要还有任意子分类挂在它下面,或者被同一账本内任意交易引用,就不能被删除,且不提供强制覆盖的方式。

#### Scenario: 删除一个没有子分类也没有交易引用的分类
- **WHEN** 先以 `operation="delete"` 不带 `confirmation_token` 调用 `manage_category`(得到预览与 `confirmation_token`),再以同样的 `id` 与 `ledger_id` 携带该 `confirmation_token` 调用 `operation="delete"`,且这个分类在两次调用之间始终没有子分类、也没有被任何交易引用
- **THEN** 分类被删除,不再出现在该账本 `list_categories` 的结果中

#### Scenario: 删除一个仍有子分类的分类
- **WHEN** 以 `operation="delete"` 调用 `manage_category`,目标分类下还挂着至少一个子分类(不论该子分类自己有没有被交易引用)
- **THEN** 请求被拒绝,返回说明该分类仍有子分类、无法删除的错误,不删除该分类,也不签发 `confirmation_token`

#### Scenario: 删除一个仍被交易引用的分类
- **WHEN** 以 `operation="delete"` 调用 `manage_category`,目标分类被同一账本内至少一笔交易引用
- **THEN** 请求被拒绝,返回说明该分类仍被引用、无法删除的错误,不删除该分类,也不签发 `confirmation_token`

#### Scenario: 目标分类不存在
- **WHEN** 调用 `manage_category`(`operation="delete"`),指定的 `id` 不对应任何已存在的分类,或对应的分类不属于提供的 `ledger_id`
- **THEN** 请求被拒绝,返回"未找到"错误
