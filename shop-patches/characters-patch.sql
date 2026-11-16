-- ============================================================================
--  泡泡战士 CN0261 · 商店热更新补丁 · 新增两名可售角色（乖小爷 / 葱头）
--
--  背景：客户端 ShopItem2 的 Characters 子类共 20 件，我们只上架了 11 种角色里的 9 件
--        （皮蛋/黑妞免费发放）。本补丁补上此前缺失、且有原生价格的两名：
--          300019 乖小爷（character_type 19，库存三档在售：1天/800、7天/1900、30天/3900）
--          300007 葱头  （character_type 7，客户端库存档全部下架，按原生价自定：
--                        1天/500、7天/1200、30天/2400）
--
--  为什么不止插商品：
--    1. character_appearances 决定「哪张卡 = 哪个角色身体」，缺行则 63 槽拿不到模型
--    2. item_equipment_rules.character_mask 决定「这件东西能被哪些角色穿」。
--       存量 215 行的掩码只有 11967 / 703 / 1727 三种，全都不含 type 7(1<<6=64)
--       与 type 19(1<<18=262144) 的位。不加位的话，切成新角色后每次读背包
--       （NormalizeAppearance 在 Items() 里跑）都会把非 63 槽装备全部脱下，
--       武器预设也会被清空。所以这里把 215 行都「只加位」，对原有 11 种角色零影响。
--
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
--  幂等：INSERT 带 NOT EXISTS；UPDATE 带「位还没加」的判断，可重复执行
--  回退：mariadb --default-character-set=utf8mb4 <库名> < characters-rollback.sql
--
--  ⚠ 前提（我无法在服务端确认）：客户端里要有 type 7 / 19 的模型资源，
--     否则换装后可能无角色/黑屏。上架前先在客户端确认能看到这两个角色。
-- ============================================================================
SET NAMES utf8mb4;

START TRANSACTION;

-- ---------------------------------------------------------------------------
--  1) 掩码加宽：存量 215 条规则加上 type 19 / type 7 的位（只加，不减）
--     对原有 11 种角色完全没有影响
-- ---------------------------------------------------------------------------
UPDATE item_equipment_rules
   SET character_mask = character_mask | CAST(1 AS UNSIGNED) << 18   -- type 19
 WHERE (character_mask & (CAST(1 AS UNSIGNED) << 18)) = 0;

UPDATE item_equipment_rules
   SET character_mask = character_mask | CAST(1 AS UNSIGNED) << 6    -- type 7
 WHERE (character_mask & (CAST(1 AS UNSIGNED) << 6)) = 0;

-- ---------------------------------------------------------------------------
--  2) 道具定义（角色卡穿 63 槽）
-- ---------------------------------------------------------------------------
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 300019, '乖小爷', 63, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 300019);

INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 300007, '葱头', 63, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 300007);

-- ---------------------------------------------------------------------------
--  3) 外观映射（item_id ↔ character_type），缺它就没有 63 槽模型
-- ---------------------------------------------------------------------------
INSERT INTO character_appearances (item_id, character_type)
SELECT 300019, 19
WHERE NOT EXISTS (SELECT 1 FROM character_appearances WHERE item_id = 300019);

INSERT INTO character_appearances (item_id, character_type)
SELECT 300007, 7
WHERE NOT EXISTS (SELECT 1 FROM character_appearances WHERE item_id = 300007);

-- ---------------------------------------------------------------------------
--  4) 装备规则：占 63 槽（1<<63）；掩码 = 原 11967 + type 7 + type 19 = 274175
--     （角色卡对所有身体放开，与已有 11 张卡一致）
-- ---------------------------------------------------------------------------
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 300019, 9223372036854775808, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 300019);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 300007, 9223372036854775808, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 300007);

-- ---------------------------------------------------------------------------
--  5) 商店参数（子类 402 Characters；display_order 沿用客户端）
-- ---------------------------------------------------------------------------
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency, duration_type)
SELECT 300019, 402, 2002, 360, '2009-01-01 00:00:00', '2060-01-01 00:00:00', 1, 'cash_cn', 'day'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 300019);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency, duration_type)
SELECT 300007, 402, 529, 360, '2009-01-01 00:00:00', '2060-01-01 00:00:00', 1, 'cash_cn', 'day'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 300007);

-- ---------------------------------------------------------------------------
--  6) 库存档（按天卖；id 用 2931021 起的合成段）
--     300019 乖小爷：客户端原生在售三档
--     300007 葱头  ：客户端三档全下架，按原生价格上架
-- ---------------------------------------------------------------------------
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931021, 300019, 1, 800, 800, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931021);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931022, 300019, 7, 1900, 1900, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931022);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931023, 300019, 30, 3900, 3900, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931023);

INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931024, 300007, 1, 500, 500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931024);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931025, 300007, 7, 1200, 1200, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931025);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931026, 300007, 30, 2400, 2400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931026);

COMMIT;

-- ---------------------------------------------------------------------------
--  7) 自检
--     · 两张卡：定义 / 外观 / 规则 / 商品 / 档位 都齐
--     · masked_rules 应等于 item_equipment_rules 总行数（每行都含 type19 与 type7 位）
-- ---------------------------------------------------------------------------
SELECT i.item_id, d.name, a.character_type, r.character_mask, i.currency,
       i.max_duration_days, i.is_on_sale,
       (SELECT COUNT(*) FROM shop_stocks s WHERE s.item_id = i.item_id AND s.is_on_sale = 1) AS tiers,
       (SELECT GROUP_CONCAT(CONCAT(s.duration_days, '天/', s.sell_price) ORDER BY s.duration_days)
          FROM shop_stocks s WHERE s.item_id = i.item_id AND s.is_on_sale = 1) AS prices
FROM shop_items i
JOIN item_definitions d ON d.id = i.item_id
LEFT JOIN character_appearances a ON a.item_id = i.item_id
LEFT JOIN item_equipment_rules r ON r.item_id = i.item_id
WHERE i.item_id IN (300019, 300007)
ORDER BY i.display_order;

SELECT
  (SELECT COUNT(*) FROM item_equipment_rules)                                            AS rules_total,
  (SELECT COUNT(*) FROM item_equipment_rules
    WHERE (character_mask & (CAST(1 AS UNSIGNED) << 18)) <> 0)                           AS masked_type19,
  (SELECT COUNT(*) FROM item_equipment_rules
    WHERE (character_mask & (CAST(1 AS UNSIGNED) << 6)) <> 0)                            AS masked_type7,
  (SELECT COUNT(*) FROM character_appearances)                                           AS appearances;
