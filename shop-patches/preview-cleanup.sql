-- ============================================================================
--  泡泡战士 CN0261 · 商店热更新补丁 · 清理「预览商品」（只展示不售卖的那批）
--
--  背景：v14 迁移在 `shop_preview_items` 里登记了 38 件 display-only 商品
--        （喷剂、喇叭、技能卡、宠物粮、戒指、跟宠…）。它们只能看不能买，
--        挤在商店列表里。本补丁把这一批从商店里整批清掉。
--
--  做法：按外键顺序删 shop_purchases → shop_permanent_stocks → shop_stocks
--        → shop_preview_items → shop_items
--        **不删 item_definitions**：玩家背包/装扮/预设里已有的这些道具原样保留
--        （脚本不再做备份，删除前请自行 mysqldump 需要的表）
--
--  幂等：清单取自临时表，删完第二次执行是空操作
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
--
--  ⚠ 只想让这批商品「能买」而不是「下架」的话，别用这个脚本，
--     要的是 `DELETE FROM shop_preview_items WHERE item_id IN (...);`（见文件末尾注释）
-- ============================================================================
SET NAMES utf8mb4;

-- 待清理清单（本次运行开始时的预览商品；第二次执行时为空）
DROP TEMPORARY TABLE IF EXISTS tmp_preview_items;
CREATE TEMPORARY TABLE tmp_preview_items (
  item_id         INT UNSIGNED NOT NULL PRIMARY KEY,
  name            VARCHAR(128) NULL,
  currency        VARCHAR(8)   NULL,
  sub_category_id INT UNSIGNED NULL,
  display_order   INT UNSIGNED NULL
);
INSERT INTO tmp_preview_items (item_id, name, currency, sub_category_id, display_order)
SELECT p.item_id, d.name, i.currency, i.sub_category_id, i.display_order
FROM shop_preview_items p
LEFT JOIN item_definitions d ON d.id = p.item_id
LEFT JOIN shop_items i ON i.item_id = p.item_id;

-- 清理前留个记录，便于核对清理了多少
SELECT COUNT(*) AS preview_to_clean,
       (SELECT COUNT(*) FROM shop_stocks WHERE item_id IN (SELECT item_id FROM tmp_preview_items)) AS stocks_to_clean
FROM tmp_preview_items;

START TRANSACTION;

-- shop_purchases / shop_permanent_stocks 指向 shop_stocks
DELETE FROM shop_purchases
 WHERE stock_id IN (SELECT id FROM shop_stocks WHERE item_id IN (SELECT item_id FROM tmp_preview_items));

DELETE FROM shop_permanent_stocks
 WHERE stock_id IN (SELECT id FROM shop_stocks WHERE item_id IN (SELECT item_id FROM tmp_preview_items));

DELETE FROM shop_stocks
 WHERE item_id IN (SELECT item_id FROM tmp_preview_items);

-- shop_preview_items 也指向 shop_items，必须先摘掉再删商品行
DELETE FROM shop_preview_items
 WHERE item_id IN (SELECT item_id FROM tmp_preview_items);

DELETE FROM shop_items
 WHERE item_id IN (SELECT item_id FROM tmp_preview_items);

COMMIT;

-- 自检
--   preview_left / shop_rows_left / stocks_left 应为 0
--   defs_left 是道具定义（**保留**，非 0 才是对的）
--   bag_left 是玩家背包里这些道具的行数（**不应变化**）
SELECT
  (SELECT COUNT(*) FROM shop_preview_items)                       AS preview_left,
  (SELECT COUNT(*) FROM shop_items
    WHERE item_id IN (SELECT item_id FROM tmp_preview_items))     AS shop_rows_left,
  (SELECT COUNT(*) FROM shop_stocks
    WHERE item_id IN (SELECT item_id FROM tmp_preview_items))     AS stocks_left,
  (SELECT COUNT(*) FROM item_definitions
    WHERE id IN (SELECT item_id FROM tmp_preview_items))          AS defs_left,
  (SELECT COUNT(*) FROM character_items
    WHERE item_id IN (SELECT item_id FROM tmp_preview_items))     AS bag_left,
  (SELECT COUNT(*) FROM tmp_preview_items)                        AS cleaned;

-- 明细：清掉了哪些商品
SELECT item_id, name, currency, sub_category_id, display_order
FROM tmp_preview_items
ORDER BY item_id;

DROP TEMPORARY TABLE IF EXISTS tmp_preview_items;

-- ---------------------------------------------------------------------------
--  【另一种需求，别和上面的混用】只取消预览标记 → 这批商品变成「可以买」：
--  （执行后商品仍在商店里，可以正常购买；想撤回就重新插回 shop_preview_items）
--
--  DELETE FROM shop_preview_items
--   WHERE item_id IN (100064, 100065, 100080, 100082, 100083,
--                     100090, 100091, 100104, 100226, 100251, 100252,
--                     100484, 100485, 100547, 100548, 100604, 100622,
--                     100623, 100624, 100625, 100626, 100627, 100628,
--                     100629, 100641, 100643, 100644, 100652, 100819,
--                     100984, 101034, 101050, 101062, 101164, 190074,
--                     301001, 301003, 301004);
-- ---------------------------------------------------------------------------
