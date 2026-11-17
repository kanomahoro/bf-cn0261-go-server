# 数据库脚本（schema revision 3）

服务端不再"跑迁移文件"，而是校验一个**版本号**：数据库里的 `schema_version.revision`
必须等于二进制编译时要求的 revision（`internal/store/upgrade.go` 的 `SchemaRevision`）。
不一致时服务端直接拒绝启动，并提示跑哪个脚本。

## 目录

| 文件 | 用途 |
|---|---|
| `baseline.sql` | **新装**基线（schema revision 3）：22 张业务表 + 商店/角色/饰品数据 + `schema_version`。整库导入，会 DROP 同名表 |
| `upgrades/NNN-*.sql` | **老库**增量升级，每个文件头部有 `-- target-revision: N`；按文件名顺序、幂等地执行 |
| `migrate.sh` | 新装/重建：导 `baseline.sql`（库里有 `schema_version` 时默认拒绝，需 `--force`） |
| `upgrade.sh` | 老库升级：先 `mysqldump` 备份，再按 revision 依次执行 `upgrades/*.sql` |
| `backups/` | `upgrade.sh` 自动备份落在这里（不进版本库） |

## revision 模型

| revision | 含义 |
|---|---|
| 1 | 14 份历史迁移建出来的老库（没有 `schema_version` 表） |
| 2 | `001-shop-convergence.sql`：`duration_type` 列 + 五个商店补丁 + 掩码收敛 + 清掉预览商品 |
| 3 | `002-accessories.sql`：功能饰品 14 件 |

- 新装 → revision 3（`baseline.sql` 里写死）
- 老库 → `upgrade.sh` 从当前 revision 逐级升到 3
- 服务端 → `SchemaRevision = 3`；其余 revision 一律拒绝启动

## 新装

```bash
BF_DB_USER=bfserver BF_DB_PASSWORD='***' BF_DB_NAME=bubblefighter_cn0261 \
  bash database/migrate.sh 127.0.0.1 3306
```

已存在的库要重建时加 `--force`（会 DROP 表，先自己备份）。

## 老库升级

```bash
BF_DB_USER=bfserver BF_DB_PASSWORD='***' BF_DB_NAME=bubblefighter_cn0261 \
  bash database/upgrade.sh 127.0.0.1 3306
# 只看计划不动库：加 --dry-run；不想备份：加 --no-backup
```

脚本会：读当前 revision → 列计划 → `mysqldump` 备份到 `database/backups/` →
逐个执行 → 校验 revision。**升级前先停服务端**。

也可以用服务端自带的自动升级（默认关闭）：

```bash
./bfserver --auto-upgrade --database-dir database
```

它只跑 `upgrades/*.sql` 里 revision 高于库的那几个，然后再做一次 revision 校验；
失败时按"文件 + 第几条语句"报错，脚本都是幂等的，修好可直接重跑。

## 加一个新的 revision（改库时）

1. 写 `upgrades/003-描述.sql`，第一段带上 `-- target-revision: 4`，全部语句幂等
   （`INSERT ... WHERE NOT EXISTS` / `ALTER ... ADD COLUMN IF NOT EXISTS` / 按清单 `DELETE`）
2. 文件末尾写版本落章：

   ```sql
   INSERT INTO schema_version (id, revision, note, applied_at)
   VALUES (1, 4, 'upgrade 003: ...', UTC_TIMESTAMP(6))
   ON DUPLICATE KEY UPDATE revision = VALUES(revision), note = VALUES(note), applied_at = VALUES(applied_at);
   ```

3. 改 `internal/store/upgrade.go` 的 `SchemaRevision = 4`
4. 用临时库验两条路收敛：`老库 + upgrade.sh` 与 `新 baseline`（每 3~5 个版本重出一次 baseline）
5. 同步 `CHANGELOG.md`、根 `README.md` 里的 revision 说明

## 约定

- **`baseline.sql` 冻结**：只由 `migrate.sh` 用；要改内容就出新 revision，别手改它
- **不要删老升级文件**：`upgrade.sh` 靠文件名顺序 + `target-revision` 判断，重复执行靠幂等
- **升级脚本里不要写 `USE` / `CREATE DATABASE`**：库名由命令行 `$db` 决定
- `shop-patches/` 保留为历史工具（对应的内容已并入 revision 2/3），新库不再需要导入
