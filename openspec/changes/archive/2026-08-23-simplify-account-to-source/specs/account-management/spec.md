## REMOVED Requirements

### Requirement: 查看全部账户
**Reason**: "账户"概念被"来源"取代,见 `source-management` 能力的"查看全部来源"。
**Migration**: 使用 `list_sources` 替代 `list_accounts`。返回字段不再包含类型、币种、余额,只有名称。

### Requirement: 创建新账户
**Reason**: "账户"概念被"来源"取代,见 `source-management` 能力的"创建新来源"。
**Migration**: 使用 `manage_source`(`operation="create"`)替代,只需提供名称,不再需要类型、币种、初始余额。

### Requirement: 更新账户信息
**Reason**: "账户"概念被"来源"取代,见 `source-management` 能力的"更新来源信息"。
**Migration**: 使用 `manage_source`(`operation="update"`)替代,只需提供名称;不再有类型、备注,也不存在"不可修改币种/余额"这类校验,因为这两个字段已经不存在。

### Requirement: 删除账户
**Reason**: "账户"概念被"来源"取代,见 `source-management` 能力的"删除来源"。
**Migration**: 使用 `manage_source`(`operation="delete"`)替代,preview → apply 两步流程与引用检查逻辑保持不变。
