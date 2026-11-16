-- ============================================================================
--  回退脚本 · 投掷物 7 件（对应 grenades-patch.sql）
--  行为：清商店侧数据 + 回收玩家持有 + 删定义；不备份，已买的会被没收。
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
-- ============================================================================
SET NAMES utf8mb4;

-- 1) 预览：谁持有
SELECT ci.character_id, c.nickname, ci.item_id, d.name, ci.quantity, ci.equipped_slot
FROM character_items ci
JOIN characters c ON c.id = ci.character_id
LEFT JOIN item_definitions d ON d.id = ci.item_id
WHERE ci.item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);

START TRANSACTION;

-- 2) 回收持有
DELETE FROM character_item_expirations
 WHERE character_item_id IN (SELECT id FROM character_items WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054));
DELETE FROM character_items WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);
DELETE FROM character_weapon_preset_items WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);
DELETE FROM character_appearances WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);

-- 3) 清商店侧
DELETE FROM shop_purchases
 WHERE stock_id IN (SELECT id FROM shop_stocks WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054));
DELETE FROM shop_permanent_stocks
 WHERE stock_id IN (SELECT id FROM shop_stocks WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054));
DELETE FROM shop_preview_items WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);
DELETE FROM shop_stocks WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);
DELETE FROM shop_items WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);
DELETE FROM item_equipment_rules WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);
DELETE FROM item_definitions WHERE id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);

COMMIT;

-- 4) 自检：四个数都应为 0
SELECT
  (SELECT COUNT(*) FROM item_definitions WHERE id IN (205003,205004,205009,205005,205015,205017,205054)) AS defs_left,
  (SELECT COUNT(*) FROM shop_items WHERE item_id IN (205003,205004,205009,205005,205015,205017,205054)) AS shop_left,
  (SELECT COUNT(*) FROM shop_stocks WHERE item_id IN (205003,205004,205009,205005,205015,205017,205054)) AS stocks_left,
  (SELECT COUNT(*) FROM character_items WHERE item_id IN (205003,205004,205009,205005,205015,205017,205054)) AS bags_left;
