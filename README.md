# BubbleFighter CN0261 Server (Go)

[![MIT License](https://img.shields.io/badge/License-MIT-D22128?style=flat-square)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go)
![Version](https://img.shields.io/badge/version-v0.1.3-blue?style=flat-square)

泡泡战士 CN0261（国服 2015 版，协议 717）服务端的 Go 重写版。单二进制、无 cgo，
协议与数据库结构与 [K1roSama/BubbleFighter_CN0261_Server](https://github.com/K1roSama/BubbleFighter_CN0261_Server)（C++ 原版）保持一致。

仓库：<https://github.com/kanomahoro/bf-cn0261-go-server>

当前版本 **v0.1.3**，已跑通「登录 → 广场 → 建房 → 对局 → 结算落库 → 回房」完整闭环。

v0.1.3 修三处：

1. **对局内「有人像雕像」的卡死**，根因是**同步阻塞写**：服务端每帧都是持锁
   `conn.Write`，而中继在一个 goroutine 里按顺序写给所有对端，只要有一台客户端短暂
   不读（卡顿、后台、加载），整条转发循环连同后面所有人的流都被堵住——被堵的那台读
   方向正常，还在发位置、照常掉血泡封，但别人收不到它的位置。现在两条连接都改成
   **入队 + 独立 writer goroutine**，队列超限只断开那一条连接，见
   [发送队列与背压](#发送队列与背压)。
2. **复活点和队友重叠**：同队两人总是落在同一个标记上，连续复活也不轮换。现在是
   **每名队友独享队内一段标记**（`stride = team_spawn_count / 本队人数`），两人一队时
   一人用 1~4、另一人用 5~8，互不干扰；同时进房改为按人数较少的一队分配，见
   [复活点](#复活点) 与 [队伍分配与平衡](#队伍分配与平衡)。
3. **结算的经验与金币会「界面多给、库里少存」**：写库和结果界面各算一遍，夹断规则不
   一致；存量越界数据还会让 `game_result_players` 的 `before + gain = after` 约束
   拒绝插入，整局结算失败。现在两处共用一个夹紧函数，界面上报的是**实际到账值**，见
   [结算与上限](#结算与上限)。

v0.1.1 的白名单（中继只转发对局内状态包、其余服务端自己消费）是与原版对齐的正确行为，
保留不变，但它**不是**那次卡死的修复，见[中继转发策略](#中继转发策略)。

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
- 中继：对局内 peer 转发（26104），按预约 key 与源 IP 绑定，且只转发白名单内的对局内状态包

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
go build -ldflags "-X main.Version=0.1.3" -o bin/bfserver ./cmd/bfserver

./bin/bfserver --version
# BubbleFighter CN0261 Go Server v0.1.3  By:rurudoo
```

## 快速开始

### 1. 建库到初始化完成

> 前置条件：MariaDB 10.11 已安装并在运行。Debian/Ubuntu 上 `sudo apt install -y
> mariadb-server && sudo systemctl enable --now mariadb` 即可；Windows 用官方
> 免安装 zip 解压后跑 `bin\mariadb-install-db.exe --datadir=<数据目录>
> --service=MariaDB --password=<root密码>`，再 `net start MariaDB`。
> 这一节从**创建数据库**开始写，到**服务端初始化完成**结束。

#### 1.1 创建数据库

```sql
CREATE DATABASE IF NOT EXISTS bubblefighter_cn0261
    CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

> 迁移脚本自己也会执行同样的 `CREATE DATABASE IF NOT EXISTS`，所以这一步
> 重复执行没有副作用；先建的好处是能在这里明确指定字符集。

#### 1.2 创建运行账号

不要用 root 跑服务端。建账号时有两个容易踩的点：

1. **`localhost` 和 `127.0.0.1` 两个 host 都要建**：MariaDB 把 socket 连接认作
   `localhost`、把 TCP 连接认作 `127.0.0.1`，只建其中一个会出现“账号明明存在却
   Access denied”
2. **必须显式指定 `mysql_native_password`**：`go-sql-driver/mysql` 只认
   `mysql_native_password` 和 `caching_sha2_password`；账号如果落到
   `unix_socket` 或 `ed25519` 插件上，服务端连库会直接失败

```sql
CREATE USER IF NOT EXISTS 'bfserver'@'127.0.0.1'
  IDENTIFIED VIA mysql_native_password USING PASSWORD('<密码>');
CREATE USER IF NOT EXISTS 'bfserver'@'localhost'
  IDENTIFIED VIA mysql_native_password USING PASSWORD('<密码>');

GRANT ALL PRIVILEGES ON bubblefighter_cn0261.* TO 'bfserver'@'127.0.0.1';
GRANT ALL PRIVILEGES ON bubblefighter_cn0261.* TO 'bfserver'@'localhost';
FLUSH PRIVILEGES;
```

建完立刻核对插件，`plugin` 一列必须是 `mysql_native_password`：

```sql
SELECT user, host, plugin FROM mysql.user WHERE user = 'bfserver';
```

几点说明：

- 服务端运行时实际只用到 `SELECT / INSERT / UPDATE / DELETE`（全部代码里没有任何
  DDL 语句），这里授予 `ALL` 是为了让同一个账号也能跑迁移和日常维护，省掉权限报错
- `ALL PRIVILEGES` **只作用于 `bubblefighter_cn0261` 这一个库**，也不包含
  `GRANT OPTION`，所以它既访问不到别的库，也不能给自己或别人授权
- 如果你的版本上 `PASSWORD()` 函数报 `FUNCTION ... does not exist`，改用
  `CREATE USER ... IDENTIFIED BY '<密码>';`，再用上面的 `SELECT` 确认插件仍是
  `mysql_native_password`；若不是，用
  `ALTER USER 'bfserver'@'127.0.0.1' IDENTIFIED VIA mysql_native_password USING PASSWORD('<密码>');` 纠正
- 账号建好后，`database/migrate.sh` 用默认的 `bfserver` 账号就能直接跑迁移（它有该
  库的 `CREATE` 权限，可以执行 `CREATE DATABASE IF NOT EXISTS` 和建表）

#### 1.3 初始化表结构（14 份迁移）

建库和全部 23 张表都在 `database/migrations/` 里，不需要手写建表语句。
顺序**必须按 `schema_migrations` 里的 v1~v14**，不能按文件名首字母（外键会失败）。

Linux / macOS / Termux —— 脚本默认就用 `bfserver` 账号，只需提供密码
（脚本会自己从每份 SQL 末尾读出**版本号**再排序）：

```bash
BF_DB_PASSWORD='***' ./database/migrate.sh
```

> `BF_DB_PASSWORD` 是**必需**的，脚本不会交互式询问，缺了会直接报
> `BF_DB_PASSWORD is required`。
> 想换账号或连接地址时再覆盖默认值：
> ```bash
> BF_DB_USER=root BF_DB_PASSWORD='***' ./database/migrate.sh 127.0.0.1 3306
> ```
> 这种 `变量=值 命令` 的写法只对当前这一条命令生效，比 `export` 安全——密码不会留在
> 当前会话里；注意它也会进 shell 历史（介意的话在命令前加一个空格，并设置
> `HISTCONTROL=ignorespace`）。

Windows（没有 bash，逐条执行，顺序就是下面这个）：

```bat
set M=C:\BF\mariadb\bin\mariadb.exe
cd /d C:\BF\bf-cn0261-go-server

%M% -u root -p < database\migrations\initialize_schema.sql
%M% -u root -p < database\migrations\create_accounts.sql
%M% -u root -p < database\migrations\create_characters.sql
%M% -u root -p < database\migrations\create_inventory.sql
%M% -u root -p < database\migrations\create_character_stats.sql
%M% -u root -p < database\migrations\create_shop.sql
%M% -u root -p < database\migrations\create_game_results.sql
%M% -u root -p < database\migrations\expand_shop_cosmetics.sql
%M% -u root -p < database\migrations\create_shop_characters.sql
%M% -u root -p < database\migrations\expand_shop_equipment.sql
%M% -u root -p < database\migrations\create_respawn_equipment.sql
%M% -u root -p < database\migrations\create_result_awards.sql
%M% -u root -p < database\migrations\create_weapon_presets.sql
%M% -u root -p < database\migrations\expand_shop_other.sql
```

#### 1.4 校验迁移结果

```bash
mariadb -u root -p -e "SELECT version, name FROM bubblefighter_cn0261.schema_migrations ORDER BY version;"
```

必须**正好 14 行、版本 1..14 连续**。少一行或名字对不上，服务端启动会直接报
`database schema version mismatch`。

#### 1.5 把密码交给服务端

二选一，**不要把密码提交进 git**：

- 环境变量（推荐）：`database.password` 留空，启动前设 `BF_DATABASE_PASSWORD`
- 写进配置：`database.password` 填值，并把配置文件权限设为 `600`

配置里的值优先于环境变量；两者都没有时启动会报
`database password is missing: set database.password or BF_DATABASE_PASSWORD`。

#### 1.6 首次启动，完成初始化

```bash
./bfserver --check-config   # 只校验配置文件，不连库
./bfserver                  # 真正启动
```

服务端启动时自动完成的初始化动作，日志里依次是：

```
[Database] checking connection and schema version   ← 连库并校验 14 份迁移
[Database] ensuring the grenade item definition     ← 补齐新手手雷（物品 205003）
[Database] purging expired login tickets            ← 清理过期登录票据
[Relay] listening            port=26104
[Authentication] listening   port=26102
[Application] ready port=26100 protocol=cn0261-ticket-login
```

看到 `[Application] ready` 就是**初始化全部完成**，可以直接跑第 5 步建号登录。

排障对应关系：

| 日志表现 | 原因 |
| --- | --- |
| 卡在 `checking connection` | 账号/密码不对，或账号没用 `mysql_native_password` |
| `database schema version mismatch` | 1.3 的 14 份迁移没跑全，或版本号不连续 |
| `database password is missing` | 1.5 没设 `BF_DATABASE_PASSWORD`，配置里也是空的 |

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
| `authentication.gameHost` | `127.0.0.1` | 预留字段，**当前服务端不下发**（只做格式校验），客户端连哪个 IP 由登录器决定 |
| `authentication.enabled` | `false` | `true`（走登录器注册时） |
| `database.tlsMode` | 数据库没开 TLS 时改 `disable`，同时 `verifyCertificate: false` | 按数据库实际配置 |

> **`relay.advertisedAddress` 会真实下发给客户端**：开对局时服务端把它作为 4 字节 IPv4
> 写进 S601（中继端点 + 20 字节 key）。填 `127.0.0.1` 只有本机能进对局，远程客户端会
> 连不上中继、卡在加载；对外开服必须填公网 IP。

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
msg="BubbleFighter CN0261 Go Server v0.1.3  By:rurudoo"
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

## 配置说明

配置只有一份文件：`config/server.json`（`--config` 可指向任意路径）。JSON **严格校验**：
未知字段、类型不符、取值越界都会在启动时报错退出，不会静默忽略。

### 数据表路径（顶层）

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `gameMapsFile` | `config/game_maps.json` | 地图表 |
| `gameWeaponsFile` | `config/game_weapons.json` | 武器表 |
| `gameDefenseFile` | `config/game_defense.json` | 防具表 |
| `gameCharactersFile` | `config/game_characters.json` | 角色 / 频道 / 模式表 |
| `gameRespawnFile` | `config/game_respawn.json` | 复活延迟表 |
| `progressionFile` | `config/progression.json` | 等级、奖励、评级表 |

### `network` — 游戏端口（26100）

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `listenAddress` | `0.0.0.0` | 监听地址；本机测试可改 `127.0.0.1` |
| `listenPort` | `26100` | 游戏/大厅端口 |
| `maxConnections` | `256` | 并发连接上限 |
| `idleTimeoutSeconds` | `300` | 连接空闲超时（秒），超时即断开 |

### `relay` — 对局中继（26104）

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | `true` | 是否启动中继；关掉后无法进行对局 |
| `listenAddress` / `listenPort` | `0.0.0.0` / `26104` | 监听地址与端口 |
| `advertisedAddress` | `127.0.0.1` | **会真实下发客户端**：开对局时写进 S601 的 4 字节 IPv4。对外开服必须填公网 IP |
| `maxConnections` | `256` | 并发中继连接上限 |
| `idleTimeoutSeconds` | `120` | 中继连接空闲超时 |

### `database` — MariaDB

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `host` / `port` | `127.0.0.1` / `3306` | 数据库地址 |
| `databaseName` | `bubblefighter_cn0261` | 库名，迁移里写死，改名需同步改 SQL |
| `username` | `bfserver` | 运行账号 |
| `password` | 空 | 留空则读环境变量 `BF_DATABASE_PASSWORD`（配置文件优先） |
| `connectTimeoutSeconds` / `queryTimeoutSeconds` | `5` / `5` | 连接与查询超时 |
| `poolSize` | `4` | 连接池大小 |
| `tlsMode` | `disable` | `require` / `disable`；本地 MariaDB 通常没开 TLS 就填 `disable` |
| `verifyCertificate` | `false` | `tlsMode: require` 时必须为 `true`，否则配置校验失败 |
| `caFile` | 空 | 自签 CA 路径，用系统信任链时留空 |

### `authentication` — 登录器认证端（26102）

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | `true` | 是否启动认证端；关掉后登录器无法取票 |
| `listenAddress` / `listenPort` | `0.0.0.0` / `26102` | 监听地址与端口 |
| `certificateFile` / `privateKeyFile` | `config/authentication_*.pem` | TLS 证书与私钥，用 `./gen-cert.sh` 生成 |
| `gameHost` | `127.0.0.1` | 预留字段，**服务端不下发**（只做格式校验） |
| `inviteCode` | 空 | 注册邀请码；留空 = 开放注册，填了则必须一致（常量时间比较） |
| `registrationsPerHour` / `registrationsPerDay` | `5` / `20` | 按客户端 IP 的注册配额；`0` 表示用默认值 |

### `logging`

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `level` | `info` | `trace`/`debug`/`info`/`warn`/`error` |
| `directory` | `Logs` | 日志目录，不存在会自动创建 |
| `maxFileSizeBytes` | `5242880` | 单文件 5 MB 触发轮转 |
| `maxFiles` | `3` | 保留 3 份历史，合计约 20 MB 上限 |

日志同时写控制台与文件；目录不可写会导致**启动失败**，不会静默降级。
`debug` 会记录每个进/出包，排查协议问题很有用，但一天能写满 5 MB，日常建议保持 `info`。

级别约定（改日志时照着来）：

| 级别 | 写什么 | 例子 |
| --- | --- | --- |
| `info` | 一局的骨架：开局、结束、结算落库；一局对局下来几行而已 | `[Round] started`、`[Round] ended`、`settlement clamped` |
| `warn` | 玩家能感知、但服务端自己扛过去的问题 | `game result packet rejected`、`character reload failed`、`loading timed out`、`[Relay] registration timeout` |
| `error` | 数据写不进去、连接 panic、启动失败 | `settlement failed`、`startup failed` |
| `debug` | 单个包的故事：进/出包、复活细节、被规则丢弃的包、重复到的确认包 | `inbound`、`[Relay] inbound`、`[Respawn] completed`、`[Respawn] confirmation ignored` |

原则：**INFO 回答「这一局发生了什么」，DEBUG 回答「这个包发生了什么」。** 复活属于后者
——一局可能复活几十次，对开服的人没有信息量，所以 `[Respawn] completed` 放在 DEBUG，
一行说清（下发的 `spawn`、客户端回传值、是否采纳、状态是否推进）；重复到达的确认包同样
只在 DEBUG。**INFO 下打一局通常只有 3 行**：`[Round] started`、`[Round] ended`、
`round cancelled`；出现夹断时再多一行 `settlement clamped`。

### `config/` 目录里的数据表

| 文件 | 内容规模 | 管什么 |
| --- | --- | --- |
| `server.json` | 服务端配置 | 上面所有字段 |
| `game_maps.json` | 57 张地图 | 地图池、主题、每图的默认比分上限与时限 |
| `game_weapons.json` | 466 把武器 | 武器 ID 白名单与伤害系数（服务端只校验 ID，不计算伤害） |
| `game_defense.json` | 448 件防具 | 防御修正，开对局时据此生成防御种子表下发 |
| `game_characters.json` | 11 角色 / 12 频道 / 2 模式 | 角色血量修正、频道与模式元信息 |
| `game_respawn.json` | 11 角色 + 10 道具 | 复活等待时长（道具优先于角色） |
| `progression.json` | 10 档等级 | 经验阈值、胜负平奖励、个人分权重、评级与奖牌门槛 |

这些 JSON 都是**运行时读取**，改完重启服务端即生效，不需要重新编译。

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

## 数据库

建库后共 **23 张表**，全部由 `database/migrations/` 里的 14 份迁移创建，不需要手写任何建表语句。

### 迁移文件（14 份，必须按版本号顺序执行）

`database/migrate.sh` 会从每份 SQL 末尾登记的 `schema_migrations` 版本号自动排序，
不要按文件名首字母顺序手动执行（外键会失败）。Windows 上没有 bash，按
[快速开始 · 1.3](#13-初始化表结构14-份迁移) 里列出的 14 条命令顺序执行即可。

| 版本 | 文件 | 建了什么 |
| --- | --- | --- |
| v1 | `initialize_schema.sql` | 建库 + `schema_migrations` 版本记录表 |
| v2 | `create_accounts.sql` | `accounts`、`login_tickets` |
| v3 | `create_characters.sql` | `characters` |
| v4 | `create_inventory.sql` | `item_definitions`、`character_items` |
| v5 | `create_character_stats.sql` | `character_stats`（Lucci / 点卷钱包） |
| v6 | `create_shop.sql` | 商店 5 张表 + `character_item_expirations` + `shop_purchases` |
| v7 | `create_game_results.sql` | `character_progression`、`game_results`、`game_result_players` |
| v8 | `expand_shop_cosmetics.sql` | `item_equipment_rules` + 化妆品商品种子 |
| v9 | `create_shop_characters.sql` | `character_appearances`（角色外观道具） |
| v10 | `expand_shop_equipment.sql` | 纯数据：武器/装备商品与库存 |
| v11 | `create_respawn_equipment.sql` | 纯数据：复活与出生装备配置 |
| v12 | `create_result_awards.sql` | `game_result_awards`（评分/评级/奖牌） |
| v13 | `create_weapon_presets.sql` | `character_weapon_presets`、`character_weapon_preset_items`、`shop_permanent_stocks` |
| v14 | `expand_shop_other.sql` | `shop_preview_items` + 其余杂项商品 |

### 表结构（按用途分组）

**账号与角色**

| 表 | 管什么 |
| --- | --- |
| `accounts` | 账号（账号名、scrypt 盐与哈希、是否停用） |
| `login_tickets` | 一次性登录票据（120 秒有效，用后即废） |
| `characters` | 角色（昵称、角色类型、创角选择），`account_id` 唯一 |
| `character_stats` | 钱包：`lucci`、`cash` |
| `character_progression` | 累计经验值 |

**道具与背包**

| 表 | 管什么 |
| --- | --- |
| `item_definitions` | 道具定义（名称、装备槽位、是否新手自带） |
| `character_items` | 角色持有的道具：数量、装备槽位；`(character_id,item_id)` 唯一 |
| `character_item_expirations` | 限期道具的到期时间（无此行 = 永久） |
| `item_equipment_rules` | 装备规则：占用槽位位图 + 允许的角色类型掩码 |
| `character_appearances` | 角色类型 → 外观道具（建号时塞进槽位 63） |

**商店**

| 表 | 管什么 |
| --- | --- |
| `shop_main_categories` | 主分类（武器 / 服装 / 角色 / 杂项…） |
| `shop_sub_categories` | 子分类（步枪 / 副武器 / 特殊武器 307 等）+ 页签类型 |
| `shop_items` | 上架商品：挂哪个子分类、货币（`lucci`/`cash_cn`）、上下架时间 |
| `shop_stocks` | 价格档位：`duration_days` + 标价 + 实价 |
| `shop_permanent_stocks` | 标记某档位为“永久”（如武器预设扩张卡） |
| `shop_preview_items` | 只展示不可购买的道具 |
| `shop_purchases` | 购买审计流水（角色、档位、花费、到期时间） |

**对局与结算**

| 表 | 管什么 |
| --- | --- |
| `game_results` | 每局一行：房间、模式、地图、双方队分、胜方、结束原因 |
| `game_result_players` | 每局每玩家一行：泡封数、击爆数、救援数、经验与 Lucci 前后值、等级 |
| `game_result_awards` | 与玩家行一对一：本局评分、评级、奖牌 |

结算时这三张表 + `character_progression` / `character_stats` 在同一事务里写入，
所以经验、金币、战绩三者永远一致。

**武器预设与元数据**

| 表 | 管什么 |
| --- | --- |
| `character_weapon_presets` | 每个角色的预设槽（2 或 3 个、当前选中项） |
| `character_weapon_preset_items` | 预设槽里存的武器道具 ID |
| `schema_migrations` | 已应用的迁移版本（启动时 `CheckSchema` 校验 1..14） |

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
  一个 goroutine，共享状态加锁，网络写统一走每连接一条发送队列（见下一节）。
- **受害者上报**：与原版一致，服务端不计算伤害，只校验客户端上报的击倒/泡封/救援/
  踢击事件并计分；队伍分 = 该队成员 Pang 数之和，达到 `game.values128[0]`（默认 20）结束对局。
- **单频道**：服务端只运行大厅频道 `60/0`。客户端工具栏仍显示两个频道，选第二个会收到
  失败结果并弹出客户端自己的提示，不会重连。
- **新手手雷**：新建角色额外获得 100 颗手雷（物品 205003）并装备在特殊武器槽 21。
  原版商城没有这个商品，这是本实现的额外设定，改 `internal/store/store.go` 的
  `StarterGrenades` 即可调整。

### 发送队列与背压

原版 `TcpListener::Send` 从不直接写 socket：每帧入队后立即返回，由独立协程
`async_write`；队列超过 `kMaxQueuedBytes`（8 MiB）就 `Stop()` 那个 session——只断它，
绝不影响其他连接。

Go 版两条连接（26100 游戏端口、26104 中继端口）采用同样的模型：

- `connection.Send` / `relayConn.send` 只做编码 + 入队，调用方**永不阻塞**
- 每条连接一个 writer goroutine，是该 socket 唯一的写入者
- 上限 8 MiB 或 8192 帧，超限时关闭这条连接并打 WARN
  （`[Network] closing connection` / `[Relay] write failed; closing connection`）

这条不是优化，是正确性：转发循环是「一个 goroutine 顺序写给所有对端」，只要允许
同步写，一台客户端不读就能让整个房间的同步流停摆。改动前请确认这一约束仍然成立。

### 复活点

复活（`S708`）携带的地图标记编号由服务端决定：

```
stride      = team_spawn_count / 本队人数
spawn_index = team * 10 + teammate_ordinal * stride + respawn_count % stride + 1
```

- `team_spawn_count` 来自地图表（本项目 57 张图都是 8），编号只会落在该地图定义过的
  T11..T18 / T21..T28 之内，不会把角色丢到地图外
- **每个队友独占队内一段标记**：两人一队时各 4 个点，第一个人用 1~4、第二个人用 5~8，
  各自在自己的份额里轮换。所以**无论座位怎么排、死亡次数怎么错开，队友都不会重叠**
- 原版是整队共用一条环形标记（`slot/2 + respawn_count`，每次 +1），两个队友只差一步，
  死几次之后就会踩到对方正在站着的点上——v0.1.2 及更早版本照搬了这条环形，现场表现为
  「复活后和队友叠在同一个点」，所以改成分段独占
- 每次复活 `respawn_count++`，`respawn_count` 每局清零

复活的收尾有两条路径：游戏连接的 `C704` 和中继上的 `P32925`（客户端上报“复活完成”）。
任一条到达都会把角色状态推回 `active` 并广播 `S707`，所以即使某一条丢了，角色也不会
卡在“已确认未激活”的状态里。客户端回传的标记只要在本队区块内就以客户端为准。

`BattleLife.OfferRespawn` 还会校验 `spawn_index/10 == team` 且
`spawn_index%10 ∈ [1,8]`，不合法时宁可不发 `S708`；这条校验是最后一道闸，正常路径下
`NextSpawnIndex` 已经保证了范围。

### 结算与上限

结算写库与结果界面（`S729`）共用同一个夹紧函数 `store.ClampGain`：
先归一化存量越界值，再加增益并夹断，返回**实际到账值**。

```
before'  = min(before, cap)          # NormalizeTotal，先钉住基值
after    = min(before' + gain, cap)  # 64 位求和，避免无上限配置时回绕
applied  = after - before'           # 界面与结果表都显示这个值
```

为什么必须先归一化基值：`game_result_players` 上有
`CHECK (experience_before + experience_gain = experience_after)`（金币同理）。只夹
结果值、把 9000 这样的存量越界值原样写进去，`9000 + 0 ≠ 4500`，插入会被数据库拒绝，
结算重试三次全失败后整局取消——**一局白打，且没有结算界面**。

| 项 | 上限 | 常量 | 到顶时 |
| --- | --- | --- | --- |
| 经验 | 等级表最后一级阈值（本项目 4500） | `Progression.MaxExperience()` | 界面 +0 |
| 金币 | 10,000,000 | `store.MaxLucci` | 界面 +0 |

- 经验对齐原版 `Advance` 的 `min(experience + gain, thresholds.back())`
- **读取侧保持严格**：越界数据在登录时直接报错（`S105` 发不出去），不会悄悄修补，
  需要用 SQL 手工压回上限
- 发生夹断时打一行 INFO `settlement clamped`，经验与金币各自的 `before/after/gain` 都在
  里面，现场可以直接对数；`S729` 被协议拒绝、角色重载失败、`S105` 刷新失败这三种玩家能
  感知的错误是 WARN

### 队伍分配与平衡

进房时按**人数较少的一队**分配（原版 `ChooseTeam` 同款），手动换队（`C515`）受
「一队最多占满 8 个座位的一半」限制，和原版 `kRoomCapacity / 2` 一致。

八人依次进房的结果：

| 顺序 | 分到 | 顺序 | 分到 |
| --- | --- | --- | --- |
| 1 | 队 1 | 5 | 队 1 |
| 2 | 队 2 | 6 | 队 2 |
| 3 | 队 1 | 7 | 队 1 |
| 4 | 队 2 | 8 | 队 2 |

最终 `4 : 4`，座位 `1,2,1,2,1,2,1,2`。开局要求两队人数相等且除房主外全部就绪，
否则房主会收到 `StartUnequal` / `StartNotReady` 的失败提示。

房间容量取房间设置里的 `capacity`，`≤0` 或 `>8` 时按 8 处理；中途有人退出后新人补位
仍按「人少的那队」分配，所以队伍不会越走越偏。

### 中继转发策略

原版的 relay 不自己转发：它把每个包交给房间协调器，由协调器校验后决定发给谁。
Go 版保留同样的边界——`internal/net/relay.go` 的 `relayForwardable` 是一份白名单，
命中才转发给同房间其他成员，其余交给游戏层处理后丢弃。

会转发（对局内 peer 状态）：

| 分类 | 包 id |
| --- | --- |
| 动作 | 32779 / 32780 / 32781 / 32782 / 32785 / 32810 / 32811 / 32813 |
| 开火 | 32898 / 32901 / 32924 |
| 近身 · 命中 | 32773 / 32777 |
| 爆炸 · 引爆 · 移除 | 32776 / 32923 / 32900 / 32906 |
| 生命 · 生命点 | 32787 / 32793 / 32903 / 32807 / 32805 / 32836 / 32801 / 32806 / 32788 / 32790 / 32920 |
| 踢击 | 32791 / 32792 / 32794 / 32795 / 32796 / 32835 / 32837 / 32815 / 32816 |
| 移动 · 注视目标 | 32884 / 32885 / 32922 |
| 复活完成 | 32925 |

**不转发**（服务端自己消费，对端不该看到）：

| 包 | 含义 | 为什么吃掉 |
| --- | --- | --- |
| `C1` | `ReqJoinRelaySession` | 中继注册，一个预约只绑一条连接 |
| `C3` | `RelayUseReport` | 绑定上报者的角色对象，纯服务端状态 |
| `C8` / `C9` | `RelayPeerRequest` | 声明“某对端我可以直连 / 不再直连” |
| `C14` / `C18` | 血量重置 / 血量追踪报告 | 诊断数据，只用于服务端比对 |
| `C20006` | 延迟探测 | 由中继自己应答 |
| `C32840` | 对端 ping | 原版只回给发送者（`Never broadcast this probe`） |
| `C32904` | 复活请求镜像 | 服务端已处理 |

> **不要把这张白名单当成卡死修复。** 它是与原版对齐的语义修正（原版同样不下发这些包），
> 但 v0.1.1 之后问题依旧复现——真正的根因是同步阻塞写，见[发送队列与背压](#发送队列与背压)。
> 转发 `C8`/`C9` 会让对端误以为存在直连、进而发起注定失败的 P2P 尝试，属于应当避免的
> 行为，所以白名单继续保留。以后再动它之前请把这两段一起读。

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
| 对局中对方定住不动、自己还在掉血/泡封 | v0.1.2 及更早版本的同步阻塞写缺陷（一台客户端不读就堵住整个转发循环）。升级到 v0.1.3 即可，见[发送队列与背压](#发送队列与背压) |
| 复活后和队友叠在同一个点 | v0.1.2 及更早版本没有实现复活点轮换。升级到 v0.1.3 即可，见[复活点](#复活点) |
| 满级角色打完一局经验一点不涨 | 正常：经验在结算时被夹到等级表上限（4500），界面显示 +0。日志里有 `settlement clamped` 即生效 |
| 金币到 1000 万后不再增加 | 正常：金币上限 `store.MaxLucci = 10,000,000`，超出部分不发放，界面显示实际到账值 |
| 打完一局没有结算界面 | 说明 `S729` 被协议校验拒绝。搜日志 `game result packet rejected`（WARN），常见原因是角色数据异常（账号 id 或昵称校验不通过） |

## 许可证

[MIT License](LICENSE)，Copyright (c) 2026 kanomahoro。

协议行为与数据库结构参考自
[K1roSama/BubbleFighter_CN0261_Server](https://github.com/K1roSama/BubbleFighter_CN0261_Server)
（Copyright (c) 2026 Albert Gong，同为 MIT），其版权声明一并保留在 `LICENSE` 中。

第三方依赖许可：`github.com/go-sql-driver/mysql`（MPL-2.0）、
`golang.org/x/crypto`（BSD-3-Clause）。

MIT 许可仅覆盖本仓库的**源代码**。游戏名称、商标、客户端、美术与音频等素材的
权利归原权利人所有，不在本许可范围内；本项目不随附任何此类素材。
