Copyright (c) 2026 kanomahoro. 本项目以 MIT 协议开源，全文见 [LICENSE](LICENSE)。

> 仅供**学习与技术研究**（网络协议、服务端架构、数据库设计）参考，请勿用于商业运营或任何侵犯他人权利的场景。
> 游戏名称、商标、美术与音频等素材的权利归原权利人所有；部分协议行为与数据库结构参考
> [K1roSama/BubbleFighter_CN0261_Server](https://github.com/K1roSama/BubbleFighter_CN0261_Server)（MIT，Copyright (c) 2026 Albert Gong）。

# Bubble Fighter CN0261(cn-717) Server (Go)

[![MIT License](https://img.shields.io/badge/License-MIT-D22128?style=flat-square)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go)
![Version](https://img.shields.io/badge/version-v0.9.18-blue?style=flat-square)

Bubble Fighter CN0261(cn-717) 服务端的 Go 重写版。单二进制、无 cgo，
协议与数据库结构与 [K1roSama/BubbleFighter_CN0261_Server](https://github.com/K1roSama/BubbleFighter_CN0261_Server)（C++ 原版）保持一致。

仓库：<https://github.com/kanomahoro/bf-cn0261-go-server>

当前版本 **v0.9.18**：竞技（1）、道具战（3）、僵尸（8）、狙击（12）、热血个人（13）、热血团队（14）、热血狙击（18）、爆破（19）、队长战（20）、海盗（101），外加单人教程（25 关）；均已跑通「登录 → 广场 → 建房 → 对局 → 结算落库 → 回房」完整闭环（海盗模式的单人局与双人局均已在真机验收）。客户端的 11 个可选模式里只剩僵尸 BOSS（16）未做。

> 详细变更一律以 [`CHANGELOG.md`](CHANGELOG.md) 为准，本文件只列里程碑。版本号在
> `cmd/bfserver/main.go` 的 `Version`，出号时与 README badge、CHANGELOG 三处一起改。

- **v0.9.18** 海盗多人收尾：双人结算（`906` 广播全房）、结算序列对齐参考实现（不发 `S732`、`S733` 前重发 `S729`）、
  结算面板计分（泡封 / 击爆三条路径 / 营救）、随机图回房间的卡片闪退修复；日志前缀与级别统一（见 `docs/logging.md`）
- **v0.9.0** 单人教程（25 关）入版、海盗模式（101）落地（首次实现，待真机）；修掉爆破整场结算掉线、爆破地图没有弹药箱、狙击射爆泡封不计分；
  日志级别按 `docs/logging.md` 重新梳理；单元测试移出版本库
- **v0.8.5** relay 改动收敛到爆破：预留期回到 2 分钟、同 key 接管只对标记可复用的 key 生效
- **v0.8.x** 爆破真机迭代：小局之间只发 `S705`（横幅与 HUD 队伍比分确认无法安全实现，定稿放弃）；
  复活预算改为每小局 2 次；relay 预留期与接管收敛到只在爆破生效
- **v0.7.x** 爆破小局重开：客户端重载场景会拆 relay，服务端改为宽限 20 秒不释放座位；
  观察者（`S974`）、爆炸先于时钟的判定、地图池校验（6 张，含两张只属于爆破的图）
- **v0.6.0** 爆破（19）落地：三局两胜、每小局攻守互换、炸弹装机/拆弹/爆炸状态机（2012/2011 → 973）、
  死亡转观察者（S974）、小局之间只重载场景不重进房间、整场只结算一次。规则见 [`BOMB.md`](BOMB.md)
- **v0.5.0** 队长战（20）真机验收通过：开局点名（S982）、阵亡转交（泡封队友不接任）、
  队长 ULTRA（C707 无对象 + kind 104 → S715）；Relay 注册窗口放宽到 60 秒；
  管理后台修复日志/对局记录翻页与钱包负数提示。规则见 [`CAPTAIN.md`](CAPTAIN.md)
- **v0.4.0** 狙击 / 热血个人 / 热血团队 / 热血狙击四种模式、管理后台完善（翻页、钱包核实、战绩布局）、
  弹药箱扩到狙击与热血、复活点轮换、感应地雷 id 修正、广场幽灵座位修复
- **v0.3.2** 管理控制台、房间密码、重复登录接管、广场幽灵座位与退房并发修复
- **v0.3.1** 仓库合并与日志/文档整理，**无协议与玩法改动**
- **v0.3.0** 僵尸模式（真机验收定稿，规则见 [`ZOMBIE.md`](ZOMBIE.md)）
- **v0.2.0** 道具战；更早的变更见 [`CHANGELOG.md`](CHANGELOG.md)

## 模式一览

客户端一共只提供 11 个可选模式（多出来的那些是别的版本才有的，**不要拿参考实现的配置当客户端能力证据**）。
「加入版本」是这套服务端里第一次能开局并落库的版本；**GitHub 主线 `main` 与最新 tag 都是 `v0.9.18`**，
下表中已完成的模式全部随主线发布。

| 模式 | 名称 | 服务端负责的部分 | 加入版本 | 状态 | 文档 |
| --- | --- | --- | --- | --- | --- |
| 1 | 竞技 | 两队、比分上限、泡封—营救 | v0.1.x | 稳定 · 已发布 | — |
| 3 | 道具战 | 15 种道具箱的创建/拾取/使用/切槽/投掷、红蓝归属、两槽记账 | v0.2.0 | 稳定 · 已发布 | — |
| 8 | 僵尸（选王，180 s） | 选王、死亡转化、王死判人类胜、无限复活、按机型实测的变身延迟 | v0.3.0 | 定稿 · 已发布 | [`ZOMBIE.md`](ZOMBIE.md) |
| 12 | 狙击 | 同竞技 + 开局强制狙击枪 + 弹药箱；射爆泡封计分 | v0.4.0 | 稳定 · 已发布 | — |
| 13 | 热血个人 | 无队伍、全员平局结算、退人到只剩一人也结算、出生点客户端权威 | v0.4.0 | 稳定 · 已发布 | — |
| 14 | 热血团队 | 去掉泡封—营救的竞技，最后一击直接死亡并计分 | v0.4.0 | 稳定 · 已发布 | — |
| 18 | 热血狙击 | 同 14 + 强制狙击枪 + 弹药箱 | v0.4.0 | 稳定 · 已发布 | — |
| 20 | 队长战 | 开局点名（S982）、阵亡转交（泡封中不接任）、队长 ULTRA | v0.5.0 | 定稿 · 已发布 | [`CAPTAIN.md`](CAPTAIN.md) |
| 19 | 爆破 | 三局两胜、攻守固定（本客户端不换边）、炸弹状态机、每小局 2 次复活、观察者 | v0.6.0 | **定稿 · 已发布** | [`BOMB.md`](BOMB.md) |
| 15 / 103 | 教程（编号关 / prologue） | 25 关单人关卡、NPC 波次、计数与结算，**不建房、不落库** | v0.9.0 | 定稿 · 已发布（真机通过） | [`TUTORIAL.md`](TUTORIAL.md) |
| 101 | 海盗（wgtHorde） | 33 张 npc_* 图、1–4 人合作、房主跑波次与刷怪、服务端转发与结算（不发奖励）、弹药箱、结算面板计分（泡封/击爆/营救） | v0.9.0 | **单人+双人真机通过 · 已发布**（v0.9.18） | [`PIRATE.md`](PIRATE.md) |
| 16 | 僵尸 BOSS（PVE 救援） | — | 未做 | 待做 | — |

逐条修复（现象 / 根因 / 改法 / 真机结论）记在 [`CHANGELOG.md`](CHANGELOG.md)，最新是 **v0.9.18**；
本表只列模式与里程碑，不重复细节。

## 功能范围

**已实现**

| 模块 | 内容 |
| --- | --- |
| 登录与角色 | 票据登录、建角、昵称校验、角色外观 |
| 大厅 | 广场（每实例 16 人）、房间列表、建房/进房/换队/准备/踢人/聊天 |
| 对局 | 加载门控、防御种子表、倒计时、生命状态机、复活、比分、结算 |
| 模式 | 竞技（1）、道具战（3）、僵尸（8）、狙击（12）、热血个人（13）、热血团队（14）、热血狙击（18）、爆破（19）、队长战（20）、海盗（101） |
| 单人教程 | 落地页「教程」→ 选关（25 关全开）→ 编号关卡（mode 15）/ prologue（mode 103），不建房间 |
| 结算 | 每局写 `game_results` / `game_result_players` / `game_result_awards`，经验与 Lucci 落库 |
| 商店与背包 | 目录、购买（Lucci/点卷）、装备、武器预设、期限道具过期清理 |
| 认证端 | 登录器自助注册（邀请码 + 按 IP 限流）、一次性登录票据 |
| 中继 | 对局内 peer 转发（26104），按预约 key 与源 IP 绑定，只转发白名单内的对局内状态包；成员中继不可用时回退走游戏连接 |
| 补给 | 道具箱与弹药箱的点位上报、按间隔刷新、拾取广播（道具战、僵尸、狙击、热血、爆破、海盗） |
| 管理后台 | 概览、在线玩家、房间、日志、战绩、公告、改钱包、封禁，列表均支持翻页（默认关闭） |

**未实现**：僵尸 BOSS（16）这一个客户端可选模式，以及客户端根本不提供的
夺星 5/6、AOS 22 等（**不要拿参考实现的配置当客户端能力证据**）、
多大厅频道、服务端反作弊（战斗判定沿用客户端上报，与 2015 原版一致）。

## 环境要求

- **数据库：MariaDB 10.11**（只在 MariaDB 上验证过，MySQL 未测试）
- **Go 1.27+**（仅构建需要，运行只要二进制）
- **客户端：Bubble Fighter CN0261(cn-717)**（BubbleFighter 2015 国服版）—— 需自备，本仓库不提供
- 依赖只有两个：`go-sql-driver/mysql`、`golang.org/x/crypto/scrypt`
- 验证环境：Debian 12 x86_64 / MariaDB 10.11.19 / 2 vCPU 2 GB（对局与压测均在此规格下跑通）

## 一键安装（Debian 12+）

需要一台全新的 Debian 12（或更新）服务器，root 权限。三步：

```bash
# 1. 更新系统，装好下载工具
sudo apt update && sudo apt -y upgrade
sudo apt install -y wget curl

# 2. 拉取 main 分支上最新的安装脚本
wget -O bfserver-install.sh \
  https://raw.githubusercontent.com/kanomahoro/bf-cn0261-go-server/main/tools/bfserver-install.sh

# 3. 运行（交互式：会问你公网 IP 和注册邀请码，回车即用默认值）
sudo bash bfserver-install.sh
```

没有 `wget` 就用 `curl -fsSL -o bfserver-install.sh <同一个地址>`。
脚本里写死了版本号和二进制的 sha256，所以从 `main` 拉脚本拿到的是**当前发布版**的安装器：
它会去 Release 下载对应 `vX.Y.Z` 的资产并校验，不会因为 main 上多一个提交就装上未经发布的东西。

八步全自动：检查系统 → 装 MariaDB → 建库建号 → 从 GitHub 下载发布版 → 跑完 14 份迁移 →
生成配置与 26102 自签证书 → 注册 systemd 服务并启动 → 自检三个端口。

- 公网 IP 自动探测（探到内网会提醒）、邀请码随机、数据库密码随机 24 位（不打印只落盘）
- 装完会提示：三个端口要放行、以及**管理后台默认关闭**和怎么开启（见下文「管理后台（可选）」）
- 其他用法：`--dry-run`（只看会做什么，不改系统）、`--yes`（全自动）、`--uninstall`（卸载，库默认保留）
- 安装目录 `/opt/bfserver`，服务名 `bfserver`，日志 `Logs/Server.log`，密码与邀请码在 `/root/.bfserver-secrets`
- **云主机要放行 26100 / 26102 / 26104**，只放行 26100 会出现「能登录、进对局转圈」

> 安装脚本只从**仓库**（main 或某个 tag）取，**不随 Release 发布**：Release 里只放二进制包和
> `SHA256SUMS`。脚本里锁定版本与 sha256，所以从 main 拿到的始终是「当前发布版」的安装器。
> 完整发布流程见 [`docs/release.md`](docs/release.md)。
> 想固定装某个旧版本：把上面 `main` 换成对应 tag（如 `v0.4.0`）即可。

手动安装、数据库准备与登录器打包：见 [`docs/manual-install.md`](docs/manual-install.md)。

## 构建

```bash
go build -o bin/bfserver ./cmd/bfserver

# 发布构建（-s -w 不能省：否则 exe 多出约 4 MB 符号表与 DWARF）
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w" -o bfserver ./cmd/bfserver
./bfserver --version
# BubbleFighter CN0261 Go Server v0.9.18  By:rurudoo

# 版本号写在 cmd/bfserver/main.go 的 Version 变量里；需要出号时再改，
# 不要用 -X main.Version 在命令行覆盖（会与 CHANGELOG/发布包不一致）
```

发布树里不带单元测试（`go build` 也不会把它们编进二进制）。需要回归测试时从历史取回：

```bash
git checkout 440570a -- internal/game/*_test.go && go test ./internal/game
```

## 端口

| 端口 | 用途 | 协议 |
| --- | --- | --- |
| 26100 | 游戏/大厅 | 自定义 Lambda 加密封包 |
| 26102 | 登录器认证 | TLS + 长度前缀 JSON |
| 26104 | 对局中继 | 自定义 TCP + 预约 key |
| 26110 | 管理后台（**可选，默认不监听**） | HTTP + Bearer token |

三个端口都要对客户端可达；只开 26100/26102 能登录，但进对局会因中继连不上而失败。

## 配置

默认配置 `config/server.json`，`--check-config` 可在不连数据库的情况下校验。

| 段 | 关键字段 | 说明 |
| --- | --- | --- |
| 顶层 | `gameMapsFile` 等 6 个 | 数据表路径，指向 `config/*.json` |
| `network` | `listenPort` 26100 | 游戏端口、连接数、空闲超时 |
| `relay` | `listenPort` 26104、`advertisedAddress` | 中继端口，**必须填公网 IP**，否则对局连不上 |
| `database` | `host`/`port`/`databaseName`/`username`/`password` | MariaDB 连接；库名只允许 `bubblefighter_cn0261(_tests)` |
| `authentication` | `listenPort` 26102、`inviteCode`、限流 | 登录器认证；证书用 `./gen-cert.sh` 生成 |
| `logging` | `level`、轮转 | `trace/debug/info/warn/error`；默认 `info`，一局只写几行骨架 |
| `zombie` | `humanTurnMs` 12000、`zombiniTurnMs` 18000 | 换队通知的等待（真机量出来的），范围 500–60000，整段可省略 |

字段级说明、日志级别约定与各段默认值：见 [`docs/configuration.md`](docs/configuration.md)

管理后台（概览、在线玩家、房间、日志、战绩、公告、改钱包）：**默认关闭**，
开启方法与安全说明见 [`docs/admin.md`](docs/admin.md)。

## 管理后台（可选）

仓库里的 `config/server.json` 带的是 `"enabled": false` 和**空 token**——**默认不监听任何端口**。
这是刻意的：后台 token 一旦进了公开提交历史就等于泄露，不应该“开箱即开”地被扫到。

开启四步：

```bash
# 1. 生成一个 token
openssl rand -hex 32

# 2. 编辑 config/server.json 的 admin 段：enabled 改 true，token 填上一步的值
#    "admin": { "enabled": true, "listenAddress": "127.0.0.1", "listenPort": 26110, "token": "***" }

# 3. 重启
systemctl restart bfserver

# 4. 本机访问（远程用 SSH 隧道，不要把 listenAddress 改成 0.0.0.0）
ssh -L 26110:127.0.0.1:26110 你的用户@服务器
# 然后浏览器开 http://127.0.0.1:26110/
```

- `enabled` 为 `true` 而 `token` 不合规（少于 16 字符 / 占位值）时，**服务端直接拒绝启动**
- 连续 10 次 token 错误，该地址会被挡 1 分钟（HTTP 429）
- 绑定成功时服务端会在日志和控制台打印 `Admin console : http://127.0.0.1:26110/`
- 功能清单、接口与权限说明：见 [`docs/admin.md`](docs/admin.md)

## 日志

默认 `info`，文件在 `Logs/Server.log`（5 MB 自动轮转，保留 3 份）。一局通常只写十几行，
适合直接贴给作者定位问题。

- **级别**：`info` 只有开局/结算/退房这类骨架（每局个位数）；`warn` 是**预期外的降级**但服务端已兜底、
  玩家基本无感；`debug` 才是逐包逐帧细节
- **子系统前缀**（`Info`/`Warn`/`Error` 一律带）：`[Application]` `[Database]` `[Round]` `[Room]` `[Plaza]`
  `[Channel]` `[Session]` `[Login]` `[Authentication]` `[Relay]` `[Supplies]` `[Defense]` `[Zombie]`
  `[Bomb]` `[Pirate]` `[Tutorial]` `[Captain]` `[Shop]` `[Admin]`
- **该 grep 哪几句**：
  - `[Round] started` / `[Round] ended` —— 一局的起止（带比分、结束原因）
  - `[Relay] transport lost` / `[Relay] registration timeout` —— 中继链路出问题
  - `loading timed out` —— 有人卡在加载，回合被取消（`[Round]` / `[Bomb]` 两种前缀）
  - `[Round] settlement clamped` —— 满级或钱包到上限，属正常
  - `[Round] result packet rejected` —— 结算包被拒，通常是角色数据异常
  - `[Defense] seeds forced without every character report` —— 有座位没报角色，防御种子被迫提前下发

分级原则（哪些事件走 Info、哪些只能 Debug）见 [`docs/logging.md`](docs/logging.md)；
各段默认值与轮转行为见 [`docs/configuration.md`](docs/configuration.md)。

> 排查计分问题时把级别调到 `debug`，`peer life` / `peer kick` 两行会打印每个攻击事件的
> `kind` / `accepted` / `victimState` / `direct` / `special` —— 这是判断「这刀算没算分」的依据。

## 命令行参数

| 参数 | 说明 |
| --- | --- |
| `--version` | 打印版本并退出（不需要配置文件） |
| `--config <path>` | 配置文件，默认 `config/server.json` |
| `--check-config` | 只校验配置与数据表，不连数据库 |
| `--create-account <name>` | 建号；配 `--nickname` 同时建角色 |
| `--issue-ticket <name>` | 签发一次性登录票据（120 秒有效、一次性） |
| `--nickname` / `--character-type` / `--password` | 配合建号使用，密码也可给 `BF_ACCOUNT_PASSWORD` |

## 数据库

- 迁移 14 份，**必须用 `database/migrate.sh`**（按版本号排序；直接 `cat *.sql` 会撞外键）
- 表按用途分组：账号与角色、房间与对局（`game_results*`）、商店与背包、聊天与社交
- 详细表结构、初始化校验与手工建库步骤：见 [`docs/database.md`](docs/database.md)

## 目录结构

```
cmd/bfserver/      入口（参数解析、启动、账号工具）
internal/config/   配置加载与校验
internal/game/     房间、对局、生命状态机、道具补给、僵尸模式
internal/net/      游戏连接与中继
internal/protocol/ 封包编解码
internal/store/    持久化
config/            默认配置与数据表（地图、武器、防御、角色、复活、成长）
database/          迁移脚本
docs/              细节文档（手动安装、配置、数据库、管理后台、实现说明、发布、日志约定）
launcher/          登录器（Python）
tools/             一键安装脚本
ZOMBIE.md          僵尸模式规则与真机验收记录
CAPTAIN.md         队长战（mode 20）规则、协议证据与真机验收记录
BOMB.md            爆破模式（mode 19）规则、协议证据与真机验收记录
TUTORIAL.md        教程模式（单人 25 关）设计稿、协议与真机验收记录
PIRATE.md          海盗模式（mode 101）方案、协议清单与待审项
```

## 已知限制

- 模式白名单只有竞技（1）、道具战（3）、僵尸（8）、狙击（12）、热血个人（13）、热血团队（14）、热血狙击（18）、爆破（19）、队长战（20）、海盗（101），其余房间创建会被拒
- 教程（单人 25 关）走落地页入口，不建房间，规则见 [`TUTORIAL.md`](TUTORIAL.md)
- 道具战与僵尸的规则由客户端保有（与 2015 原版一致），服务端只做状态与结算
- 单大厅频道，无跨频道分流
- 战斗判定依赖客户端上报，无服务端反作弊
- 同一账号可重复登录，但只有第一个能进广场/房间

## 常见问题

| 现象 | 处理 |
| --- | --- |
| `errno: 150 Foreign key constraint is incorrectly formed` | 迁移顺序错了，用 `database/migrate.sh` |
| `Unknown collation: 'utf8mb4_0900_ai_ci'` | 这是 MySQL 8 的排序规则；本项目只支持 MariaDB |
| 启动报 `database schema version mismatch` | `schema_migrations` 不是恰好 1..14，重跑迁移 |
| 启动报找不到 `authentication_certificate.pem` | 跑 `./gen-cert.sh`，或把 `authentication.enabled` 置 false |
| 登录器提示「邀请码不正确」 | 填入与服务端 `authentication.inviteCode` 相同的码；留空表示开放注册 |
| 登录器提示「服务繁忙」 | 触发按 IP 限流（默认 5/小时、20/天），等窗口过去或调配置 |
| 对局中对方定住不动（v0.1.2 及更早） | 同步阻塞写缺陷，升级即可，见 [`docs/design.md`](docs/design.md#发送队列与背压) |
| 复活后和队友叠在一个点（v0.1.2 及更早） | 无复活点轮换，升级即可，见 [`docs/design.md`](docs/design.md#复活点) |
| 满级角色结算显示 +0 经验 | 正常：经验被夹到等级表上限，日志有 `[Round] settlement clamped` |
| 金币到上限不再增加 | 正常：上限 `store.MaxLucci = 100,000,000`（1 亿） |
| 打完一局没有结算界面 | 搜日志 `game result packet rejected`（WARN），多为角色数据异常 |
| 对局里有人的位置不更新、但还能被打 | 他的中继连接没建立起来，搜 `registration timeout`；先确认 **26104** 对客户端可达 |
| 管理后台打不开 | 默认就没有监听：先把 `admin.enabled` 改成 `true` 并自己填 `token`（至少 16 字符），否则服务端会拒绝启动 |

## 发布

打 tag、构建产物、回填 `BIN_SHA256`、上传 Release 资产的完整顺序：见 [`docs/release.md`](docs/release.md)。
Release 资产只包含**二进制包**与 `SHA256SUMS`；安装脚本一直在仓库里（`tools/bfserver-install.sh`），
通过 raw 链接获取，不跟着 Release 发。

## 许可证

MIT，见 [LICENSE](LICENSE)。版权所有 (c) 2026 kanomahoro；
部分协议行为与数据库结构参考 BubbleFighter_CN0261_Server（MIT，Copyright (c) 2026 Albert Gong）。

> 本项目仅供**学习与技术研究**（网络协议、服务端架构、数据库设计）参考，请勿用于商业运营
> 或任何侵犯他人权利的场景。游戏名称、商标、美术与音频等素材的权利归原权利人所有。

## 文档

完整索引见 [`docs/README.md`](docs/README.md)（使用/架构/模式/素材四类）；
各版本改了什么、为什么改，见 [`CHANGELOG.md`](CHANGELOG.md)（最新在最上面）。
