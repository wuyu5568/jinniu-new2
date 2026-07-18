# P0 冒烟测试（curl）

前置：MySQL 已导入 `scripts/schema.sql`，服务已启动，配置中 `admin_username`/`admin_password` / `genesis_address` 与下文一致。

```text
BASE=http://127.0.0.1:8000
ADMIN_KEY=change-me-admin-key
GENESIS=0x9ddef22beb04103ae3726807cd8af247bf2b8bf6
```

## 1. 健康检查

```bash
curl -s "$BASE/health"
```

## 2. 登录挑战（需真实钱包签名）

```bash
curl -s -X POST "$BASE/api/app_server/v1/login/challenge" \
  -H "Content-Type: application/json" \
  -d "{\"address\":\"$GENESIS\"}"
```

响应含 `message`、`nonce`、`expires_at`。待签名文案示例：

```text
Jinniu login
nonce: <nonce>
```

### 用 Foundry cast 签名（示例）

```bash
MSG='Jinniu login\nnonce: YOUR_NONCE'
SIG=$(cast wallet sign --private-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 "$MSG")
```

Hardhat 默认账户 #0 私钥仅用于本地测试。

## 3. 登录验证

```bash
curl -s -X POST "$BASE/api/app_server/v1/login/verify" \
  -H "Content-Type: application/json" \
  -d "{
    \"address\":\"$GENESIS\",
    \"signature\":\"$SIG\",
    \"nonce\":\"YOUR_NONCE\",
    \"invite_code\":\"\"
  }"
```

保存返回的 `token`。

## 4. 检查地址是否已注册

```bash
curl -s "$BASE/api/app_server/v1/auth/check?address=$GENESIS"
```

## 5. 资料

```bash
curl -s "$BASE/api/app_server/v1/me" -H "Authorization: Bearer $TOKEN"
```

## 6. 管理端充值

```bash
curl -s -X POST "$BASE/api/admin_jinniu/v1/users/recharge" \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -d "{\"address\":\"$GENESIS\",\"amount\":\"1000\"}"
```

## 7. 认购

```bash
curl -s -X POST "$BASE/api/app_server/v1/locations" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"amount":"500"}'
```

记录返回的 `id` 作为 `LOCATION_ID`。

## 8. 列表认购订单

```bash
curl -s "$BASE/api/app_server/v1/locations" -H "Authorization: Bearer $TOKEN"
```

## 9. 手动结算（静态 → 代数 → 社区 V → 社区基础奖+小区同级平级）

```bash
curl -s -X POST "$BASE/api/admin_jinniu/v1/settle/run" \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -d '{}'
```

可选指定订单：`{"order_ids":[1]}`

## 10. 账本流水

```bash
curl -s "$BASE/api/app_server/v1/ledger" -H "Authorization: Bearer $TOKEN"
```

## 11. 提取资产申请

```bash
curl -s -X POST "$BASE/api/app_server/v1/withdraws" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"amount\":\"10\",\"order_ids\":[$LOCATION_ID]}"
```

## 12. 管理端审批 / 拒绝

```bash
# 列表
curl -s "$BASE/api/admin_jinniu/v1/withdraws" -H "X-Admin-Key: $ADMIN_KEY"

# 通过
curl -s -X POST "$BASE/api/admin_jinniu/v1/withdraws/1/approve" \
  -H "X-Admin-Key: $ADMIN_KEY"

# 拒绝（可选 remark）
curl -s -X POST "$BASE/api/admin_jinniu/v1/withdraws/1/reject" \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -d '{"remark":"test reject"}'
```

## 13. 业务参数

```bash
curl -s "$BASE/api/admin_jinniu/v1/business_configs" -H "X-Admin-Key: $ADMIN_KEY"

curl -s -X PUT "$BASE/api/admin_jinniu/v1/business_configs/1" \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -d '{"value":"0.06"}'
```

## 邀请注册流程

1. 创世账号先登录（无 `invite_code`）
2. 新用户 challenge → 签名 → verify，body 中 `invite_code` 填创世地址
3. 继续充值 / 认购 / 结算，验证代数奖与社区奖

## 说明

- 登录 **必须** 提供有效 `personal_sign`；无 DEV 跳过开关。
- 列表接口（locations、withdraws、ledger、configs）响应为裸 JSON 数组。
- 提取手续费默认 6%，申请时立即扣可提余额。
