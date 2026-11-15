-- ============================================================================
--  泡泡战士 CN0261 · 商店热更新补丁
--  内容：武器 31 件（步枪 10 / 散弹 9 / 近战 4 / 狙击 5 / 手枪 3）
--  说明：棒棒大锤 红(207039) / 绿(207040) 已剔除（客户端只有占位价 99999999、从未定价售卖），
--        它们的清理见 maintenance/2026-11-15-recall-hammer-red-green.sql
--  生成：2026-11-15
--  数据来源：客户端 ShopItem2.xml（ShopItem / Stock / SlotLocation）
--
--  用法：mariadb <库名> < 本文件        （可在服务运行中执行；文件内不带 USE，库名必须写在命令行上）
--  幂等：每条 INSERT 都带 NOT EXISTS 守卫，重复执行无副作用
--  生效：不用重启服务端；客户端重新进商店即可看到
--
--  ⚠ 销售窗口统一成 durationFrom ~ 2060-01-01。
--    客户端 ShopItem.durationTo 里有 10 件早就过期了（2011-10-12 / 2011-09-24 / 2013-04-24 等），
--    服务端 ShopCatalog() 会按 sale_ends_at > UTC_TIMESTAMP 过滤，照抄原值会导致这些商品在商店里
--    根本看不到。2026-11-15 修：一律延到 2060-01-01（与仓库现有 65 件武器一致）。
--
--  每件都已在 config/game_weapons.json 里有 definitionId 定义（见每条注释），peer 校验不会拒。
--
--  槽位：客户端 slotLocation → SlotLocation id
--    sWeapon1=18（步枪/散弹/狙击）、sWeapon2=19（手枪）、sWeapon3=20（近战）
--    occupied_slots = 1 << 槽位，character_mask = 11967
--
--  价格（按同类现有武器，只发 7 天 / 30 天两档；货币 lucci，与现有武器一致）
--    手枪 普通 3000/6000 ；手枪 HC 500/800
--    步枪·散弹·狙击 普通 4500/9000 ；HC 1380/2580
--    近战：棒棒大锤/手杖 600/1200 ；战锤/火锤 880/1980
--  自建 stock id 用 291xxxx 段（客户端未使用）
-- ============================================================================
SET NAMES utf8mb4;

START TRANSACTION;

-- 207037 龙鳞战锤  [Near]  definitionId=7037  slot=20  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 207037, '龙鳞战锤', 20, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 207037);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 207037, CAST(1 AS UNSIGNED) << 20, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 207037);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 207037, 306, 519, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 207037);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011688, 207037, 7, 880, 880, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011688);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011689, 207037, 30, 1980, 1980, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011689);

-- 207055 凡尔赛玫瑰之锤  [Near]  definitionId=7055  slot=20  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 207055, '凡尔赛玫瑰之锤', 20, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 207055);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 207055, CAST(1 AS UNSIGNED) << 20, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 207055);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 207055, 306, 2026, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 207055);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014482, 207055, 7, 880, 880, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014482);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014483, 207055, 30, 1980, 1980, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014483);

-- 207922 绮丽双彩手杖  [Near]  definitionId=7922  slot=20  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 207922, '绮丽双彩手杖', 20, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 207922);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 207922, CAST(1 AS UNSIGNED) << 20, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 207922);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 207922, 306, 115, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 207922);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910003, 207922, 7, 600, 600, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910003);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910004, 207922, 30, 1200, 1200, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910004);

-- 207926 年年有余火锤  [Near]  definitionId=7926  slot=20  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 207926, '年年有余火锤', 20, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 207926);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 207926, CAST(1 AS UNSIGNED) << 20, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 207926);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 207926, 306, 115, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 207926);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910005, 207926, 7, 880, 880, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910005);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910006, 207926, 30, 1980, 1980, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910006);

-- 201108 复仇者手枪  [Pistol]  definitionId=1108  slot=19  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 201108, '复仇者手枪', 19, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 201108);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 201108, CAST(1 AS UNSIGNED) << 19, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 201108);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 201108, 305, 1, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 201108);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910007, 201108, 7, 3000, 3000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910007);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910008, 201108, 30, 6000, 6000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910008);

-- 201110 沙漠之鹰手枪  [Pistol]  definitionId=1110  slot=19  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 201110, '沙漠之鹰手枪', 19, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 201110);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 201110, CAST(1 AS UNSIGNED) << 19, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 201110);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 201110, 305, 1, 360, '2011-10-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 201110);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011724, 201110, 7, 3000, 3000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011724);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011725, 201110, 30, 6000, 6000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011725);

-- 201342 黄金左轮手枪R3 HC  [Pistol]  definitionId=1342  slot=19  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 201342, '黄金左轮手枪R3 HC', 19, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 201342);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 201342, CAST(1 AS UNSIGNED) << 19, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 201342);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 201342, 305, 14011, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 201342);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910009, 201342, 7, 500, 500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910009);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015824, 201342, 30, 800, 800, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015824);

-- 202015 喜兔步枪HC  [Rifle]  definitionId=2015  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202015, '喜兔步枪HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202015);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202015, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202015);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202015, 302, 1, 360, '2011-09-06 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202015);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011620, 202015, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011620);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011621, 202015, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011621);

-- 202207 狸猫团步枪HC  [Rifle]  definitionId=2207  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202207, '狸猫团步枪HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202207);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202207, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202207);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202207, 302, 1, 360, '2013-03-23 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202207);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013194, 202207, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013194);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013252, 202207, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013252);

-- 202308 复仇者步枪  [Rifle]  definitionId=2308  slot=18  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202308, '复仇者步枪', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202308);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202308, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202308);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202308, 302, 1, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202308);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910010, 202308, 7, 4500, 4500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910010);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910011, 202308, 30, 9000, 9000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910011);

-- 202312 守护者步枪HC  [Rifle]  definitionId=2312  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202312, '守护者步枪HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202312);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202312, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202312);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202312, 302, 1, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202312);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013214, 202312, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013214);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013471, 202312, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013471);

-- 202315 AK47步枪  [Rifle]  definitionId=2320  slot=18  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202315, 'AK47步枪', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202315);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202315, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202315);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202315, 302, 1, 360, '2011-10-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202315);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011715, 202315, 7, 4500, 4500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011715);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011716, 202315, 30, 9000, 9000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011716);

-- 202327 恶魔步枪HC  [Rifle]  definitionId=2327  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202327, '恶魔步枪HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202327);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202327, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202327);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202327, 302, 1, 360, '2013-05-25 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202327);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013879, 202327, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013879);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910012, 202327, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910012);

-- 202333 蝰蛇之牙步枪 HC  [Rifle]  definitionId=2333  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202333, '蝰蛇之牙步枪 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202333);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202333, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202333);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202333, 302, 1, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202333);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013716, 202333, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013716);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013806, 202333, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013806);

-- 202342 黄金左轮步枪R3 HC  [Rifle]  definitionId=2342  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202342, '黄金左轮步枪R3 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202342);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202342, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202342);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202342, 302, 24008, 360, '2013-06-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202342);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015973, 202342, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015973);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015670, 202342, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015670);

-- 202376 蓝离激光步枪 HC  [Rifle]  definitionId=2376  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202376, '蓝离激光步枪 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202376);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202376, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202376);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202376, 302, 2724, 360, '2011-03-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202376);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016532, 202376, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016532);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016546, 202376, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016546);

-- 202378 旋律咩咩步枪 HC  [Rifle]  definitionId=2378  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 202378, '旋律咩咩步枪 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 202378);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 202378, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 202378);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 202378, 302, 2727, 360, '2011-03-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 202378);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910013, 202378, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910013);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910014, 202378, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910014);

-- 203014 喜兔散弹枪HC  [Shotgun]  definitionId=3014  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 203014, '喜兔散弹枪HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 203014);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 203014, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 203014);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 203014, 303, 1, 360, '2011-09-06 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 203014);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011623, 203014, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011623);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011624, 203014, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011624);

-- 203103 狸猫团散弹枪HC  [Shotgun]  definitionId=3103  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 203103, '狸猫团散弹枪HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 203103);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 203103, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 203103);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 203103, 303, 1, 360, '2013-03-23 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 203103);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013193, 203103, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013193);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013208, 203103, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013208);

-- 203206 复仇者散弹枪  [Shotgun]  definitionId=3206  slot=18  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 203206, '复仇者散弹枪', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 203206);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 203206, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 203206);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 203206, 303, 1, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 203206);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910015, 203206, 7, 4500, 4500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910015);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910016, 203206, 30, 9000, 9000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910016);

-- 203209 汽锤散弹枪  [Shotgun]  definitionId=3209  slot=18  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 203209, '汽锤散弹枪', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 203209);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 203209, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 203209);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 203209, 303, 1, 360, '2011-10-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 203209);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011718, 203209, 7, 4500, 4500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011718);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1011719, 203209, 30, 9000, 9000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1011719);

-- 203220 狸猫团散弹枪  [Shotgun]  definitionId=3220  slot=18  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 203220, '狸猫团散弹枪', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 203220);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 203220, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 203220);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 203220, 303, 1, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 203220);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013203, 203220, 7, 4500, 4500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013203);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013183, 203220, 30, 9000, 9000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013183);

-- 203333 蝰蛇之牙散弹枪 HC  [Shotgun]  definitionId=3333  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 203333, '蝰蛇之牙散弹枪 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 203333);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 203333, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 203333);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 203333, 303, 1, 360, '2011-11-08 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 203333);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013754, 203333, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013754);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013755, 203333, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013755);

-- 203342 黄金左轮散弹枪R3 HC  [Shotgun]  definitionId=3342  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 203342, '黄金左轮散弹枪R3 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 203342);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 203342, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 203342);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 203342, 303, 34020, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 203342);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910017, 203342, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910017);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015826, 203342, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015826);

-- 203372 蓝离激光散弹枪 HC  [Shotgun]  definitionId=3372  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 203372, '蓝离激光散弹枪 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 203372);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 203372, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 203372);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 203372, 303, 2724, 360, '2011-03-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 203372);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016534, 203372, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016534);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016548, 203372, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016548);

-- 203373 旋律咩咩散弹枪 HC  [Shotgun]  definitionId=3373  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 203373, '旋律咩咩散弹枪 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 203373);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 203373, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 203373);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 203373, 303, 2727, 360, '2011-03-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 203373);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910018, 203373, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910018);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910019, 203373, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910019);

-- 204107 复仇者狙击枪  [SniperRifle]  definitionId=4107  slot=18  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 204107, '复仇者狙击枪', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 204107);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 204107, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 204107);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 204107, 304, 1, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 204107);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910020, 204107, 7, 4500, 4500, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910020);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910021, 204107, 30, 9000, 9000, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910021);

-- 204342 黄金左轮狙击枪R3 HC  [SniperRifle]  definitionId=4342  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 204342, '黄金左轮狙击枪R3 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 204342);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 204342, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 204342);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 204342, 304, 44013, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 204342);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910022, 204342, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910022);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015827, 204342, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015827);

-- 204365 天空之翼狙击枪 HC  [SniperRifle]  definitionId=4365  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 204365, '天空之翼狙击枪 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 204365);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 204365, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 204365);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 204365, 304, 2722, 360, '2011-11-21 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 204365);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910023, 204365, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910023);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910024, 204365, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910024);

-- 204367 蓝离激光狙击枪 HC  [SniperRifle]  definitionId=4367  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 204367, '蓝离激光狙击枪 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 204367);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 204367, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 204367);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 204367, 304, 2724, 360, '2011-11-21 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 204367);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016617, 204367, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016617);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016619, 204367, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016619);

-- 204368 旋律咩咩狙击枪 HC  [SniperRifle]  definitionId=4368  slot=18  HC 档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 204368, '旋律咩咩狙击枪 HC', 18, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 204368);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 204368, CAST(1 AS UNSIGNED) << 18, 11967
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 204368);
INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 204368, 304, 2727, 360, '2011-11-21 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 204368);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910025, 204368, 7, 1380, 1380, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910025);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2910026, 204368, 30, 2580, 2580, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2910026);

COMMIT;

-- 自检：应该 31 行，每行 tiers = 2
SELECT i.item_id, d.name, d.equipment_slot AS slot, i.sub_category_id, i.currency,
       COUNT(s.id) AS tiers,
       GROUP_CONCAT(CONCAT(s.duration_days, '天/', s.sell_price) ORDER BY s.duration_days) AS prices
FROM shop_items i
JOIN item_definitions d ON d.id = i.item_id
LEFT JOIN shop_stocks s ON s.item_id = i.item_id
WHERE i.item_id IN (207037, 207055, 207922, 207926, 201108, 201110, 201342, 202015, 202207, 202308, 202312, 202315, 202327, 202333, 202342, 202376, 202378, 203014, 203103, 203206, 203209, 203220, 203333, 203342, 203372, 203373, 204107, 204342, 204365, 204367, 204368)
GROUP BY i.item_id, d.name, d.equipment_slot, i.sub_category_id, i.currency
ORDER BY d.equipment_slot, i.item_id;
