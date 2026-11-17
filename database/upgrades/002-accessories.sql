-- ============================================================================
-- target-revision: 3
--  升级 002：schema revision 2 → 3（功能饰品 14 件）
--
--  内容：14 件功能饰品的道具定义 / 槽位规则 / 商店参数 / 30 条档位。
--  货币：全部 lucci（运营决定：统一游戏币，不跟客户端的 cashCN）
--  掩码：274175（客户端这 14 件都是 allowChr=all / denyChr=none）
--  定价与上架期：运营自定（不是照抄客户端），含两件客户端已停售的延长至 2060。
--
--  幂等：所有 INSERT 带 NOT EXISTS；重复执行无副作用。
--  来源：master 提供的 accessories-patch（2026-11-16），本仓库按统一口径修订掩码。
--  用法：bash database/upgrade.sh（先备份），或服务端 --auto-upgrade。
-- ============================================================================
SET NAMES utf8mb4;

START TRANSACTION;

-- ==========================================================================
--  视野类饰品（5 件）
-- ==========================================================================

-- 100068 幽灵护目镜  slot=10(sEye+sMask) 子类 203 货币 lucci occupied=1536
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100068, '幽灵护目镜', 10, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100068);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100068, 1536, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100068);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100068, 203, 24, 360, '2011-08-10 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100068);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011324, 100068, 7, 1200, 1200, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011324);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011325, 100068, 30, 2400, 2400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011325);

-- 100221 HBS瞄准眼镜  slot=10(sEye+sMask) 子类 203 货币 lucci occupied=1536
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100221, 'HBS瞄准眼镜', 10, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100221);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100221, 1536, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100221);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100221, 203, 139, 360, '2009-01-01 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100221);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011330, 100221, 7, 1500, 1500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011330);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011331, 100221, 30, 3000, 3000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011331);

-- 100342 超级明星☆黑墨镜  slot=10(sEye+sMask) 子类 203 货币 lucci occupied=1536
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100342, '超级明星☆黑墨镜', 10, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100342);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100342, 1536, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100342);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100342, 203, 1006, 360, '2011-03-01 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100342);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013055, 100342, 7, 1500, 1500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013055);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013056, 100342, 30, 3000, 3000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013056);

-- 100642 WATT汽动护目镜  slot=10(sEye+sMask) 子类 203 货币 lucci occupied=1536
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100642, 'WATT汽动护目镜', 10, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100642);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100642, 1536, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100642);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100642, 203, 300294, 360, '2009-01-01 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100642);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013541, 100642, 7, 1500, 1500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013541);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013542, 100642, 30, 3000, 3000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013542);

-- 100668 滑雪护目镜  slot=10(sEye+sMask) 子类 203 货币 lucci occupied=1536
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100668, '滑雪护目镜', 10, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100668);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100668, 1536, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100668);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100668, 203, 300296, 360, '2013-01-08 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100668);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013632, 100668, 7, 1500, 1500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013632);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013633, 100668, 30, 3000, 3000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013633);

-- ==========================================================================
--  臂章 / 护腕（2 件）
-- ==========================================================================

-- 100253 爆烈达人臂章  slot=13(sElbowR) 子类 204 货币 lucci occupied=8192
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100253, '爆烈达人臂章', 13, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100253);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100253, 8192, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100253);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100253, 204, 159, 360, '2011-12-08 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100253);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012046, 100253, 7, 2500, 2500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012046);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012047, 100253, 30, 5000, 5000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012047);

-- 100423 闪闪五星护腕  slot=13(sElbowR) 子类 204 货币 lucci occupied=8192
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100423, '闪闪五星护腕', 13, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100423);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100423, 8192, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100423);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100423, 204, 238, 360, '2011-10-18 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100423);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011772, 100423, 7, 2500, 2500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011772);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011773, 100423, 30, 5000, 5000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011773);

-- ==========================================================================
--  头饰（2 件）
-- ==========================================================================

-- 100372 幸运海星  slot=1(sHeadUC) 子类 202 货币 lucci occupied=2
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100372, '幸运海星', 1, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100372);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100372, 2, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100372);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100372, 202, 209, 360, '2011-09-13 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100372);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011572, 100372, 7, 1000, 1000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011572);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011573, 100372, 30, 2000, 2000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011573);

-- 100376 微笑章鱼  slot=1(sHeadUC) 子类 202 货币 lucci occupied=2
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100376, '微笑章鱼', 1, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100376);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100376, 2, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100376);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100376, 202, 213, 360, '2011-09-06 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100376);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011569, 100376, 7, 1500, 1500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011569);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011570, 100376, 30, 2500, 2500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011570);

-- ==========================================================================
--  腰饰（1 件）
-- ==========================================================================

-- 100257 甜蜜的巧克力宝箱  slot=15(sPelvic) 子类 204 货币 lucci occupied=32768
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100257, '甜蜜的巧克力宝箱', 15, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100257);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100257, 32768, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100257);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100257, 204, 162, 360, '2009-01-01 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100257);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012588, 100257, 7, 1000, 1000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012588);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1012589, 100257, 30, 2000, 2000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1012589);

-- ==========================================================================
--  尾饰（2 件）
-- ==========================================================================

-- 100578 魔幻泡泡（红色）  slot=17(sTail) 子类 205 货币 lucci occupied=131072
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100578, '魔幻泡泡（红色）', 17, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100578);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100578, 131072, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100578);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100578, 205, 300113, 360, '2011-08-26 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100578);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013179, 100578, 7, 2400, 2400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013179);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013180, 100578, 30, 4800, 4800, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013180);

-- 190049 魔幻泡泡(黄金)  slot=17(sTail) 子类 205 货币 lucci occupied=131072
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 190049, '魔幻泡泡(黄金)', 17, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 190049);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 190049, 131072, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 190049);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 190049, 205, 300557, 360, '2013-10-01 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 190049);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015026, 190049, 7, 2400, 2400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015026);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015027, 190049, 30, 4800, 4800, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015027);

-- ==========================================================================
--  复活类（2 件）
-- ==========================================================================

-- 100142 快速复活手表  slot=24(sWristL) 子类 204 货币 lucci occupied=16777216
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100142, '快速复活手表', 24, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100142);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100142, 16777216, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100142);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100142, 204, 73, 360, '2011-10-25 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100142);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2930001, 100142, 1, 500, 500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2930001);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2930002, 100142, 7, 2000, 2000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2930002);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2930003, 100142, 30, 4000, 4000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2930003);

-- 101161 浣碧桃心手环  slot=24(sWristL) 子类 204 货币 lucci occupied=16777216
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 101161, '浣碧桃心手环', 24, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 101161);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 101161, 16777216, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 101161);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 101161, 204, 422, 360, '2009-01-01 00:00:00', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 101161);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2930004, 101161, 1, 500, 500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2930004);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2930005, 101161, 7, 2000, 2000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2930005);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2930006, 101161, 30, 4000, 4000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2930006);

COMMIT;

-- ---------------------------------------------------------------------------
-- 自检：应该 14 行
-- ---------------------------------------------------------------------------
SELECT i.item_id, d.name, d.equipment_slot AS slot, i.sub_category_id, i.currency,
       COUNT(s.id) AS tiers,
       GROUP_CONCAT(CONCAT(s.duration_days, '天/', s.sell_price) ORDER BY s.duration_days) AS prices
FROM shop_items i
JOIN item_definitions d ON d.id = i.item_id
LEFT JOIN shop_stocks s ON s.item_id = i.item_id
WHERE i.item_id IN (100068, 100221, 100342, 100642, 100668, 100253, 100423, 100372, 100376, 100257, 100578, 190049, 100142, 101161)
GROUP BY i.item_id, d.name, d.equipment_slot, i.sub_category_id, i.currency
ORDER BY d.equipment_slot, i.item_id;

-- ==========================================================================
--  版本落章：revision = 3
-- ==========================================================================
INSERT INTO schema_version (id, revision, note, applied_at)
VALUES (1, 3, 'upgrade 002: accessories', UTC_TIMESTAMP(6))
ON DUPLICATE KEY UPDATE revision = VALUES(revision), note = VALUES(note), applied_at = VALUES(applied_at);

-- 自检：饰品 14 件、库存 30 条、revision 3
SELECT
  (SELECT COUNT(*) FROM item_definitions WHERE id IN (100068,100142,100221,100253,100257,100342,100372,100376,100423,100578,100642,100668,101161,190049)) AS defs,
  (SELECT COUNT(*) FROM shop_items      WHERE item_id  IN (100068,100142,100221,100253,100257,100342,100372,100376,100423,100578,100642,100668,101161,190049)) AS shop_rows,
  (SELECT COUNT(*) FROM shop_stocks     WHERE item_id  IN (100068,100142,100221,100253,100257,100342,100372,100376,100423,100578,100642,100668,101161,190049)) AS stocks,
  (SELECT revision FROM schema_version WHERE id = 1) AS revision;
