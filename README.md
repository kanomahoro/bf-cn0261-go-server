Copyright (c) 2026 kanomahoro. 本项目以 MIT 协议开源，全文见 [LICENSE](LICENSE)。

> 仅供**学习与技术研究**（网络协议、服务端架构、数据库设计）参考，请勿用于商业运营或任何侵犯他人权利的场景。
> 游戏名称、商标、美术与音频等素材的权利归原权利人所有。
>
> 部分协议行为与数据库结构参考 [K1roSama/BubbleFighter_CN0261_Server](https://github.com/K1roSama/BubbleFighter_CN0261_Server)（C++ 原版，MIT）；
> 中继语义、包表与部分模式规则参考 [Wasabi-FantasyTaleDev/BubbleFighter.Fantasy](https://github.com/Wasabi-FantasyTaleDev/BubbleFighter.Fantasy)（同为 cn-717 的 C# 实现，MIT）。
> 两处参考都只取协议事实与行为约定，没有引入其代码。

# Bubble Fighter CN0261(cn-717) Server (Go)

[![MIT License](https://img.shields.io/badge/License-MIT-D22128?style=flat-square)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go)
![Version](https://img.shields.io/badge/version-v1.2.0-blue?style=flat-square)

Bubble Fighter CN0261(cn-717) 服务端的 Go 重写版：单二进制、无 cgo，协议与数据库结构对齐 C++ 原版，
中继与部分模式的网络行为对齐 `BubbleFighter.Fantasy`。
仓库：<https://github.com/kanomahoro/bf-cn0261-go-server>

当前版本 **v1.2.0**：客户端 11 个可选模式全部实现，外加单人教程（25 关），
均已跑通「登录 → 广场 → 建房 → 对局 → 结算落库 → 回房」完整闭环。
本版把普通模式房间扩到 **12 座**（海盗 101 / 僵尸 BOSS 16 仍是 4 座）、击杀上限改为服务端自定的 **40**，
并修掉广场换装不刷新。逐条变更以 [`CHANGELOG.md`](CHANGELOG.md) 为准。

- **v1.2.0** 十二座 / 击杀上限 40 / 僵尸局不吃击杀上限 / 爆破中继宽限 60 s / 广场换装即时可见；
  老库需手工把结算座位的 `CHECK` 放宽到 `slot < 12`（见 [`database/README.md`](database/README.md)）
- **v1.1.0** 单一数据库基线 + 表结构校验；中继改为参考实现的透传策略（≥`0x8000` 原样转发）
- **v1.0.0** 11 个模式全部真机验收，发布第一个正式版；更早的里程碑见 [`CHANGELOG.md`](CHANGELOG.md)

## 模式一览

客户端一共只提供 11 个可选模式（多出来的那些是别的版本才有的，**不要拿参考实现的配置当客户端能力证据**）。

| 模式 | 名称 | 服务端负责的部分 | 加入版本 | 状态 | 文档 |
| --- | --- | --- | --- | --- | --- |
| 1 | 竞技 | 两队、比分上限 40（服务端定）、泡封—营救 | v0.1.x | 稳定 · 已发布 | — |
| 3 | 道具战 | 15 种道具箱的创建/拾取/使用/切槽/投掷、红蓝归属、两槽记账 | v0.2.0 | 稳定 · 已发布 | — |
| 8 | 僵尸（选王，180 s） | 选王、死亡转化、王死判人类胜、无限复活、按机型实测的变身延迟 | v0.3.0 | 定稿 · 已发布 | [`ZOMBIE.md`](ZOMBIE.md) |
| 12 | 狙击 | 同竞技 + 开局强制狙击枪 + 弹药箱；射爆泡封计分 | v0.4.0 | 稳定 · 已发布 | — |
| 13 | 热血个人 | 无队伍、全员平局结算、退人到只剩一人也结算、出生点客户端权威 | v0.4.0 | 稳定 · 已发布 | — |
| 14 | 热血团队 | 去掉泡封—营救的竞技，最后一击直接死亡并计分 | v0.4.0 | 稳定 · 已发布 | — |
| 18 | 热血狙击 | 同 14 + 强制狙击枪 + 弹药箱 | v0.4.0 | 稳定 · 已发布 | — |
| 20 | 队长战 | 开局点名（S982）、阵亡转交（泡封中不接任）、队长 ULTRA | v0.5.0 | 定稿 · 已发布 | [`CAPTAIN.md`](CAPTAIN.md) |
| 19 | 爆破 | 三局两胜、攻守固定（本客户端不换边）、炸弹状态机、每小局 2 次复活、观察者 | v0.6.0 | 定稿 · 已发布 | [`BOMB.md`](BOMB.md) |
| 15 / 103 | 教程（编号关 / prologue） | 25 关单人关卡、NPC 波次、计数与结算，**不建房、不落库** | v0.9.0 | 定稿 · 已发布 | [`TUTORIAL.md`](TUTORIAL.md) |
| 101 | 海盗（wgtHorde） | 33 张 npc_* 图、1–4 人合作、房主跑波次与刷怪、服务端转发与结算、弹药箱、计分 | v0.9.0 | 单人+双人真机通过 | [`PIRATE.md`](PIRATE.md) |
| 16 | 僵尸袭击 BOSS 战 | 7 张 `rel_zombie*` 图、1–4 人合作、救援任务闹钟、PVE 点数、三格计数道具、感染转队 | v0.9.19 | 单人+双人真机通过 | [`PVEBOSS.md`](PVEBOSS.md) |

## 功能范围

| 模块 | 内容 |
| --- | --- |
| 登录与角色 | 票据登录、建角、昵称校验、角色外观 |
| 大厅 | 广场（每实例 16 人）、房间 **12 座**（海盗 / 僵尸 BOSS 4 座）、房间列表、建房/进房/换队/准备/踢人/聊天 |
| 对局 | 加载门控、防御种子表、倒计时、生命状态机、复活、比分、结算 |
| 结算 | 每局写 `game_results` / `game_result_players` / `game_result_awards`，经验与 Lucci 落库 |
| 商店与背包 | 目录、购买（Lucci/点卷）、装备、武器预设、期限道具过期清理 |
| 认证端 | 登录器自助注册（邀请码 + 按 IP 限流）、一次性登录票据 |
| 中继 | 对局内 peer 转发（26104），按预约 key 与源 IP 绑定；≥`0x8000` 的帧原样转发，只吃复活请求与延迟探测 |
| 补给 | 道具箱与弹药箱的点位上报、按间隔刷新、拾取广播 |
| 管理后台 | 概览、在线玩家、房间、日志、战绩、公告、改钱包、封禁；列表均支持翻页（默认关闭） |

**未实现**：客户端根本不提供的模式（夺星 5/6、AOS 22 等）、多大厅频道、服务端反作弊
（战斗判定沿用客户端上报，与 2015 原版一致）。

## 环境要求

- **数据库：MariaDB 10.11**（只在 MariaDB 上验证过，MySQL 未测试）
- **Go 1.27+**（仅构建需要，运行只要二进制）
- **客户端：Bubble Fighter CN0261(cn-717)**（2015 国服版）——需自备，本仓库不提供
- 依赖只有两个：`go-sql-driver/mysql`、`golang.org/x/crypto/scrypt`
- 验证环境：Debian 12 x86_64 / MariaDB 10.11.19 / 2 vCPU 2 GB（对局与压测均在此规格跑通）

## 一键安装（Debian 12+）

```bash
sudo apt update && sudo apt install -y wget curl
wget -O bfserver-install.sh \
  https://raw.githubusercontent.com/kanomahoro/bf-cn0261-go-server/main/tools/bfserver-install.sh
sudo bash bfserver-install.sh        # 交互式：问公网 IP 与注册邀请码，回车用默认
```

八步全自动（检查系统 → 装 MariaDB → 建库建号 → 下载并校验发布包 → 导入基线 → 生成配置与 26102 自签证书 →
注册 systemd 服务 → 自检端口）。脚本里写死了版本号与二进制 sha256，所以从 `main` 拿到的始终是**当前发布版**的安装器。

- **更新**：重跑脚本即可——旧 `config/` 备份到 `config.bak.<时间>/`，`server.json` 与证书沿用，
  库已是本版基线则跳过导入、数据保留；旧结构会**在动任何文件之前**停下
- **重建**：`--rebuild-db`（先备份到 `/opt/bf-backup-<库名>-<时间>.sql.gz`，会清空账号与战绩）
- 其它：`--ip <地址>`（NAT 云主机）、`--dry-run`、`--yes`、`--uninstall`、`--offline`（配 `--bin` / `--src` 用本地包）
- 想固定装某个旧版本：把上面地址里的 `main` 换成对应 tag（但**脚本本身建议始终从 `main` 拉**，tag 里的是当时的快照）
- **云主机放行 26100 / 26102 / 26104**；只放 26100 会出现「能登录、进对局转圈」

安装细节、手动安装、登录器打包：见 [`docs/manual-install.md`](docs/manual-install.md)。

## 登录器（可选）

`launcher/` 是一套 Windows 图形登录器（Python 3.9+ / tkinter）：填服务器 IP 与 26102 端口即可注册/登录，
登录成功后自己拉起客户端并把一次性票据写进内存（不落文件）；支持单机多开（互斥体改名）与可选的
高分辨率内存补丁。源码在 [`launcher/`](launcher/)，打包与用法见 [`docs/manual-install.md`](docs/manual-install.md)。

## 构建

```bash
go build -o bin/bfserver ./cmd/bfserver

# 发布构建（-s -w 与 -buildvcs=false 不能省：前者省 4 MB 符号表，后者保证产物可复现）
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w" -o bfserver ./cmd/bfserver
./bfserver --version    # BubbleFighter CN0261 Go Server v1.2.0  By:rurudoo
```

版本号写在 `cmd/bfserver/main.go` 的 `Version`，出号时与 README badge、CHANGELOG、安装脚本四处一起改。
发布树不带单元测试（`*_test.go` 在 `.gitignore`），回归时从历史取回。完整流程见 [`docs/release.md`](docs/release.md)。

## 端口

| 端口 | 用途 | 协议 |
| --- | --- | --- |
| 26100 | 游戏/大厅 | 自定义 Lambda 加密封包 |
| 26102 | 登录器认证 | TLS + 长度前缀 JSON |
| 26104 | 对局中继 | 自定义 TCP + 预约 key |
| 26110 | 管理后台（**可选，默认不监听**） | HTTP + Bearer token |

## 配置

默认 `config/server.json`，`--check-config` 可在不连库的情况下校验。字段级说明见
[`docs/configuration.md`](docs/configuration.md)。

| 段 | 关键字段 | 说明 |
| --- | --- | --- |
| 顶层 | `gameMapsFile` 等 6 个 | 数据表路径，指向 `config/*.json` |
| `network` | `listenPort` 26100 | 游戏端口、连接数、空闲超时 |
| `relay` | `listenPort` 26104、`advertisedAddress` | 中继端口，**必须填公网 IP**，否则对局连不上 |
| `database` | `host` / `port` / `databaseName` / `username` / `password` | MariaDB 连接；结构对不对由启动校验决定 |
| `authentication` | `listenPort` 26102、`inviteCode`、限流 | 证书用 `./gen-cert.sh` 或安装脚本生成 |
| `logging` | `level`、轮转 | `trace/debug/info/warn/error`，默认 `info` |
| `zombie` | `humanTurnMs` / `zombiniTurnMs` | 换队通知等待，默认 12000 / 18000 ms |

## 管理后台（可选）

仓库里的配置是 `"enabled": false` 且 token 为空——**默认不监听任何端口**，因为 token 一旦进公开提交历史就等于泄露。
开启四步与安全说明见 [`docs/admin.md`](docs/admin.md)（`openssl rand -hex 32` 生成 token → 填 `admin` 段 →
重启 → SSH 隧道访问 `127.0.0.1:26110`）。token 不合规时服务端直接拒绝启动。

## 日志

默认 `info`，文件 `Logs/Server.log`（5 MB 轮转、保留 3 份），一局只写十几行骨架。
`warn` 是已兜底的降级，`debug` 才是逐包细节。级别约定、`[子系统]` 前缀表与排查 grep 清单见
[`docs/logging.md`](docs/logging.md)。

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

- **只有一份基线**：`database/baseline.sql`（schema revision 1，22 张业务表 + 资料表数据 + `schema_version`），
  用 `database/migrate.sh` 导入；非空库默认拒绝，`--force` 才重建
- **结构校验**：服务端只比对表结构（表/列/类型/可空/键/自增，`internal/store/schema_expected.go`），
  只改数据（上架/改价/发卡/玩家数据）完全不受影响；改结构要重新生成清单并重打二进制
- ⚠ **CHECK 约束不在比对范围**：例如 `chk_game_result_players_slot CHECK (slot < 12)`，
  放宽它必须手工 `ALTER`，否则运行期写入会被拒
- 改结构流程见 [`database/README.md`](database/README.md)，表结构与手工建库见 [`docs/database.md`](docs/database.md)

## 商店数据

商店内容全部在数据库基线里：HC 五件套 8 套（40 件）、武器 31 件、能量水晶/手镯/推进器/饰品 13 件、
投掷物 7 件（子类 307，按个数卖）、新角色 2 个。新装不用再导任何补丁。
⚠ 武器包有已知定价问题（HC 版与普通版数值相同但更便宜），尚未修正。

## 目录结构

```
cmd/bfserver/      入口（参数解析、启动、账号工具）
internal/config/   配置加载与校验
internal/game/     房间、对局、生命状态机、道具补给、各模式
internal/net/      游戏连接、认证(TLS)与中继
internal/protocol/ 封包编解码
internal/store/    持久化与结构校验
config/            默认配置与数据表（地图、武器、防御、角色、复活、成长）
database/          数据库基线（baseline.sql）与导入脚本（migrate.sh）
docs/              细节文档（安装、配置、数据库、后台、实现、发布、日志）
launcher/          登录器（Python）
tools/             一键安装脚本与结构清单生成
```

## 已知限制

- 模式白名单只有上表 11 个模式 + 教程，其余房间创建会被拒
- 战斗判定依赖客户端上报，服务端不做反作弊；道具战与僵尸的规则同样由客户端保有（与 2015 原版一致）
- 单大厅频道，无跨频道分流
- 同一账号重复登录会**接管**：旧连接被踢下线，不会两条连接共用一个账号
- 个人热血（13）的出生点仍只有 8 个，12 人局会有座位复用
- 广场刷新外观时没做变化检测：购买成功（即使外观没变）也会广播一次相同的座位卡；也没补发位置包

## 常见问题

| 现象 | 处理 |
| --- | --- |
| `errno: 150 Foreign key constraint is incorrectly formed` | 手工按字母序跑了 SQL；用 `database/migrate.sh` 导基线 |
| `Unknown collation: 'utf8mb4_0900_ai_ci'` | 这是 MySQL 8 的排序规则；本项目只支持 MariaDB |
| 启动报 `database structure does not match this build` | 库结构与二进制不一致：按报错补结构，或 `migrate.sh --force` 重建（先备份） |
| 启动报找不到 `authentication_certificate.pem` | 跑 `./gen-cert.sh`，或把 `authentication.enabled` 置 false |
| 登录器提示「邀请码不正确」 | 填与服务端 `authentication.inviteCode` 相同的码；留空表示开放注册 |
| 登录器提示「服务繁忙」 | 触发按 IP 限流（默认 5/小时、20/天），等窗口过去或调配置 |
| 打完一局没有结算界面 | 搜日志 `game result packet rejected`（WARN），多为角色数据异常 |
| 对局里有人的位置不更新、但还能被打 | 他的中继连接没建立起来，搜 `registration timeout`；先确认 **26104** 可达 |
| 管理后台打不开 | 默认没监听：把 `admin.enabled` 改 `true` 并自填 token（≥16 字符） |

## 发布

打 tag、构建、回填 `BIN_SHA256`、上传资产的顺序见 [`docs/release.md`](docs/release.md)；
每个交付包对应的 commit/tag/哈希见 [`docs/releases.md`](docs/releases.md)。
Release 只放二进制包与 `SHA256SUMS`，安装脚本一直在仓库里（raw 链接）。

## 许可证

MIT，见 [LICENSE](LICENSE)。版权所有 (c) 2026 kanomahoro；
另见文件顶部的参考实现与免责声明。
