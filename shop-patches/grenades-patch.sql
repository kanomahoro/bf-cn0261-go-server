-- ============================================================================
--  泡泡战士 CN0261 · 商店热更新补丁 · 投掷物（手雷 / 粘雷 / 地雷）7 件
--
--  背景：投掷物原先由服务端启动时 EnsureGrenadeItems() 直接塞进 item_definitions，
--        现在改成商店数据补丁，和武器补丁一样手动导入，玩家从商店购买。
--        新角色不再免费携带（建号逻辑已同步移除）。
--
--  内容：7 件商品 —— 定义 7 条 + 槽位规则 7 条 + 商店参数 7 条 + 库存档 20 条
--
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
--  幂等：每条 INSERT 带 NOT EXISTS 守卫；ALTER 带 IF NOT EXISTS；可重复执行
--  生效：不用重启服务端，客户端重新进商店即可看到
--
--  槽位：21（特殊武器槽，一次只能装备一件）occupied_slots = 1 << 21 = 2097152
--  ⚠ 这些商品都是「按个数卖」：durationType = count，shop_stocks.duration_days 是个数。
--        导入后 shop_items.duration_type 必须是 'count'（本文件末尾已处理），
--        否则购买路径会把数量当天数，买 50 个只到手 1 个。
--
--  商品一览（价格单位为点券/游戏币，见 currency 列）
--    205003 液体手雷E02      cash_cn  上限 10000 50个/100、120个/200、300个/400
--        ↳ 价格照抄客户端在售档
--    205004 粘稠手雷 E01-S   lucci    上限 10000 50个/1000、120个/2400、300个/6000
--        ↳ 客户端无在售档（历史活动价 5 个/100），按 20/个折算
--    205009 液态地雷SF01     cash_cn  上限 10000 50个/200、120个/400、300个/800
--        ↳ 价格照抄客户端在售档
--    205005 液态地雷M01      cash_cn  上限 10000 50个/200、120个/400、300个/800
--        ↳ 价格照抄客户端在售档
--    205015 泡泡棒球队手雷      cash_cn  上限 10000 50个/300、120个/600、300个/1200
--        ↳ 价格照抄客户端在售档
--    205017 液体雷达手雷R01    cash_cn  上限 10000 50个/400、120个/800、300个/1600
--        ↳ 价格照抄客户端在售档；武器 5017 带 EIZ 制导
--    205054 超级龙鳞手雷       cash_cn  上限 20000 50个/500、100个/900
--        ↳ 客户端原上架期 2009-01-01~2015-03-03 已结束，本补丁延长至 2060
-- ============================================================================
SET NAMES utf8mb4;

-- 服务端启动时也会跑这条（EnsureShopUnits），这里重复一次保证本补丁可单独执行。
ALTER TABLE shop_items
  ADD COLUMN IF NOT EXISTS duration_type ENUM('day','count') NOT NULL DEFAULT 'day'
  COMMENT 'day: shop_stocks.duration_days is days; count: it is a piece count'
  AFTER max_duration_days;

START TRANSACTION;

-- ---------------------------------------------------------------------------
--  道具定义（投掷物统一穿槽位 21）
-- ---------------------------------------------------------------------------
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 205003, '液体手雷E02', 21, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 205003);

INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 205004, '粘稠手雷 E01-S', 21, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 205004);

INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 205009, '液态地雷SF01', 21, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 205009);

INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 205005, '液态地雷M01', 21, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 205005);

INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 205015, '泡泡棒球队手雷', 21, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 205015);

INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 205017, '液体雷达手雷R01', 21, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 205017);

INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 205054, '超级龙鳞手雷', 21, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 205054);

-- ---------------------------------------------------------------------------
--  槽位规则：占特殊武器槽 21；角色掩码固定 274175（13 种身体全开放）
--            （早期写法是取 character_appearances 的并集，会因导入顺序不同而得到不同结果）
-- ---------------------------------------------------------------------------

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 205003, 2097152, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 205003);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 205004, 2097152, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 205004);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 205009, 2097152, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 205009);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 205005, 2097152, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 205005);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 205015, 2097152, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 205015);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 205017, 2097152, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 205017);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 205054, 2097152, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 205054);

-- ---------------------------------------------------------------------------
--  商店参数（子类 307 SpecialWeapon；display_order 沿用客户端顺序）
-- ---------------------------------------------------------------------------

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency, duration_type)
SELECT 205003, 307, 490, 10000, '2009-01-01 00:00:00', '2060-01-01 00:00:00', 1, 'cash_cn', 'count'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 205003);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency, duration_type)
SELECT 205004, 307, 491, 10000, '2009-01-01 00:00:00', '2060-01-01 00:00:00', 1, 'lucci', 'count'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 205004);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency, duration_type)
SELECT 205009, 307, 493, 10000, '2011-11-02 00:00:00', '2060-01-01 00:00:00', 1, 'cash_cn', 'count'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 205009);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency, duration_type)
SELECT 205005, 307, 604, 10000, '2009-01-01 00:00:00', '2060-01-01 00:00:00', 1, 'cash_cn', 'count'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 205005);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency, duration_type)
SELECT 205015, 307, 495, 10000, '2010-01-01 00:00:00', '2060-01-01 00:00:00', 1, 'cash_cn', 'count'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 205015);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency, duration_type)
SELECT 205017, 307, 500, 10000, '2011-04-24 00:00:00', '2060-01-01 00:00:00', 1, 'cash_cn', 'count'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 205017);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency, duration_type)
SELECT 205054, 307, 42, 20000, '2009-01-01 00:00:00', '2060-01-01 00:00:00', 1, 'cash_cn', 'count'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 205054);

-- ---------------------------------------------------------------------------
--  库存档（duration_days = 个数；sell_price 与 show_price 同价，无折扣）
--    库存 id 用 2931001 起的合成段，避开客户端真实的 stock id
-- ---------------------------------------------------------------------------

-- 205003 液体手雷E02
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931001, 205003, 50, 100, 100, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931001);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931002, 205003, 120, 200, 200, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931002);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931003, 205003, 300, 400, 400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931003);

-- 205004 粘稠手雷 E01-S
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931004, 205004, 50, 1000, 1000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931004);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931005, 205004, 120, 2400, 2400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931005);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931006, 205004, 300, 6000, 6000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931006);

-- 205009 液态地雷SF01
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931007, 205009, 50, 200, 200, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931007);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931008, 205009, 120, 400, 400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931008);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931009, 205009, 300, 800, 800, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931009);

-- 205005 液态地雷M01
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931010, 205005, 50, 200, 200, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931010);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931011, 205005, 120, 400, 400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931011);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931012, 205005, 300, 800, 800, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931012);

-- 205015 泡泡棒球队手雷
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931013, 205015, 50, 300, 300, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931013);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931014, 205015, 120, 600, 600, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931014);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931015, 205015, 300, 1200, 1200, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931015);

-- 205017 液体雷达手雷R01
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931016, 205017, 50, 400, 400, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931016);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931017, 205017, 120, 800, 800, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931017);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931018, 205017, 300, 1600, 1600, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931018);

-- 205054 超级龙鳞手雷
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931019, 205054, 50, 500, 500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931019);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2931020, 205054, 100, 900, 900, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2931020);

-- 数量类标记（重复执行无副作用）
UPDATE shop_items SET duration_type = 'count'
 WHERE item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054);

COMMIT;

-- ---------------------------------------------------------------------------
-- 自检：应该 7 行，每行 tiers 与下面的档位数一致
-- ---------------------------------------------------------------------------
SELECT i.item_id, d.name, i.currency, i.duration_type, i.max_duration_days,
       COUNT(s.id) AS tiers,
       GROUP_CONCAT(CONCAT(s.duration_days, '个/', s.sell_price) ORDER BY s.duration_days) AS prices
FROM shop_items i
JOIN item_definitions d ON d.id = i.item_id
LEFT JOIN shop_stocks s ON s.item_id = i.item_id AND s.is_on_sale = 1
WHERE i.item_id IN (205003, 205004, 205009, 205005, 205015, 205017, 205054)
GROUP BY i.item_id, d.name, i.currency, i.duration_type, i.max_duration_days
ORDER BY i.display_order;
