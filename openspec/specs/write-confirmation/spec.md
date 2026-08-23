# write-confirmation Specification

## Purpose
为破坏性写操作(账本删除、来源删除、分类删除、交易删除)提供一个无状态、可防重放的 preview → apply 两步确认机制,避免误操作导致数据丢失,同时不需要服务端保存任何会话状态。未来新增的破坏性操作(例如批量修改交易)可以复用同一套确认规则。

## Requirements

### Requirement: 不带确认令牌的调用只预览、不执行
对一个破坏性操作,SHALL 以调用时是否携带 `confirmation_token` 参数来区分这是预览(preview)还是确认执行(apply)。不携带 `confirmation_token` 的调用 SHALL 只做只读检查、不产生任何数据变化。

#### Scenario: 预览一个当前允许执行的破坏性操作
- **WHEN** 调用某个破坏性操作,不携带 `confirmation_token`,且目标资源当前满足该操作的所有前置条件
- **THEN** 请求返回待操作资源的信息、一个 `confirmation_token`、该令牌的过期时间,以及表示"等待确认"的状态,不产生任何实际的数据变化

#### Scenario: 预览一个当前不允许执行的破坏性操作
- **WHEN** 调用某个破坏性操作,不携带 `confirmation_token`,但目标资源当前不满足该操作的前置条件(例如仍被其他数据引用)
- **THEN** 请求被拒绝,返回说明前置条件未满足的错误,不签发 `confirmation_token`

### Requirement: 确认令牌的内容与签名
`confirmation_token` SHALL 是一个自包含(无状态)的令牌,携带以下信息并使用服务端持有的密钥做签名,使其在不知道密钥的情况下无法被伪造或篡改:被确认的具体操作(含目标资源类型,防止跨资源类型复用同一令牌)、目标资源的 id、一个反映目标资源当前状态的 revision 值、该令牌的过期时间。

#### Scenario: 令牌签名使用独立的密钥
- **WHEN** server 签发或校验 `confirmation_token`
- **THEN** 使用专门用于确认令牌的密钥(与 MCP 传输层认证用的密钥相互独立),该密钥通过启动配置提供

#### Scenario: 未配置确认密钥
- **WHEN** server 启动时,用于签发/校验确认令牌的密钥未被配置
- **THEN** server 启动失败并报错,不以缺少该配置的状态提供服务

### Requirement: 确认执行时校验令牌的完整性、时效与状态一致性
携带 `confirmation_token` 调用一个破坏性操作时,server SHALL 依次校验:签名是否有效、令牌绑定的操作与本次调用是否一致、令牌是否已过期、令牌里的 revision 是否与目标资源的当前状态一致。任意一项校验失败,SHALL 拒绝执行,不产生任何数据变化。

#### Scenario: 签名无效或被篡改
- **WHEN** 携带的 `confirmation_token` 签名校验不通过
- **THEN** 请求被拒绝,返回签名无效的错误,不执行操作

#### Scenario: 令牌绑定的操作与本次调用不一致
- **WHEN** 携带的 `confirmation_token` 是为另一个操作或另一类资源签发的(例如为删除来源签发的令牌被用来确认删除分类)
- **THEN** 请求被拒绝,不执行操作

#### Scenario: 令牌已过期
- **WHEN** 携带的 `confirmation_token` 已超过签发时设定的有效期(15 分钟)
- **THEN** 请求被拒绝,返回说明令牌已过期、需要重新预览的错误,不执行操作

#### Scenario: 目标资源在预览之后发生了变化
- **WHEN** 携带的 `confirmation_token` 通过了签名、操作匹配、时效校验,但目标资源自签发以来的当前状态(自身关键字段,或影响该操作是否允许执行的引用/依赖状态)与令牌里记录的 revision 不一致
- **THEN** 请求被拒绝,返回说明状态已变化、需要重新预览的错误,不执行操作

#### Scenario: 全部校验通过后仍在执行前重新确认一次
- **WHEN** 携带的 `confirmation_token` 通过了以上全部校验
- **THEN** server 在真正执行这次破坏性操作之前,原子性地重新检查一次该操作当前是否仍然满足前置条件;仍然满足才执行,否则拒绝并报错,不执行部分变更

#### Scenario: 用同一个令牌重复确认执行
- **WHEN** 一个 `confirmation_token` 已经被成功用于执行过一次操作,随后被再次用于确认执行
- **THEN** 由于目标资源已不处于令牌签发时的状态(通常已不存在或引用状态已变化),第二次调用被拒绝,不重复执行
