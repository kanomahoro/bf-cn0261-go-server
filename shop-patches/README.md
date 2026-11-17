# 商店数据补丁（2026-11-15）

数据库热更新，执行完即生效：**不用重新编译、不用重启服务端**，玩家重新进商店就能看到。

## 文件

| 文件 | 内容 | 件数 |
|---|---|---|
| `clothes-patch.sql` | 衣服补丁：HC 五件套 8 套（帽/上衣/裤/手套/鞋各 8 件） | 40 |
| `weapons-patch.sql` | 武器补丁（步枪 10 / 散弹 9 / 近战 4 / 狙击 5 / 手枪 3） | 31 |
| `misc-items-patch.sql` | 杂项：能量水晶 4 + 强制起跳手镯 1 + 推进器 6 + 小饰品 2 | 13 |
| `grenades-patch.sql` | 投掷物：手雷 / 粘稠手雷 / 地雷 7 件（子类 307，按个数卖） | 7 |
| `characters-patch.sql` | 角色卡：乖小爷(300019) / 葱头(300007)，含外观映射与装备掩码加宽 | 2 |
| `grant-starter-cards.sql` | 维护脚本：全服补发角色卡「皮蛋 / 黑妞」（**不是商店补丁**） | 2 张卡 |
| `preview-cleanup.sql` | 清理脚本：把 38 件「只展示不售卖」的预览商品从商店整批删掉 | -38 |
| `*-rollback.sql` | 上面几个 patch 各自对应的回退（`preview-cleanup.sql` 不备份、无回退） | — |

合计 **93 件道具、244 条库存档位**（再加上 `preview-cleanup.sql` 的清理效果）。

## 导入

```bash
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < clothes-patch.sql
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < weapons-patch.sql
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < misc-items-patch.sql
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < grenades-patch.sql
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < characters-patch.sql
# 可选：清掉只展示不售卖的预览商品
# mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < preview-cleanup.sql
```

- 顺序随意（2026-11-17 实测：正序和「characters 先导」两种顺序产出的 `item_equipment_rules` 完全一致）。
  `characters-patch.sql` 已自带 `duration_type` 的 `ALTER`，不再依赖 `grenades-patch` 先导
- 默认以当前系统用户连接；以 root 执行即 `root@localhost` 走 socket 免密
- 每条 INSERT 带 `WHERE NOT EXISTS`，重复执行无副作用
- 每个文件末尾打印自检 SELECT，行数对得上就成功

## 回退

```bash
mariadb --default-character-set=utf8mb4 bubblefighter_cn0261 < clothes-rollback.sql
```

先预览谁买过 → 回收玩家持有 → 清商店侧数据 → 自检四个数归零。
⚠️ **不备份**，已买的道具会被直接没收。

## 导入顺序

`characters-patch.sql` 与 `clothes-patch.sql` **没有先后依赖**（实测两种顺序结果完全一致）：
角色补丁里针对 HC 衣服的掩码校正，在衣服还没导入时是空操作；而改好后的 `clothes-patch.sql`
自身写的就是客户端掩码，所以先导哪个都对。

## 角色卡补丁的特别说明（`characters-patch.sql`）

新增角色不只是插商品：必须同时补 `character_appearances`（63 槽模型映射），并按客户端
`ShopItem2.xml` 的 `allowChr` / `denyChr`（每件装备都写明允许哪些角色穿）校正
`item_equipment_rules.character_mask`。不校正的话，换成新角色后每次读背包
（`NormalizeAppearance` 在 `Items()` 里跑）会把不该脱的装备脱掉。

按客户端数据换算（名字→类型：Dao=1 Dizni=2 Bazzi=3 Kephi=4 Uni=5 Mos=6 Ethi=7 Marid=8
LXMarid=10 Leo=11 Lavi=12 Twotop=14 LXUni=19），补丁一共改 **128 条**规则
（93 条普通装备 + 35 条 HC 套装件）：

| 组 | 件数 | 结果掩码 | 含义 |
|---|---|---|---|
| 衣服 A | 62 | 262911 | 葱头 + 乖小爷 都能穿（企鹅、小熊、夏威夷、中秋喜兔、万圣节、爱斯基摩、超级明星、红玫瑰、BF学校霸王、泡泡自由…） |
| 衣服 B | 26 | 767 | 只有葱头能穿（朋克熊、金星训练、风帽训练、闪闪五星、红玫瑰部分…），客户端 allowChr 不含 LXUni |
| BF冒险王 | 5 | 263935 | 五件套，两个角色都能穿 |
| HC 僵尸王 / 圣战骑士 | 10 | 274175 | 客户端 allowChr 是全 13 种（原 11967） |
| HC 法老王 / 爱斯基摩 | 10 | 262911 | 加 葱头 + 乖小爷（原 703） |
| HC 恶魔战士 | 5 | 272127 | 除拉比(12) 的 12 种，含 葱头 + 乖小爷（原 9919） |
| HC 犀利 / 流光绝影 | 10 | 271485 | 男性 9 种，含 葱头 + 乖小爷（原 9277） |
| HC 流光艳影 | 5 | 2690（不改） | 客户端限 黑妞/蓝蓝/蓝妹妹/拉比，两名新角色都不在内，保留原值 |

13 种以外（Leo/Lavi/Twopup 等被 allowChr 排除的组合）保持客户端原样，没有放开。

> ✅ **从零导入的库已修复（2026-11-17）**：以前迁移种子和部分补丁写的是 `11967`（旧版 11 种身体全开放），
> 不含 type 7 / 19，全新库上不给这些行补位的话，新角色一读背包就会被 `NormalizeAppearance` 把武器饰品全脱。
> 现在三处已对齐：
> 1. `characters-patch.sql` 末尾加了兜底 `UPDATE ... SET character_mask = 274175 WHERE character_mask = 11967`
> 2. 各补丁与迁移里的「全开放」掩码常量统一成 `274175`（13 种），`grenades-patch` 不再按
>    `character_appearances` 现算（那条写法会因导入顺序得到不同结果）
> 3. `characters-patch.sql` 自带 `shop_items.duration_type` 的 `ALTER`，可以单独先导
>
> 实测（空库跑 14 份迁移 + 5 个补丁）：掩码收敛为 `274175`(178) / `262911`(72) / `767`(26) /
> `271485`(10) / `263935`(5) / `272127`(5) / `2690`(5)，**`11967` 归零**；旧库（旧迁移 + 新补丁）结果相同。
> 自检看 `characters-patch.sql` 末尾那组数：`WHERE character_mask = 11967` 应为 0。

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
