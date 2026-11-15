# 商店数据补丁（2026-11-15）

数据库热更新，执行完即生效：**不用重新编译、不用重启服务端**，玩家重新进商店就能看到。

## 文件

| 文件 | 内容 | 件数 |
|---|---|---|
| `clothes-patch.sql` | 衣服补丁：HC 五件套 8 套（帽/上衣/裤/手套/鞋各 8 件） | 40 |
| `weapons-patch.sql` | 武器补丁（步枪 10 / 散弹 9 / 近战 4 / 狙击 5 / 手枪 3） | 31 |
| `misc-items-patch.sql` | 杂项：能量水晶 4 + 强制起跳手镯 1 + 推进器 6 + 小饰品 2 | 13 |
| `grant-starter-cards.sql` | 维护脚本：全服补发角色卡「皮蛋 / 黑妞」（**不是商店补丁**） | 2 张卡 |
| `*-rollback.sql` | 上面三个 patch 各自对应的回退 | — |

合计 **84 件道具、218 条库存档位**。

## 导入

```bash
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < clothes-patch.sql
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < weapons-patch.sql
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < misc-items-patch.sql
```

- 顺序随意，三者无依赖
- 默认以当前系统用户连接；以 root 执行即 `root@localhost` 走 socket 免密
- 每条 INSERT 带 `WHERE NOT EXISTS`，重复执行无副作用
- 每个文件末尾打印自检 SELECT，行数对得上就成功

## 回退

```bash
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < clothes-rollback.sql
```

先预览谁买过 → 回收玩家持有 → 清商店侧数据 → 自检四个数归零。
⚠️ **不备份**，已买的道具会被直接没收。

## 三条要记住的

1. **衣服这 8 套是 HC 装扮，作用是激活能量水晶**：客户端 `PPAequipCheckValue = 5`，即穿满 5 件 HC 才生效；
   只有 `desc2` 标注「能量水晶专用HC装扮」的才算真 HC（名字带 HC 不算）。
2. **角色卡不上商店**：皮蛋/黑妞用 `grant-starter-cards.sql` 直接补进背包（真永久、幂等、不动已装备的角色）。
3. **武器定价已知问题（未修）**：客户端 HC 与普通版数值相同但 HC 更便宜（1380 vs 4500），
   且 16 件客户端货币是 `cashCN`、补丁写成 `lucci`。要修另出重定价补丁。

## 写补丁的规矩

`WHERE NOT EXISTS` 守卫 + 单事务 + `SET NAMES utf8mb4` + 不带 `USE` + 不写 `schema_migrations` + 结尾自检；
插入顺序 `item_definitions` → `item_equipment_rules` → `shop_items` → `shop_stocks`；
`sale_ends_at` 一律 `2060-01-01`（照抄客户端 `durationTo` 会因过期在商店里消失）。

更详细的分组说明与校验记录见仓库历史版本或 `git log`。
