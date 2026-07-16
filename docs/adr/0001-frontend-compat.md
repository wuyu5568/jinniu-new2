# ADR 0001: 前端兼容层（taurus + dapp-admin）

## 状态

已接受（2026-07-13）

## 背景

金牛（jinniu）后端在 `new2` 仓库独立演进，用户端与管理端前端仍分别维护于：

- `taurus` — 钱包 DApp（原 `/api/app_server` 风格）
- `dapp-admin` — Vue 管理后台（原 `/api/admin_dhb` 风格）

需要在**后端增加兼容层**、前端做**最小改动**即可对接。

## 决策

### 1. 用户登录：`eth_authorize` 无 nonce

- 路径：`POST /api/app_server/eth_authorize`
- 签名字段：`sign`（兼容 `signature`）
- 邀请码字段：`code`（钱包地址）
- 签名消息：对**钱包地址字符串本身**做 `personal_sign`（与 taurus `ETH.signMessage(account)` 一致）
- **不使用** nonce/challenge；保留 `/v1/login/challenge` + `/v1/login/verify` 供新客户端选用

### 2. 管理端：用户名密码 + JWT

- 移除 `auth.admin_key` 与 `X-Admin-Key` 中间件
- 新增 `auth.admin_username` / `auth.admin_password`（MVP 明文配置）
- `POST /api/admin_jinniu/login` → `{ token }`
- 管理 JWT claims：`role=admin`；与用户 JWT（`uid` claim）分离

### 3. API 前缀

| 端 | 前缀 | 说明 |
|----|------|------|
| 用户 | `/api/app_server/...` | 无 `/v1/` 的 compat 路由 + 可选 `/v1/` REST |
| 管理 | `/api/admin_jinniu/...` | 替换原 `admin_dhb` |

### 4. 认购模式

- 余额认购：扣减 `account_balance`，不再依赖链上 BUY 合约
- taurus `/node` 调用 `POST app_server/buy`；链上充值页标记不可用

### 5. 前端仓库外置

- 不在 `new2` 内嵌前端；仅文档与 compat 路由约定
- 本地联调默认 `http://127.0.0.1:8000`

## 后果

- _positive_：taurus / dapp-admin 可快速对接现有后端
- _positive_：v1 REST 与管理 JWT 统一，便于后续收敛
- _negative_：compat 响应字段大量填默认值，部分管理端操作（锁定、改级别等）尚未实现
- _negative_：仪表盘 KPI（今日数据等）为简化占位

## 参考

- `app/app/internal/service/compat.go` — compat 处理器实现
- `app/app/internal/server/http.go` — 路由注册
