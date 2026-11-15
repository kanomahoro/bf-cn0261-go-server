-- ============================================================================
--  泡泡战士 CN0261 · 回退脚本 · 杂项道具
--  对应补丁：misc-items-patch.sql（13 件：水晶 4 / 手镯 1 / 推进器 6 / 饰品 2）
--
--  行为：先查谁买过 → 有就一同回收 → 再删商店数据。
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
--  幂等：可重复执行
--  ⚠ 回收没有备份，玩家已买的道具会被直接没收。
-- ============================================================================
SET NAMES utf8mb4;

-- ---------------------------------------------------------------------------
-- 1) 预览：谁持有、每件几档库存
-- ---------------------------------------------------------------------------
SELECT ci.character_id, c.nickname AS character_name, ci.item_id, d.name, ci.quantity, ci.equipped_slot
FROM character_items ci
JOIN characters c ON c.id = ci.character_id
LEFT JOIN item_definitions d ON d.id = ci.item_id
WHERE ci.item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

SELECT i.item_id, d.name,
       (SELECT COUNT(*) FROM shop_stocks s WHERE s.item_id = i.item_id) AS stocks,
       (SELECT COUNT(*) FROM shop_purchases p JOIN shop_stocks s2 ON s2.id = p.stock_id
         WHERE s2.item_id = i.item_id) AS purchases
FROM shop_items i
LEFT JOIN item_definitions d ON d.id = i.item_id
WHERE i.item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

START TRANSACTION;

-- ---------------------------------------------------------------------------
-- 2) 回收玩家持有
-- ---------------------------------------------------------------------------
DELETE FROM character_items
WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

DELETE FROM character_weapon_preset_items
WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

DELETE FROM character_appearances
WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

-- ---------------------------------------------------------------------------
-- 3) 清商店侧数据
-- ---------------------------------------------------------------------------
DELETE FROM shop_purchases
WHERE stock_id IN (SELECT id FROM shop_stocks
                   WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
                   110140, 100172, 100311, 110172, 100085, 100377));

DELETE FROM shop_permanent_stocks
WHERE stock_id IN (SELECT id FROM shop_stocks
                   WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
                   110140, 100172, 100311, 110172, 100085, 100377));

DELETE FROM shop_preview_items
WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

DELETE FROM shop_stocks
WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

DELETE FROM shop_items
WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

DELETE FROM item_equipment_rules
WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

DELETE FROM item_definitions
WHERE id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
        110140, 100172, 100311, 110172, 100085, 100377);

COMMIT;

-- ---------------------------------------------------------------------------
-- 4) 自检：四个数都应该是 0
-- ---------------------------------------------------------------------------
SELECT
  (SELECT COUNT(*) FROM item_definitions
    WHERE id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
          110140, 100172, 100311, 110172, 100085, 100377)) AS defs_left,
  (SELECT COUNT(*) FROM shop_items
    WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
          110140, 100172, 100311, 110172, 100085, 100377)) AS shop_left,
  (SELECT COUNT(*) FROM shop_stocks
    WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
          110140, 100172, 100311, 110172, 100085, 100377)) AS stocks_left,
  (SELECT COUNT(*) FROM character_items
    WHERE item_id IN (100412, 100431, 100500, 100546, 100224, 100310, 101552,
          110140, 100172, 100311, 110172, 100085, 100377)) AS in_bags_left;
