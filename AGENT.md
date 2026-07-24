# Project workflow

每完成一个步骤，立即将一条可审计记录加进 `log.md`，再开始下一个步骤。

- 标题必须使用 `YYYY-MM-DDTHH:MM:SS+08:00 — 事件`；持续操作记录开始和结束时间。
- 运行、外部 API 调用和验证必须记录命令/接口、结果或退出状态、artifact/run ID 与是否触及数据库。
- 历史记录只能使用可证实的时间：Git author time、artifact/run ID、报告字段或文件元数据。无法证实精确时间时写 `exact time unavailable`，绝不猜测。
- API Key、完整认证头、原始敏感数据不得写入日志。
