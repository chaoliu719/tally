## Why

`manage_ledger` 的 `operation="update"` 目前允许"`name` 与/或 `comment` 至少提供一个"的部分更新——未提供的字段保留原值。这与项目里 `account`（`source` 的前身实体）/`source`/`category` 一贯遵循的"更新即完整替换，所有字段必须一并提供"惯例不一致：`account-management`（已归档）明确写过"不支持只修改其中一个字段而保留其余字段原值"，`category-management` 的 `update` 也要求 `name`/`parent_id` 一并提供。

这个不一致是当初撰写 `ledger-management` spec（`add-ledger-entity` 变更）时遗漏既有惯例导致的疏忽，不是刻意的设计差异——没有任何 `design.md`/`proposal.md` 记录过为什么账本要单独允许部分更新。现在补上，让 `manage_ledger` 的更新语义与 `source`/`category` 保持一致，降低调用方（Agent）需要记忆"哪个资源的更新是部分的、哪个是全量的"这一额外心智负担。

## What Changes

- `manage_ledger`（`operation="update"`）改为要求 `name` 与 `comment` 两个字段必须一并提供（完整替换），不再允许只传其中一个而保留另一个字段原值。**BREAKING**：调用方此前依赖"只传 `name` 保留原 `comment`"（或反之）的行为将不再受支持，必须显式传入两个字段的完整目标值。
- 更新 `manage_ledger` 工具的 description，去掉"and/or ... at least one must be provided"的部分更新措辞，改为要求两者都提供。

## Capabilities

### Modified Capabilities
- `ledger-management`: "更新账本信息" Requirement 的更新语义从"`name` 与/或 `comment`，至少一个"改为"`name` 与 `comment` 必须一并提供（完整替换）"。

## Impact

- `internal/tools/ledgers.go`：`manage_ledger` 工具 description、`updateLedger` 的字段校验逻辑（去掉"未提供字段保留原值"的合并逻辑，改为两个字段都必填）。
- `internal/tools/ledgers_test.go`：更新既有的部分更新测试用例，新增"只传 `name` 应被拒绝"和"只传 `comment` 应被拒绝"的场景。
- `README.md`：如果其中描述了 `manage_ledger` 的更新语义，需要同步措辞。
- 无数据库 schema 变更，无需迁移。
