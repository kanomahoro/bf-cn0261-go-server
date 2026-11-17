# 数据库脚本（schema revision 1）

整个 schema 只有一个文件：`database/baseline.sql`。服务端不跑迁移文件，只校验一个**版本号**：
库里 `schema_version.revision` 必须等于二进制编译时要求的 revision（当前 **1**），
不一致直接拒绝启动。

> 项目还没正式上线，所以**没有升级路径**：版本对不上就重建库（`migrate.sh --force`）。
> 以后真要长期运营，再加 `database/upgrades/` 增量通道，并在 `internal/store/schema.go`
> 里把 `SchemaRevision` 往上加。

## 文件

| 文件 | 用途 |
|---|---|
| `baseline.sql` | 基线：22 张业务表 + 商店/角色/饰品数据 + `schema_version`。整库导入，会 DROP 同名表 |
| `migrate.sh` | 新装/重建：导入基线。库里有 `schema_version` 时默认拒绝，需 `--force` |

## 新装

```bash
BF_DB_USER=bfserver BF_DB_PASSWORD='***' BF_DB_NAME=bubblefighter_cn0261 \
  bash database/migrate.sh 127.0.0.1 3306
```

## 重建（会清空数据）

```bash
BF_DB_USER=bfserver BF_DB_PASSWORD='***' BF_DB_NAME=bubblefighter_cn0261 \
  bash database/migrate.sh 127.0.0.1 3306 --force
```

重建前自己备份：`mysqldump -u ... --single-transaction <库> | gzip > 备份.sql.gz`。

## 校验

```bash
mariadb -e "SELECT * FROM bubblefighter_cn0261.schema_version"
# revision 应为 1
```

服务端启动时也会做同样的校验；不一致的输出是：

```
Startup error: database schema revision 0, this build needs 1; rebuild it with "bash database/migrate.sh --force"
```

## 约定

- `baseline.sql` **只由 `migrate.sh` 用**，不要手改；改内容就重出基线并 bump revision
- 脚本里不要写 `USE` / `CREATE DATABASE`：库名由命令行/`BF_DB_NAME` 决定
- `shop-patches/` 是历史工具，内容已并入基线，新库不需要导入
