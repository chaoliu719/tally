# Spec Delta

## ADDED Requirements

### Requirement: 时间线交易行分别展示商户名与备注
时间线面板的每一行 SHALL 把商户名与备注当作两个独立信息展示:`merchant` 非空时作为主标题,`comment` 非空时作为主标题下方的副行;`merchant` 为空时,主标题回退为 `comment`;两者都为空时,主标题回退为分类名,分类名也为空时回退为交易 ID。主标题下方已有的分类名副行 SHALL 与备注副行同时存在时保留,不得用备注覆盖分类。

#### Scenario: 商户名与备注都有值
- **WHEN** 一笔交易的 `merchant` 是"星巴克(南京西路店)"、`comment` 是"和同事下午茶"
- **THEN** 该行主标题显示"星巴克(南京西路店)",副行显示"和同事下午茶",两者不合并

#### Scenario: 只有备注
- **WHEN** 一笔交易的 `merchant` 为空、`comment` 是"和同事下午茶"
- **THEN** 该行主标题显示"和同事下午茶",不显示空白的主标题

#### Scenario: 只有商户名
- **WHEN** 一笔交易的 `merchant` 有值、`comment` 为空
- **THEN** 该行主标题显示商户名,副行显示分类名(若分类存在),不因备注为空而隐藏分类

#### Scenario: 两者都为空
- **WHEN** 一笔交易的 `merchant` 与 `comment` 都为空
- **THEN** 该行主标题回退为分类名;分类名也为空时回退为交易 ID,不显示空白行
