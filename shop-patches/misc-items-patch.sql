-- ============================================================================
--  泡泡战士 CN0261 · 商店热更新补丁 · 杂项道具
--  内容：13 件 —— 能量水晶 4 + 强制起跳手镯 1 + 推进器 6（主 3 / 辅 3）+ 小饰品 2
--
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
--        （可在服务运行中执行；文件内不带 USE，库名写在命令行上）
--  幂等：每条 INSERT 都带 NOT EXISTS 守卫，重复执行无副作用
--  生效：不用重启服务端，客户端重新进商店即可看到
--
--  槽位与装备规则
--    水晶      slot 5  (sHeadBC)  occupied_slots = 1<<5   子类 202 头饰
--    强制起跳  slot 24 (sWristL)                        子类 204 前饰品
--    主推进器  slot 16 (sBodyB)                         子类 205 背饰品
--    辅助推进器 slot 27 (sPelvicB)                       子类 205 背饰品
--    小饰品    slot 14 / 16（sBodyF / sBodyB）           子类 204 / 205
--
--  销售窗口：客户端 durationFrom → 2060-01-01
--    （踩过的坑：照抄客户端 durationTo 会有商品早就过期，商店里直接看不到）
--  生成：2026-11-15（由 crystal-accessories + jump-bracelet + thrusters + accessories-2 合并）
-- ============================================================================
SET NAMES utf8mb4;

START TRANSACTION;

-- ==========================================================================
--  能量水晶（头饰槽 sHeadBC，激活条件：穿满 5 件 HC 装扮）
-- ==========================================================================

-- 100412 超级击爆加速水晶  slot=5(sHeadBC) 子类 202 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100412, '超级击爆加速水晶', 5, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100412);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100412, 32, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100412);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100412, 202, 227, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100412);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012245, 100412, 1, 160, 160, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012245);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012246, 100412, 7, 500, 500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012246);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012247, 100412, 30, 1000, 1000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012247);

-- 100431 信天翁翻滚水晶  slot=5(sHeadBC) 子类 202 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100431, '信天翁翻滚水晶', 5, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100431);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100431, 32, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100431);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100431, 202, 240, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100431);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012483, 100431, 7, 400, 400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012483);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012484, 100431, 30, 800, 800, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012484);

-- 100500 击爆加速水晶  slot=5(sHeadBC) 子类 202 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100500, '击爆加速水晶', 5, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100500);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100500, 32, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100500);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100500, 202, 275, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100500);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012248, 100500, 1, 150, 150, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012248);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012249, 100500, 7, 450, 450, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012249);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012250, 100500, 30, 900, 900, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012250);

-- 100546 怪物能量水晶  slot=5(sHeadBC) 子类 202 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100546, '怪物能量水晶', 5, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100546);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100546, 32, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100546);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100546, 202, 765, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100546);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013994, 100546, 1, 200, 200, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013994);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013996, 100546, 7, 590, 590, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013996);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013998, 100546, 30, 990, 990, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013998);

-- ==========================================================================
--  强制起跳手镯（腕部 sWristL）
-- ==========================================================================

-- 100224 强制起跳手镯  slot=24(sWristL) 子类 204 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100224, '强制起跳手镯', 24, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100224);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100224, 16777216, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100224);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100224, 204, 1000, 360, '2011-04-24 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100224);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012928, 100224, 1, 500, 500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012928);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012930, 100224, 7, 2000, 2000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012930);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012931, 100224, 30, 4000, 4000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012931);

-- ==========================================================================
--  推进器 · 主推进器（背部 sBodyB）
-- ==========================================================================

-- 100310 霸王HC推进器  slot=16(sBodyB) 子类 205 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100310, '霸王HC推进器', 16, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100310);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100310, 65536, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100310);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100310, 205, 184, 360, '2012-02-28 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100310);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012623, 100310, 1, 240, 240, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012623);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012625, 100310, 7, 720, 720, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012625);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012626, 100310, 30, 1450, 1450, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012626);

-- 101552 时空等离子推进器  slot=16(sBodyB) 子类 205 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 101552, '时空等离子推进器', 16, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 101552);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 101552, 134283264, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 101552);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 101552, 205, 218, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 101552);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2901551, 101552, 1, 390, 390, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2901551);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2901552, 101552, 7, 1170, 1170, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2901552);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2901553, 101552, 30, 2350, 2350, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2901553);

-- 110140 黄金液体推进器  slot=16(sBodyB) 子类 205 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 110140, '黄金液体推进器', 16, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 110140);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 110140, 65536, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 110140);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 110140, 205, 301, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 110140);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011762, 110140, 1, 160, 160, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011762);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011763, 110140, 7, 520, 520, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011763);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011764, 110140, 30, 1040, 1040, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011764);

-- ==========================================================================
--  推进器 · 辅助推进器（腰部 sPelvicB）
-- ==========================================================================

-- 100172 补助推进器  P01-S2  slot=27(sPelvicB) 子类 205 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100172, '补助推进器  P01-S2', 27, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100172);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100172, 134217728, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100172);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100172, 205, 91, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100172);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011495, 100172, 1, 300, 300, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011495);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011496, 100172, 7, 1200, 1200, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011496);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2901720, 100172, 30, 2400, 2400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2901720);

-- 100311 霸王HC辅助推进器  slot=27(sPelvicB) 子类 205 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100311, '霸王HC辅助推进器', 27, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100311);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100311, 134217728, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100311);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100311, 205, 185, 360, '2012-02-28 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100311);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012628, 100311, 1, 150, 150, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012628);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012630, 100311, 7, 450, 450, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012630);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012631, 100311, 30, 900, 900, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012631);

-- 110172 黄金辅助推进器  slot=27(sPelvicB) 子类 205 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 110172, '黄金辅助推进器', 27, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 110172);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 110172, 134217728, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 110172);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 110172, 205, 306, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 110172);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011765, 110172, 1, 150, 150, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011765);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011766, 110172, 7, 450, 450, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011766);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011767, 110172, 30, 900, 900, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011767);

-- ==========================================================================
--  小饰品（前饰品 / 背饰品）
-- ==========================================================================

-- 100085 降低致命的盾牌挂件  slot=14(sBodyF) 子类 204 货币 lucci
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100085, '降低致命的盾牌挂件', 14, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100085);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100085, 16384, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100085);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100085, 204, 36, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100085);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1300168, 100085, 7, 1000, 1000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1300168);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1300077, 100085, 30, 2000, 2000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1300077);

-- 100377 金色龟壳  slot=16(sBodyB) 子类 205 货币 cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100377, '金色龟壳', 16, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100377);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100377, 65536, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100377);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100377, 205, 214, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100377);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011884, 100377, 7, 2500, 2500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011884);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011713, 100377, 30, 5000, 5000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011713);

COMMIT;

-- ---------------------------------------------------------------------------
-- 自检：应该 13 行；水晶 4 / 手镯 1 / 推进器 6 / 饰品 2，每行 tiers 见右列
-- ---------------------------------------------------------------------------
SELECT i.item_id, d.name, d.equipment_slot AS slot, i.sub_category_id, i.currency,
       COUNT(s.id) AS tiers,
       GROUP_CONCAT(CONCAT(s.duration_days, '天/', s.sell_price) ORDER BY s.duration_days) AS prices
FROM shop_items i
JOIN item_definitions d ON d.id = i.item_id
LEFT JOIN shop_stocks s ON s.item_id = i.item_id
WHERE i.item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552, 110140, 100172, 100311, 110172, 100085, 100377)
GROUP BY i.item_id, d.name, d.equipment_slot, i.sub_category_id, i.currency
ORDER BY d.equipment_slot, i.item_id;
