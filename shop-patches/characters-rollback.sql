-- ============================================================================
--  回退脚本 · 新增的两名可售角色（乖小爷 300019 / 葱头 300007）
--
--  行为：撤掉商店侧数据、外观映射、装备规则、道具定义，并把掩码里加过的位清掉。
--  ⚠ 会没收玩家已买的卡；若有角色正穿着这两张卡，会被一并脱掉（63 槽置空）。
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
-- ============================================================================
SET NAMES utf8mb4;

-- 1) 预览：谁持有 / 谁正穿着
SELECT ci.character_id, c.nickname, ci.item_id, d.name, ci.equipped_slot
FROM character_items ci
JOIN characters c ON c.id = ci.character_id
LEFT JOIN item_definitions d ON d.id = ci.item_id
WHERE ci.item_id IN (300019, 300007);

START TRANSACTION;

-- 2) 回收持有与过期行
DELETE FROM character_item_expirations
 WHERE character_item_id IN (SELECT id FROM character_items WHERE item_id IN (300019, 300007));
DELETE FROM character_items WHERE item_id IN (300019, 300007);
DELETE FROM character_weapon_preset_items WHERE item_id IN (300019, 300007);

-- 3) 商店侧
DELETE FROM shop_purchases
 WHERE stock_id IN (SELECT id FROM shop_stocks WHERE item_id IN (300019, 300007));
DELETE FROM shop_permanent_stocks
 WHERE stock_id IN (SELECT id FROM shop_stocks WHERE item_id IN (300019, 300007));
DELETE FROM shop_preview_items WHERE item_id IN (300019, 300007);
DELETE FROM shop_stocks WHERE item_id IN (300019, 300007);
DELETE FROM shop_items WHERE item_id IN (300019, 300007);

-- 4) 规则 / 外观 / 定义（注意顺序：外观和规则都指向 item_definitions）
DELETE FROM item_equipment_rules WHERE item_id IN (300019, 300007);
DELETE FROM character_appearances WHERE item_id IN (300019, 300007);
DELETE FROM item_definitions WHERE id IN (300019, 300007);

-- 5) 掩码回到补丁前：按当前值反推原来的几组
--    注意：274175 包含第 1.1 节的兜底结果（通用装备也变成了 274175），
--    所以这一步会把通用装备一起退回 11967（即新角色又穿不上）—— 这是预期行为
UPDATE item_equipment_rules SET character_mask = 11967 WHERE character_mask = 274175;
UPDATE item_equipment_rules SET character_mask = 703   WHERE character_mask IN (262911, 767);
UPDATE item_equipment_rules SET character_mask = 1727  WHERE character_mask = 263935;


-- 6) 昨天那套 HC 衣服（clothes-patch.sql）的掩码回到它上线时的值
UPDATE item_equipment_rules SET character_mask = 703
 WHERE item_id IN (
  100571, 100572, 100573, 100574, 100575, 101265, 101266, 101267,
  101268, 101269
 );
UPDATE item_equipment_rules SET character_mask = 9277
 WHERE item_id IN (
  100658, 100659, 100660, 100661, 100662, 100790, 100791, 100792,
  100793, 100794
 );
UPDATE item_equipment_rules SET character_mask = 9919
 WHERE item_id IN (
  100584, 100585, 100586, 100587, 100588
 );
UPDATE item_equipment_rules SET character_mask = 11967
 WHERE item_id IN (
  100566, 100567, 100568, 100569, 100570, 100800, 100801, 100802,
  100803, 100804
 );

COMMIT;

-- 7) 自检：四个数都应为 0
SELECT
  (SELECT COUNT(*) FROM item_definitions WHERE id IN (300019, 300007))          AS defs_left,
  (SELECT COUNT(*) FROM character_appearances WHERE item_id IN (300019, 300007)) AS appearances_left,
  (SELECT COUNT(*) FROM shop_items WHERE item_id IN (300019, 300007))           AS shop_left,
  (SELECT COUNT(*) FROM character_items WHERE item_id IN (300019, 300007))      AS bags_left;
