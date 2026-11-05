# 数据库

建库后共 **23 张表**，全部由 `database/migrations/` 里的 14 份迁移创建，不需要手写任何建表语句。

## 迁移文件（14 份，必须按版本号顺序执行）

`database/migrate.sh` 会从每份 SQL 末尾登记的 `schema_migrations` 版本号自动排序，
不要按文件名首字母顺序手动执行（外键会失败）。Windows 上没有 bash，按
[手动安装 · 1.3](manual-install.md#13-初始化表结构14-份迁移) 里列出的 14 条命令顺序执行即可。

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

## 表结构（按用途分组）

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

## 运维：手工改余额

后台还没有管理界面，改金币/点券直接改库即可，**改完不用重启服务端**：

```sql
-- 按角色昵称：1000 万金币 + 1000 万点券
UPDATE character_stats SET lucci = 10000000, cash = 10000000,
       updated_at = UTC_TIMESTAMP(6)
WHERE character_id = (SELECT id FROM characters WHERE nickname = '角色名');

-- 按账号名（一个账号一个角色）
UPDATE character_stats SET lucci = 10000000, cash = 10000000,
       updated_at = UTC_TIMESTAMP(6)
WHERE character_id = (SELECT c.id FROM characters c
                      JOIN accounts a ON a.id = c.account_id
                      WHERE a.account_name = '账号名');

-- 加减（原子操作，不会覆盖别人的改动）
UPDATE character_stats SET lucci = lucci + 500000,
       updated_at = UTC_TIMESTAMP(6)
WHERE character_id = 42;
```

上限：`lucci` 结算封顶 1 亿（`store.MaxLucci`），`cash`/`lucci` 的列约束都是 `<= 2147483647`，
填超了整条 UPDATE 会失败。

**生效时机**（服务端本来就会重读余额并下发）：登录、在商店买一次东西、打完一局结算。
玩家若一直挂在广场不动，客户端要等这三种时机之一才会显示新数字。
