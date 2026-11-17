# 数据库脚本

整个 schema 只有一个文件：`database/baseline.sql`。服务端启动时**不校验版本号**，而是
**比对表结构**（表 / 列 / 类型 / 可空性 / 键 / 自增）：

- **只改数据**（上架、改价、发卡、玩家账号…）→ 完全不校验，随便改，服务端照常启动
- **改结构**（加表、删列、改类型、改索引）→ 需要重新生成结构清单并重打服务端；
  漏了的话服务端会拒绝启动并列出差异

## 文件

| 文件 | 用途 |
|---|---|
| `baseline.sql` | 基线：22 张业务表 + 商店/角色/饰品数据 + `schema_version`。整库导入，会 DROP 同名表 |
| `migrate.sh` | 新装/重建：导入基线。目标库**非空**时默认拒绝（不看 `schema_version`，老库同样拦）；`--force` 会逐表清空后重建 |
| `../tools/db-schema-expected.sh` | 重新生成服务端用的结构清单（`internal/store/schema_expected.go`） |

## 新装

```bash
BF_DB_USER=bfserver BF_DB_PASSWORD='***' BF_DB_NAME=bubblefighter_cn0261 \
  bash database/migrate.sh 127.0.0.1 3306
```

## 重建（会清空数据）

```bash
mysqldump -u ... --single-transaction <库> | gzip > 备份.sql.gz      # 先备份
BF_DB_USER=bfserver BF_DB_PASSWORD='***' BF_DB_NAME=bubblefighter_cn0261 \
  bash database/migrate.sh 127.0.0.1 3306 --force
```

`--force` 的行为：

1. 先列出目标库当前的所有表并逐张 `DROP`（用 `SET FOREIGN_KEY_CHECKS=0` 包住），
   所以旧版本留下的 `schema_migrations`、用户自建表也会清掉；只授权了库级权限的账号
   也能重建（不需要 `DROP DATABASE`）。
2. 导入 `baseline.sql`。
3. 校验 `schema_version.revision` 与基线头部的 `-- schema-revision:` 一致。
4. 校验基线里每张 `CREATE TABLE` 出的表都存在（半截导入会被抓出来）；
   若库里还有基线之外的表，只打印一句提示，不判失败——服务端本身容忍多余表。

脚本的行为约定：

| 情况 | 结果 |
|---|---|
| 目标库为空 | 直接导入 |
| 目标库非空、无 `--force` | 拒绝并提示，退出码 1（**不会**静默覆盖） |
| 目标库非空、有 `--force` | 清空后重建 |
| 库名不是字母/数字/`_`/`-` | 拒绝（库名会拼进 SQL 标识符） |
| 端口不是数字、多余参数 | 拒绝 |
| 连不上、密码错、缺 `BF_DB_PASSWORD` | 拒绝并给出原因 |
| 导入后缺表 / revision 不符 | 报错，退出码 1 |

## 改结构时的流程（重要）

1. 在测试库上改结构，并验证服务端/接口
2. 重新生成结构清单（服务端的"合同"）：

   ```bash
   BF_DB_NAME=bubblefighter_cn0261 ./tools/db-schema-expected.sh > internal/store/schema_expected.go
   ```

3. 重出 `baseline.sql`，并把这个改动写进 `CHANGELOG.md`
4. 重打服务端二进制（清单是编译进去的）
5. 老库按上面的「重建」流程处理（当前没有升级通道）

> `schema_version.revision` 现在只是**标签**（后台展示、排障看基线批次），不参与启动判断；
> 想改就改，不会影响服务端。真正生效的契约是 `internal/store/schema_expected.go`。

## 启动时看到的结构报错

```
Startup error: database structure does not match this build (1 problem(s) across 127 expected columns):
missing column shop_items.display_order; rebuild it with "bash database/migrate.sh --force"
```

**多出来的表和列不算错**（允许后续平滑加字段）；缺表、缺列、类型/可空/键/自增不一致才算。

## 约定

- `baseline.sql` 只由 `migrate.sh` 用，不要手改；改内容就重出基线
- 脚本里不要写 `USE` / `CREATE DATABASE`：库名由命令行/`BF_DB_NAME` 决定
- 早期的 `shop-patches/` 目录已删除（内容全部并入基线；需要旧文件查 git 历史）
