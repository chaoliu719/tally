# Spec Delta

## MODIFIED Requirements

### Requirement: 备注原样保留
`备注` SHALL 直接写交易的 `comment` 原文,SHALL NOT 改写、精简或补充解释。`merchant` SHALL NOT 被拼接到 `备注` 列,也 SHALL NOT 为它新增列——目标网页只认固定的 7 列,商户名不进入导出文件。`comment` 超过 60 字时,Agent SHALL 在回复里提示目标网页的备注输入框有长度限制、预览中可能被截断。

#### Scenario: 备注含账单原文
- **WHEN** 交易的 `comment` 是"美团-外卖订单 1234,含配送费"
- **THEN** CSV 里该字段与 `comment` 完全一致,不做任何加工

#### Scenario: 交易同时有商户名与备注
- **WHEN** 一笔交易的 `merchant` 是"星巴克(南京西路店)"、`comment` 是"和同事下午茶"
- **THEN** CSV 的 `备注` 列只写"和同事下午茶",不写成"星巴克(南京西路店) 和同事下午茶",也不新增商户列

#### Scenario: 交易只有商户名没有备注
- **WHEN** 一笔交易的 `merchant` 有值、`comment` 为空
- **THEN** CSV 的 `备注` 列为空,不把 `merchant` 填进该列
