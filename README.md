# BubbleFighter CN0261 Server (Go)

[![MIT License](https://img.shields.io/badge/License-MIT-D22128?style=flat-square)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go)
![Version](https://img.shields.io/badge/version-v0.1.0-blue?style=flat-square)

泡泡战士 CN0261（国服 2015 版，协议 717）服务端的 Go 重写版。单二进制、无 cgo，
协议与数据库结构与 [K1roSama/BubbleFighter_CN0261_Server](https://github.com/K1roSama/BubbleFighter_CN0261_Server)（C++ 原版）保持一致。

仓库：<https://github.com/kanomahoro/bf-cn0261-go-server>

当前版本 **v0.1.0**，已跑通「登录 → 广场 → 建房 → 对局 → 结算落库 → 回房」完整闭环。

```bash
git clone https://github.com/kanomahoro/bf-cn0261-go-server.git
cd bf-cn0261-go-server
```

> **⚠️ 声明：本项目不提供客户端镜像**
>
> 仓库中只有服务端源码、数据库结构与配置样例，
> **不含** `BubbleFighter.exe`、客户端安装包、游戏美术资源、音频或任何游戏数据文件。
> 想跑起来需要**自备** CN0261（泡泡战士国服 2015 版）客户端。
>
> 本项目仅供**学习与技术研究**（网络协议、服务端架构、数据库设计）参考，
> 请勿用于商业运营或任何侵犯他人权利的场景。使用本项目所产生的法律与合规风险
> 由使用者自行承担。游戏名称、商标、美术与音频等素材的权利归原权利人所有，
> 本项目与原开发方、发行方、运营方均无关联。

## 功能范围

已实现：

- 登录与角色：票据登录、建角、昵称校验、角色外观
- 大厅：广场（每实例 16 人）、房间列表、建房/进房/换队/准备/踢人/聊天
- 对局：加载门控、防御种子表、倒计时、生命状态机、复活、比分、结算
- 结算：每局写 `game_results` / `game_result_players` / `game_result_awards`，经验与 Lucci 落库
- 商店与背包：目录、购买（Lucci/点卷）、装备、武器预设、期限道具过期清理
- 认证端：登录器自助注册（邀请码 + 按 IP 限流）、一次性登录票据
- 中继：对局内 peer 转发（26104），按预约 key 与源 IP 绑定

尚未实现：

- 除竞技模式（mode 1）以外的所有模式（道具战、夺星、僵尸、海盗等）
- 多个大厅频道（当前单频道，见「已知限制」）
- 反作弊（战斗判定采用客户端上报，与原版一致）

## 环境要求

- **数据库：MariaDB 10.11**（本项目只在 MariaDB 上验证过，MySQL 未测试，请勿假定兼容）
- Go **1.27+**（仅构建需要，运行只需二进制）
- 客户端：泡泡战士 CN0261（2015 国服版，协议 717）——**需自备，本仓库不提供**
- 依赖仅两个：`go-sql-driver/mysql`（驱动名如此，实际连的是 MariaDB）、`golang.org/x/crypto/scrypt`

### 开发与测试环境

| 项 | 版本 |
| --- | --- |
| 系统 | Debian GNU/Linux 12 (bookworm)，x86_64 |
| 内核 | 6.1.0-48-cloud-amd64 |
| Go | go1.27.3 linux/amd64 |
| 数据库 | MariaDB 10.11.19（Debian 包） |
| 客户端 | 泡泡战士 CN0261，协议 717 |
| 机器 | 2 vCPU / 2 GB 内存（对局与压测均在此规格下验证） |

> 只有在上述组合上做过完整验证。其他发行版、其他 MariaDB 大版本、MySQL 都可能踩到
> 排序规则、`mysql_native_password` 插件或驱动握手的差异。

## 构建

```bash
go build -o bin/bfserver ./cmd/bfserver

# 发布构建，把版本号写进二进制
go build -ldflags "-X main.Version=0.1.0" -o bin/bfserver ./cmd/bfserver

./bin/bfserver --version
# BubbleFighter CN0261 Go Server v0.1.0  By:rurudoo
```

## 快速开始

### 1. 建库

最省事的是用仓库自带的迁移脚本（它会按每份 SQL 里登记的版本号排序执行，
**不要**按文件名首字母顺序手动跑，外键会失败）：

```bash
BF_DB_USER=root BF_DB_PASSWORD='<root密码>' ./database/migrate.sh
```

也可以手动执行：

```bash
mysql -u root -p < database/migrations/initialize_schema.sql
# 然后按 v2..v14 的顺序执行其余 13 份（版本号写在每份文件末尾的 INSERT 里）
```

迁移完成后 `schema_migrations` 必须正好 14 行：

```sql
SELECT version, name FROM schema_migrations ORDER BY version;
```

建运行账号：

```sql
-- MariaDB：必须显式指定密码插件，否则 Go 驱动握手会失败
CREATE USER 'bfserver'@'127.0.0.1' IDENTIFIED VIA mysql_native_password USING PASSWORD('<密码>');
GRANT SELECT, INSERT, UPDATE, DELETE ON bubblefighter_cn0261.* TO 'bfserver'@'127.0.0.1';
```

### 2. 配置

```bash
cp config/server.json Local/server.json    # Local/ 已在 .gitignore 中排除
```

按部署方式改这几项：

| 配置项 | 本地测试 | 对外开服 |
| --- | --- | --- |
| `network.listenAddress` | `127.0.0.1` | `0.0.0.0` |
| `relay.listenAddress` | `127.0.0.1` | `0.0.0.0` |
| `relay.advertisedAddress` | `127.0.0.1` | 服务器**公网 IP** |
| `authentication.listenAddress` | `127.0.0.1` | `0.0.0.0` |
| `authentication.gameHost` | `127.0.0.1` | 服务器**公网 IP** |
| `authentication.enabled` | `false` | `true`（走登录器注册时） |
| `database.tlsMode` | 数据库没开 TLS 时改 `disable`，同时 `verifyCertificate: false` | 按数据库实际配置 |

数据库密码有两种来源，**配置文件优先**，留空才读环境变量：

```bash
export BF_DATABASE_PASSWORD='<密码>'
```

### 3. 认证端证书（可选）

登录器用 TLS 连 26102，自签证书即可。仓库根目录带生成脚本：

```bash
./gen-cert.sh                                      # localhost + 127.0.0.1，10 年
HOSTS="bf.example.com 203.0.113.10" ./gen-cert.sh  # 追加对外域名/IP
FORCE=1 ./gen-cert.sh                              # 已存在时覆盖
```

脚本会生成 `config/authentication_certificate.pem` 与 `config/authentication_private_key.pem`
（即 `config/server.json` 里 `authentication` 指向的两个文件），私钥权限自动 600。
客户端不校验证书链，CN/SAN 填什么不影响登录。

### 4. 启动

```bash
./bin/bfserver --config Local/server.json --check-config   # 只校验配置，不连库
./bin/bfserver --config Local/server.json                  # 正式启动
```

正常启动的日志：

```
msg="BubbleFighter CN0261 Go Server v0.1.0  By:rurudoo"
msg="[Application] environment" go=go1.27.3 config=Local/server.json database=bfserver@127.0.0.1:3306/bubblefighter_cn0261 gamePort=26100 authPort=26102 relayPort=26104
msg="[Application] ready port=26100 protocol=cn0261-ticket-login"
msg="[Relay] listening" port=26104
msg="[Authentication] listening" port=26102
```

日志同时写控制台和 `Logs/Server.log`（滚动：单文件 5 MB，保留 3 份）。

### 5. 建号

```bash
export BF_ACCOUNT_PASSWORD='<游戏账号密码>'

# 账号 + 角色（同一事务，等价于玩家在登录器里自助注册）
./bin/bfserver --config Local/server.json --create-account lisi --nickname 角色名

# 只建账号，首次登录时由客户端进入建角流程
./bin/bfserver --config Local/server.json --create-account lisi

# 签发一次性登录票据（120 秒有效，一次性）
./bin/bfserver --config Local/server.json --issue-ticket lisi
```

`--nickname` 必须配合 `--create-account`；`--character-type` 仅在给了 `--nickname` 时有效。

**`--character-type` 只接受 `1` 和 `2`**（默认 `1`）——这是原版 `starter_selection` 的语义，
代码里是硬校验（角色类型必须与新手选择一致）。

游戏内建角（C16）则可使用完整角色表，服务端按 `character_appearances`（11 个）挂外观：

| 代码 | 角色 | 外观道具 |
| --- | --- | --- |
| 1 | 皮蛋 | 300001 |
| 2 | 黑妞 | 300002 |
| 3 | 宝宝 | 300003 |
| 4 | 胖墩 | 300004 |
| 5 | 小乖 | 300005 |
| 6 | 小强 | 300006 |
| 8 | 蓝蓝 | 300008 |
| 10 | 蓝妹妹 | 300010 |
| 11 | 雷欧 | 300011 |
| 12 | 拉比 | 300012 |
| 14 | 杰克将军 | 300014 |

> 表里没有 7、9、13：客户端资源本身没有这三个人物的外观道具，迁移里也就没有对应行。

## 登录器（可选）

`launcher/` 是配套的 Windows 登录器源码，用来替代原版启动器：它负责拿票据、注入客户端进程、
以及在同一个机器上开多个客户端。

| 文件 | 作用 |
| --- | --- |
| `launcher.py` | 主界面：服务器地址/端口、账号密码、注册对话框 |
| `start_client.py` | 启动并注入客户端：改单实例互斥体名、搬运登录票据 |

### 运行前提

- **Windows**（`start_client.py` 用 `ctypes` 调 Win32 API）
- **Python 3.9+**，带 tkinter（官方安装包默认包含）
- 登录器文件与 `BubbleFighter.exe` 放在**同一个目录**（改过的启动器要在客户端安装目录里跑）
- 客户端必须是 **CN0261** 原版镜像（**需自备，仓库不提供**）：`start_client.py` 会用 SHA-256 校验，
  并在固定偏移把单实例互斥体名从 `Lambda` 改成 `B261_<实例号>`（A~Z、0~9），镜像不匹配会直接拒绝启动

### 使用

```bash
python launcher.py
```

1. 填服务器 IP 与认证端口（默认 `26102`）
2. 有账号直接登录；没有就点「注册新账号」，填账号/密码/角色名/邀请码
3. 登录成功后自动拉起客户端并带上票据
4. 想多开就再跑一份 `launcher.py`：每个实例的互斥体名不同，互不冲突，
   但要**用不同账号**（同一角色同时进广场会被服务端拒绝）

登录器不保存密码，只在 `%APPDATA%\BubbleFighterCN0261Launcher\config.json`
里记住**服务器地址、端口、账号名**。

### 打包成单文件 exe

```bash
pip install pyinstaller
pyinstaller --noconfirm --onefile --windowed --name BubbleFighterCN0261Launcher launcher.py
```

产物在 `dist/` 下，把它和 `BubbleFighter.exe` 放同一目录即可。

> 说明：登录器只是**客户端侧的便利工具**，没有它也能玩——服务端的认证端（26102）发的是
> 标准的一次性票据，任何能完成「TLS 取票 + 注入客户端」的启动器都可以替代。

## 端口

| 端口 | 用途 | 协议 |
| --- | --- | --- |
| 26100 | 游戏/大厅 | 自定义 Lambda 加密封包 |
| 26102 | 登录器认证 | TLS + 长度前缀 JSON |
| 26104 | 对局中继 | 自定义 TCP + 预约 key |

三个端口都要对客户端可达；只开 26100/26102 能登录，但进对局会因为中继连不上而失败。

## 命令行参数

| 参数 | 说明 |
| --- | --- |
| `--version` | 打印版本并退出（不需要配置文件） |
| `--config <path>` | 配置文件，默认 `config/server.json` |
| `--check-config` | 只校验配置与数据表，不连数据库 |
| `--create-account <name>` | 建号；配 `--nickname` 同时建角色 |
| `--issue-ticket <name>` | 签发一次性登录票据 |
| `--nickname <name>` | 角色名，配合 `--create-account` |
| `--character-type <1\|2>` | 新手角色类型，默认 1 |
| `--password <pw>` | 账号密码（或环境变量 `BF_ACCOUNT_PASSWORD`） |

## 目录结构

```
cmd/bfserver            程序入口：参数解析、启动顺序、账号管理
internal/config         JSON 配置加载与严格校验
internal/logging        slog + 滚动文件
internal/protocol       传输编解码、全部封包读写
internal/store          MariaDB 持久化（账号、角色、背包、商店、结算）
internal/features       业务服务：注册、角色、背包、商店
internal/game           房间、广场、对局、生命状态机、结算、包分发
internal/net            游戏 / 认证 / 中继三个监听器
internal/world          静态数据表（地图、武器、角色、防御、复活、成长）
config/                 服务端配置 + 6 份游戏数据表
database/               14 份迁移 + migrate.sh
launcher/               登录器源码（Python + tkinter，Windows）
gen-cert.sh             生成认证端自签证书
```

## 实现说明

- **并发模型**：原版是 Boost.Asio 单线程事件循环，状态天然无锁；Go 版改为每连接
  一个 goroutine，共享状态加锁，且**所有网络写在锁外**（先取快照再发送，避免慢客户端
  拖住整个房间）。
- **受害者上报**：与原版一致，服务端不计算伤害，只校验客户端上报的击倒/泡封/救援/
  踢击事件并计分；队伍分 = 该队成员 Pang 数之和，达到 `game.values128[0]`（默认 20）结束对局。
- **单频道**：服务端只运行大厅频道 `60/0`。客户端工具栏仍显示两个频道，选第二个会收到
  失败结果并弹出客户端自己的提示，不会重连。
- **新手手雷**：新建角色额外获得 3 颗手雷（物品 205003）并装备在特殊武器槽 21。
  原版商城没有这个商品，这是本实现的额外设定，改 `internal/store/store.go` 的
  `StarterGrenades` 即可调整。

## 已知限制

- 仅支持竞技模式（`mode 1`，`sub_mode 0/1/3/4`）；道具战（`mode 3`）会让服务端变成
  纯客户端权威，因此拒绝开启
- 单大厅频道，无跨频道分流
- 战斗判定依赖客户端上报，无服务端反作弊
- 同一账号可重复登录，但只有第一个能进广场/房间（第二个会在进广场时被拒）

## 常见问题

| 现象 | 原因与处理 |
| --- | --- |
| `errno: 150 Foreign key constraint is incorrectly formed` | 迁移按文件名顺序跑了。用 `database/migrate.sh`，它按版本号排序 |
| `Unknown collation: 'utf8mb4_0900_ai_ci'` | 这份报错来自 MySQL 8 的排序规则；本仓库的迁移用的是 `utf8mb4_unicode_ci`，在 MariaDB 上正常 |
| `Unknown system variable 'charset'` | 驱动把 `charset` 当系统变量下发（MariaDB 不认）。代码已改用 `Collation` |
| 启动报 `database schema version mismatch` | `schema_migrations` 不是恰好 1..14 或名字对不上，重新跑迁移 |
| 启动报找不到 `authentication_certificate.pem` | `authentication.enabled` 为 true 但没生成证书：跑 `./gen-cert.sh`，或先置为 false |
| 登录器注册提示"邀请码不正确" | 服务端配了 `authentication.inviteCode`，填入同一个码即可；留空表示开放注册 |
| 登录器注册提示"服务繁忙" | 触发了按 IP 的注册限流（默认 5 次/小时、20 次/天），等窗口过去或调整配置 |

## 许可证

[MIT License](LICENSE)，Copyright (c) 2026 kanomahoro。

协议行为与数据库结构参考自
[K1roSama/BubbleFighter_CN0261_Server](https://github.com/K1roSama/BubbleFighter_CN0261_Server)
（Copyright (c) 2026 Albert Gong，同为 MIT），其版权声明一并保留在 `LICENSE` 中。

第三方依赖许可：`github.com/go-sql-driver/mysql`（MPL-2.0）、
`golang.org/x/crypto`（BSD-3-Clause）。

MIT 许可仅覆盖本仓库的**源代码**。游戏名称、商标、客户端、美术与音频等素材的
权利归原权利人所有，不在本许可范围内；本项目不随附任何此类素材。
