#!/bin/sh
# tally plugin — 时间上下文注入
#
# 向对话注入当前日期 / 星期 / 本地时区,作为 agent 把「今天」「昨天」「前天」
# 「上周五」等相对时间表达解析为具体日期的锚点。
#
# 约束(见 openspec/specs/plugin-time-context-hook/spec.md):
# - 只同步读取一次系统时钟,输出一行,立即退出。
# - 不拦截、不阻断、不修改任何工具调用;不做语义判断;不驻留后台进程。
# - 注入的时区是「用户本地时区」,供缺少更明确信息时作默认假设;
#   不改变 tally MCP 中 time 字段为纯 Unix 秒数、无时区语义的事实。

set -eu

today=$(date '+%Y-%m-%d')
dow=$(date '+%u')
tz=$(date '+%Z %z')

case "$dow" in
  1) weekday='星期一' ;;
  2) weekday='星期二' ;;
  3) weekday='星期三' ;;
  4) weekday='星期四' ;;
  5) weekday='星期五' ;;
  6) weekday='星期六' ;;
  7) weekday='星期日' ;;
  *) weekday='' ;;
esac

printf '记账时间锚点:今天是 %s %s,用户本地时区 %s。解析「今天 / 昨天 / 前天 / 上周X / N 天前」等相对时间表达时以此为锚点,并用 shell 的 date 命令换算成具体日期,不要心算。\n' \
  "$today" "$weekday" "$tz"
