# 手动安装与登录器

## 1. 建库到初始化完成

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

## 2. 配置

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

## 3. 认证端证书（可选）

登录器用 TLS 连 26102，自签证书即可。仓库根目录带生成脚本：

```bash
./gen-cert.sh                                      # localhost + 127.0.0.1，10 年
HOSTS="bf.example.com 203.0.113.10" ./gen-cert.sh  # 追加对外域名/IP
FORCE=1 ./gen-cert.sh                              # 已存在时覆盖
```

脚本会生成 `config/authentication_certificate.pem` 与 `config/authentication_private_key.pem`
（即 `config/server.json` 里 `authentication` 指向的两个文件），私钥权限自动 600。
客户端不校验证书链，CN/SAN 填什么不影响登录。

## 4. 启动

```bash
./bin/bfserver --config Local/server.json --check-config   # 只校验配置，不连库
./bin/bfserver --config Local/server.json                  # 正式启动
```

正常启动的日志：

```
msg="BubbleFighter CN0261 Go Server v0.3.1  By:rurudoo"
msg="[Application] environment" go=go1.27.3 config=Local/server.json database=bfserver@127.0.0.1:3306/bubblefighter_cn0261 gamePort=26100 authPort=26102 relayPort=26104
msg="[Application] ready port=26100 protocol=cn0261-ticket-login"
msg="[Relay] listening" port=26104
msg="[Authentication] listening" port=26102
```

日志同时写控制台和 `Logs/Server.log`（滚动：单文件 5 MB，保留 3 份）。

## 5. 建号

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

## 运行前提

- **Windows**（`start_client.py` 用 `ctypes` 调 Win32 API）
- **Python 3.9+**，带 tkinter（官方安装包默认包含）
- 登录器文件与 `BubbleFighter.exe` 放在**同一个目录**（改过的启动器要在客户端安装目录里跑）
- 客户端必须是 **CN0261** 原版镜像（**需自备，仓库不提供**）：`start_client.py` 会用 SHA-256 校验，
  并在固定偏移把单实例互斥体名从 `Lambda` 改成 `B261_<实例号>`（A~Z、0~9），镜像不匹配会直接拒绝启动

## 使用

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

## 打包成单文件 exe

```bash
pip install pyinstaller
pyinstaller --noconfirm --onefile --windowed --name BubbleFighterCN0261Launcher launcher.py
```

产物在 `dist/` 下，把它和 `BubbleFighter.exe` 放同一目录即可。

> 说明：登录器只是**客户端侧的便利工具**，没有它也能玩——服务端的认证端（26102）发的是
> 标准的一次性票据，任何能完成「TLS 取票 + 注入客户端」的启动器都可以替代。
