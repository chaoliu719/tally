## Purpose

让唯一用户能够通过 MCP 工具查看现有交易分类、创建新分类,为记录交易提供必要的分类主数据。这次改动只覆盖查看与创建,更新、删除、排序等能力留给后续 change。

## ADDED Requirements

### Requirement: 查看全部交易分类
用户 SHALL 能够通过 `list_categories` 工具获取当前账本下的全部交易分类及其所属类型(收入/支出/转账)。

#### Scenario: 账本为空
- **WHEN** 调用 `list_categories` 时账本中还没有任何分类
- **THEN** 返回一个空列表,而不是错误

#### Scenario: 账本已有分类
- **WHEN** 调用 `list_categories` 时账本中已存在分类
- **THEN** 返回全部分类,每个分类包含名称与所属类型

### Requirement: 创建新交易分类
用户 SHALL 能够通过 `manage_category` 工具创建一个新的交易分类,指定名称与所属类型(收入/支出/转账)。

#### Scenario: 提供有效信息创建分类
- **WHEN** 调用 `manage_category` 创建分类,提供的名称与类型均合法
- **THEN** 分类被创建,随后出现在 `list_categories` 的结果中

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_category` 创建分类,但缺少名称或类型
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不创建任何分类
