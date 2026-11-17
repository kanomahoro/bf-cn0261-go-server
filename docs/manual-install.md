# 手动安装与登录器

> 想省事就直接用一键脚本（Debian 12+，见 [README 的「一键安装」](../README.md#一键安装debian-12)）：
> 它会把下面 1~3 步全做掉。本文留给你想自己掌控每一步、或者在别的发行版 / Windows 上部署时用。

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

> 初始化脚本（`database/migrate.sh`）自己也会 `CREATE DATABASE IF NOT EXISTS`，所以这一步
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
- 账号建好后，`database/migrate.sh` 用默认的 `bfserver` 账号就能直接跑（它有该
  库的 `CREATE` 权限，可以执行 `CREATE DATABASE IF NOT EXISTS` 和建表）

#### 1.3 初始化表结构（导入数据库基线）

整个 schema 就在一个文件里：`database/baseline.sql`（22 张业务表 + 商店/角色/饰品数据 +
`schema_version`），**一次性导入**，不再需要按顺序跑一堆迁移文件。

Linux / macOS / Termux —— 脚本默认就用 `bfserver` 账号，只需提供密码：

```bash
BF_DB_PASSWORD='***' ./database/migrate.sh
```

> `BF_DB_PASSWORD` 是**必需**的，脚本不会交互式询问，缺了会直接报
> `BF_DB_PASSWORD is required`。
> 想换账号/库名/连接地址时再覆盖默认值：
> ```bash
> BF_DB_USER=root BF_DB_NAME=bubblefighter_cn0261 BF_DB_PASSWORD='***' \
>   ./database/migrate.sh 127.0.0.1 3306
> ```
> 这种 `变量=值 命令` 的写法只对当前这一条命令生效，比 `export` 安全——密码不会留在
> 当前会话里；注意它也会进 shell 历史（介意的话在命令前加一个空格，并设置
> `HISTCONTROL=ignorespace`）。
>
> 库里已经有 `schema_version`（表已初始化过）时脚本会**拒绝导入**（基线会 DROP 同名表）：
> 确实要重建（会丢数据）就加 `--force`，重建前自己备份。

Windows（没有 bash）：

```bat
set M=C:\BF\mariadb\bin\mariadb.exe
%M% -u root -p -e "CREATE DATABASE IF NOT EXISTS bubblefighter_cn0261 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
%M% -u root -p bubblefighter_cn0261 < database\baseline.sql
```

#### 1.4 校验初始化结果

```bash
mariadb -u root -p -e "SELECT revision FROM bubblefighter_cn0261.schema_version WHERE id = 1;"
```

版本号只是标签（当前 1），**启动时不校验它**；服务端校验的是表结构。想确认结构对不对，直接启动服务端看日志即可。

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
[Database] checking connection and schema structure  ← 连库并比对表/列/类型（不看数据）
[Database] purging expired login tickets            ← 清理过期登录票据
[Relay] listening            port=26104
[Authentication] listening   port=26102
[Application] ready port=26100 protocol=cn0261-ticket-login
```

看到 `[Application] ready` 就是**初始化全部完成**，可以直接跑第 5 步建号登录。

> 服务端启动时**不改表结构、也不校验版本号**，只比对表结构。
> 结构不一致时会列出差异，例如：
> `database structure does not match this build (1 problem(s) …): missing column shop_items.display_order; …`
> 只改数据（上架/改价/玩家数据）不会触发任何报错。

商店内容（衣服 / 武器 / 杂项 / 投掷物 / 新角色 / 功能饰品）**都已包含在基线里**，
新装不需要再导任何 `shop-patches/*.sql`。

排障对应关系：

| 日志表现 | 原因 |
| --- | --- |
| 卡在 `checking connection` | 账号/密码不对，或账号没用 `mysql_native_password` |
| `database structure does not match this build` | 库结构不匹配：按报错补结构，或 `bash database/migrate.sh --force` 重建（先备份） |
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
msg="BubbleFighter CN0261 Go Server v0.4.0  By:rurudoo"
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
| `hidpi.py` | 可选的分辨率扩展：把原生 7 档分辨率表换成显示器支持的完整列表 |

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

## 高分辨率扩展（可选）

主界面有一个「开启高分辨率扩展」勾选框，勾选后启动客户端时会额外打一组内存补丁，
让设置菜单里出现显示器真正支持的模式（含 1920×1080）；不勾选则完全不动客户端。

- 补丁点来自 MIT 项目 `Wasabi-FantasyTaleDev/FantasyTale.Launcher` 的
  `bubblefighter.fantasy.resolution-extension` 插件，`builds/cn-717.toml` 描述符
- 只改内存、不改文件：补丁在 `CreateProcess` 挂起期间写入，进程恢复即生效
- 客户端原生分辨率表是 7 档（800×600 ~ 1600×900），固定在 VA `0xFDE9F8`；
  补丁把它换成新表，并 NOP 掉「索引 > 6 就跳走」的检查
- 上架规则与该插件一致：保留原 7 档，硬件模式需 ≥1024×768 且宽高比不超过 16:9，
  800×600 例外；新增项按 `宽x高` 升序排列
- **上限 1920×1080**：客户端画布按 1600×900 设计，真机实测超过 1080p 会被拉伸失真，
  所以 4K/1440p 显示器也只列出到 1080p
- 默认只靠 `EnumDisplaySettingsW` 枚举真机支持的模式，不硬塞任何档位；
  比插件多一道保守过滤：超过桌面尺寸的硬件模式直接丢弃
  （桌面尺寸读 `ENUM_CURRENT_SETTINGS` 的**物理**分辨率，不受 Windows 显示缩放影响）
- 默认**不动**客户端的显示判定函数（插件默认也是不动，只有用户填自定义分辨率时才挂钩）。
  万一菜单里看不到新档位，把 `hidpi.FORCE_PREDICATE` 改成 `True` 再试
- **任何一处校验对不上就整体放弃**；写操作按插件同款做法带回滚（记原字节，失败按逆序写回），
  不会留下改了一半的分辨率表指针
- **客户端是加壳的**：补丁要等代码段解包后才打得进去（可能几秒），所以补丁在**独立线程**里
  轮询就绪后再挂起进程打补丁；**票据注入同时立刻执行**，不被它拖慢。
  两边共用一把锁，保证「挂起-改-恢复」不会嵌套。状态栏显示「最大分辨率扩展到 宽x高」；
  没等到就在状态栏说明原因，游戏照常开
- 自检（不启动游戏）：`python hidpi.py path\to\BubbleFighter.exe`
  只核对 PE 头，随后会报补丁点字节不符——**加壳时这是预期结果**（文件里还是密文），
  补丁点只能在运行期用 `image_ready` 轮询确认

> 注意：cn-717 的描述符里**没有** `[main_stage]` 段（cn-406 才有），所以本扩展只扩
> 分辨率菜单与设置校验，不扩游戏内 3D 画布 —— 高分辨率下 UI 变清晰，画面是否拉伸
> 取决于客户端自身的缩放，需要实机确认。

## 打包成单文件 exe

```bash
pip install pyinstaller
pyinstaller --noconfirm --onefile --windowed --name BubbleFighterCN0261Launcher launcher.py
```

产物在 `dist/` 下，把它和 `BubbleFighter.exe` 放同一目录即可。

> 说明：登录器只是**客户端侧的便利工具**，没有它也能玩——服务端的认证端（26102）发的是
> 标准的一次性票据，任何能完成「TLS 取票 + 注入客户端」的启动器都可以替代。
