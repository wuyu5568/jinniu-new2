# ADR 0019: 账本时间按北京墙钟存储与展示

**Status**: accepted

生产原为 UTC 墙钟写入 `DATETIME`。改为：一次性将既有 `DATETIME` +8 小时；EC2 时区 `Asia/Shanghai`；DSN `loc=Asia/Shanghai` 且会话 `time_zone='+08:00'`。`settle_runs.settle_date`（上海自然日）与 `TIMESTAMP` 列不加 8 小时。此后接口 `createdAt` 等与库内墙钟均为北京时间。
