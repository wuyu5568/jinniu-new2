# 演示手册（金牛 new2）

用现成演示种子 + 管理端 / 用户端，约 10 分钟走通主路径。

## 前置

1. MySQL 已建库并导入 `scripts/schema.sql`
2. 后端：`.\bin\app.exe -conf app/app/configs/config.yaml`（默认 `:8000`）
3. 管理端：外部仓 **dapp-admin**（如 `http://localhost:8080/admin/`，账号见 `config.yaml`：`admin` / `admin123`）
4. 用户端：外部仓 **taurus**（如 `:85`）；演示登录用已有钱包或创世地址

创世地址（配置）：`0x9ddef22beb04103ae3726807cd8af247bf2b8bf6`

## 灌演示数据

种子会在创世账号下写入演示树（认购/提取/锁定/V 级/平级拓扑等），**幂等可重复跑**（按地址 upsert）。

```bash
# 有 make
make seed-demo

# Windows 无 make
.\scripts\regress.ps1 -SeedDemo

# 等价
go run scripts/seed_demo_users.go
```

成功末行类似：`SEED DEMO USERS DONE`。失败会打印 `FAIL [expect]` / `FAIL [peer]`（等级或平级拓扑自检未过）。

演示地址形如 `0xd00000…` + 序号（见脚本 `addr(n)`），挂在创世（uid=2）下。

## 管理端看什么（dapp-admin）

| 页 | 看点 |
|----|------|
| 数据统计 | 激活人数、认购/收益 KPI |
| 用户数据 | 搜演示地址；VIP、锁定、停分红、账户/可提 |
| 认购列表 | 进行中 / 已出局订单 |
| 提现列表 | 待审 `pending`；通过后「待打款」`rewarded`；打款默认关 |
| 账本流水 | 静态 / 社区等类型 |
| 账户充值 | 人工充值；「拉链上充值」完成后看摘要（拉取/入账/跳过） |
| 业务参数 | 利率、门槛等（改后立即生效、不回溯） |
| 健康检查 | `GET /health` 应含 `"db":"ok"`；看板/`settle_status` 有今日日结字段 |
| 手动日结 | 同日第二次应 `already_settled`；`force` 仅本地 `allow_force_settle: true`；首页会显示今日是否已结 |

## 用户端看什么（taurus）

侧栏主路径：首页 / 充值 / 认购 / 团队 / 资产 / 提现 / 奖励记录。

| 页 | 看点 |
|----|------|
| 认购 | 档位、下单（需账户余额） |
| 团队 | 直推树 |
| 资产 / 奖励 | 流水 `list` |
| 提现 | 申请、待审可取消、列表状态（待审/待打款/已打款） |
| 充值记录 | 账户充值流水 |

未接页（商城/转账等）已重定向首页。

## 建议演示顺序（口述）

1. 跑 `seed-demo` → 管理端用户列表能搜到演示账号  
2. 看板 KPI 非空 → 认购列表有进行中单  
3. 提现列表审一笔 pending → 应变为「待打款」（`rewarded`）；打款默认关，点「打款」应报 `BIZ_PAYOUT_DISABLED`  
4. 点日结（无 force）→ 已结则跳过  
5. taurus 登录创世或已注册演示钱包 → 看订单 / 团队 / 收益  

## 注意

- 种子**不**手写社区等级，按认购业绩重算；改门槛后需重跑种子或日结刷新等级  
- 生产务必 `allow_force_settle: false`；`payout_enabled` 默认 false，开打款须设 `JINNIU_PAYOUT_MAX_USDT` 与热钱包（见 `docs/ops-payout.md`）
- 链上充值跳过补账：管理端「按序号补账」或 `POST /deposit_replay`
- 回归冒烟：`.\scripts\regress.ps1 -All`（需服务已启动）  
