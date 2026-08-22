## Purpose

让唯一用户能够通过 MCP 工具查看现有交易分类、创建新分类,为记录交易提供必要的分类主数据。分类体系与 ezbookkeeping 原生模型一致,保留两层结构:一级分类(用于分组,不可直接用于记账)与二级分类(挂在某个一级分类下,`create_transaction` 必须引用二级分类)。二级分类之下不能再有子级。这次改动只覆盖查看与创建,更新、删除、排序等能力留给后续 change。

## ADDED Requirements

### Requirement: 查看全部交易分类
用户 SHALL 能够通过 `list_categories` 工具获取当前账本下的全部交易分类及其所属类型(收入/支出/转账)与父分类 id,以便区分一级分类与二级分类。

#### Scenario: 账本为空
- **WHEN** 调用 `list_categories` 时账本中还没有任何分类
- **THEN** 返回一个空列表,而不是错误

#### Scenario: 账本已有分类
- **WHEN** 调用 `list_categories` 时账本中已存在分类
- **THEN** 返回全部分类,每个分类包含名称、所属类型与父分类 id(一级分类的父分类 id 为空/0)

### Requirement: 创建新交易分类
用户 SHALL 能够通过 `manage_category` 工具创建一个新的交易分类,指定名称、所属类型(收入/支出/转账),以及可选的 `parent_id`。不提供 `parent_id`(或提供 0)时创建一级分类;提供一个已存在的一级分类 id 时,在其下创建二级分类。

#### Scenario: 创建一级分类
- **WHEN** 调用 `manage_category` 创建分类,不提供 `parent_id`(或提供 0),名称与类型均合法
- **THEN** 分类被创建为一级分类,随后出现在 `list_categories` 的结果中;这个分类本身不能被 `create_transaction` 引用

#### Scenario: 在一级分类下创建二级分类
- **WHEN** 调用 `manage_category` 创建分类,提供的 `parent_id` 是当前账本中已存在的一级分类,名称与类型均合法
- **THEN** 分类被创建为该一级分类下的二级分类,随后出现在 `list_categories` 的结果中,可以被 `create_transaction` 引用

#### Scenario: parent_id 指向的分类已经是二级分类
- **WHEN** 调用 `manage_category` 创建分类,提供的 `parent_id` 对应一个已存在的二级分类(该分类自身已有父分类)
- **THEN** 请求被拒绝,返回说明"分类体系只支持两级,不能在二级分类下再创建子分类"的错误,不创建任何分类

#### Scenario: parent_id 指向不存在的分类
- **WHEN** 调用 `manage_category` 创建分类,提供的 `parent_id` 在当前账本中不存在
- **THEN** 请求被拒绝,返回说明父分类不存在的错误,不创建任何分类

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_category` 创建分类,但缺少名称或类型
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不创建任何分类
