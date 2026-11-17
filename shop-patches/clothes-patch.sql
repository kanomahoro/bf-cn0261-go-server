-- ============================================================================
--  泡泡战士 CN0261 · 商店热更新补丁
--  内容：HC 五件套 8 套 × 5 件 = 40 件（帽 / 上衣 / 裤 / 手套 / 鞋）
--
--  为什么只上这 8 套：能量水晶（PowerPack）的激活条件是「穿满 5 件 HC 装扮」——
--    客户端 ShopItemSpec: PPAequipCheckValue = 5，注释「装备值达到 6 时触发」
--    （5 件 HC + 水晶自身 1 件 = 6）；desc1 也写明「HC 服装 5 种佩戴后…」
--  这 8 套在客户端 desc2 均标注「能量水晶专用HC装扮」，是真正能凑满五件的套装。
--  （另 9 套里「炫丽圣诞」名字带 HC 但客户端 desc2 为空，不算真 HC，已剔除）
--
--  生成：2026-11-15（由 clothes-53 + gloves-shoes-30 合并而来，只保留完整 HC 套）
--  数据来源：客户端 ShopItem2.xml（ShopItem / Stock / PartLocation / SlotLocation）
--
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
--        （可在服务运行中执行；文件内不带 USE，库名必须写在命令行上）
--  幂等：每条 INSERT 都带 NOT EXISTS 守卫，重复执行无副作用
--  生效：不用重启服务端；客户端重新进商店即可看到
--
--  价格（统一同类档位）
--    帽     1天 240 / 7天 720 / 30天 1250
--    上衣   1天 210 / 7天 620 / 30天 1050
--    裤     1天 110 / 7天 320 / 30天 550
--    手套   1天 110 / 7天 250 / 30天 550
--    鞋     1天 110 / 7天 250 / 30天 550
--  货币跟随客户端：cashCN → cash_cn（7 套）；lucci → lucci（爱斯基摩）
--
--  槽位：服装 = 32 + PartLocation（Head→33、BodyU→39、BodyD→40、HandR→38、Feet→42）
--        occupied_slots = 1 << 槽位
--  character_mask：按客户端 ShopItem.allowChr / denyChr 逐件换算（角色位 = 1 << (type-1)）
--        274175 = 全 13 位（僵尸王 / 圣战骑士）              原 11967
--        262911 = 10 位，含 葱头(7) / 乖小爷(19)（法老王 / 爱斯基摩）  原 703
--        272127 = 12 位，除拉比(12)（恶魔战士）              原 9919
--        271485 = 男性 9 位（犀利 / 流光绝影）              原 9277
--         2690 = 女性 4 位（流光艳影：客户端限 黑妞/蓝蓝/蓝妹妹/拉比，保留原值）
--        说明：客户端 allowChr 里 葱头(7) / 乖小爷(19) 的位已按上表加进前四种掩码；
--              只有「流光艳影」5 件因客户端 denyChr 不含这两名角色，保持 2690 不变
--  销售窗口：客户端 durationFrom → 2060-01-01
--    （踩过的坑：照抄客户端 durationTo 会有商品早就过期，商店里直接看不到）
-- ============================================================================
SET NAMES utf8mb4;

START TRANSACTION;

-- ════════════════════════════════════════════════════════════════════════
--  僵尸王（僵尸斯坦）（5 件齐全）
-- ════════════════════════════════════════════════════════════════════════

-- 100566 僵尸斯坦头盔HC  [帽] slot=33  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100566, '僵尸斯坦头盔HC', 33, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100566);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100566, 8589934592, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100566);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100566, 102, 300124, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100566);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013325, 100566, 1, 240, 240, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013325);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014011, 100566, 7, 720, 720, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014011);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015407, 100566, 30, 1250, 1250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015407);

-- 100567 僵尸王上衣HC  [上衣] slot=39  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100567, '僵尸王上衣HC', 39, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100567);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100567, 549755813888, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100567);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100567, 108, 300125, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100567);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920004, 100567, 1, 210, 210, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920004);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013275, 100567, 7, 620, 620, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013275);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015408, 100567, 30, 1050, 1050, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015408);

-- 100568 僵尸王下衣HC  [裤] slot=40  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100568, '僵尸王下衣HC', 40, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100568);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100568, 1099511627776, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100568);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100568, 109, 300126, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100568);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013323, 100568, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013323);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013459, 100568, 7, 320, 320, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013459);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015409, 100568, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015409);

-- 100569 僵尸王手套HC  [手套] slot=38  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100569, '僵尸王手套HC', 38, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100569);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100569, 274877906944, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100569);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100569, 105, 300127, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100569);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013324, 100569, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013324);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013460, 100569, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013460);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015410, 100569, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015410);

-- 100570 僵尸王鞋子HC  [鞋] slot=42  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100570, '僵尸王鞋子HC', 42, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100570);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100570, 4398046511104, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100570);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100570, 106, 300128, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100570);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920102, 100570, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920102);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013276, 100570, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013276);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015411, 100570, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015411);

-- ════════════════════════════════════════════════════════════════════════
--  法老王（5 件齐全）
-- ════════════════════════════════════════════════════════════════════════

-- 100571 法老王 发型 HC  [帽] slot=33  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100571, '法老王 发型 HC', 33, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100571);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100571, 8589934592, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100571);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100571, 102, 300119, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100571);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015490, 100571, 1, 240, 240, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015490);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013391, 100571, 7, 720, 720, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013391);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013396, 100571, 30, 1250, 1250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013396);

-- 100572 法老王 上衣 HC  [上衣] slot=39  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100572, '法老王 上衣 HC', 39, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100572);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100572, 549755813888, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100572);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100572, 108, 300120, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100572);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015491, 100572, 1, 210, 210, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015491);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013392, 100572, 7, 620, 620, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013392);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013397, 100572, 30, 1050, 1050, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013397);

-- 100573 法老王下装 HC  [裤] slot=40  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100573, '法老王下装 HC', 40, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100573);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100573, 1099511627776, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100573);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100573, 109, 300121, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100573);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015492, 100573, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015492);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013393, 100573, 7, 320, 320, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013393);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013398, 100573, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013398);

-- 100574 法老王 手套 HC  [手套] slot=38  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100574, '法老王 手套 HC', 38, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100574);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100574, 274877906944, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100574);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100574, 105, 300122, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100574);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015493, 100574, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015493);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013394, 100574, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013394);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013399, 100574, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013399);

-- 100575 法老王 鞋 HC  [鞋] slot=42  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100575, '法老王 鞋 HC', 42, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100575);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100575, 4398046511104, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100575);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100575, 106, 300123, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100575);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015494, 100575, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015494);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013395, 100575, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013395);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1013400, 100575, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1013400);

-- ════════════════════════════════════════════════════════════════════════
--  恶魔战士（5 件齐全）
-- ════════════════════════════════════════════════════════════════════════

-- 100584 恶魔战士 头盔Hc  [帽] slot=33  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100584, '恶魔战士 头盔Hc', 33, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100584);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100584, 8589934592, 272127
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100584);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100584, 102, 300265, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100584);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920005, 100584, 1, 240, 240, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920005);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920006, 100584, 7, 720, 720, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920006);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014660, 100584, 30, 1250, 1250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014660);

-- 100585 恶魔战士 上衣Hc  [上衣] slot=39  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100585, '恶魔战士 上衣Hc', 39, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100585);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100585, 549755813888, 272127
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100585);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100585, 108, 300264, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100585);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920007, 100585, 1, 210, 210, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920007);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920008, 100585, 7, 620, 620, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920008);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014661, 100585, 30, 1050, 1050, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014661);

-- 100586 恶魔战士 下衣Hc  [裤] slot=40  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100586, '恶魔战士 下衣Hc', 40, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100586);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100586, 1099511627776, 272127
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100586);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100586, 109, 300263, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100586);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920009, 100586, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920009);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920010, 100586, 7, 320, 320, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920010);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014662, 100586, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014662);

-- 100587 恶魔战士 手套Hc  [手套] slot=38  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100587, '恶魔战士 手套Hc', 38, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100587);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100587, 274877906944, 272127
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100587);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100587, 105, 300262, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100587);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920103, 100587, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920103);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920104, 100587, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920104);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014663, 100587, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014663);

-- 100588 恶魔战士 鞋子Hc  [鞋] slot=42  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100588, '恶魔战士 鞋子Hc', 42, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100588);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100588, 4398046511104, 272127
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100588);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100588, 106, 300261, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100588);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920105, 100588, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920105);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920106, 100588, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920106);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014664, 100588, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014664);

-- ════════════════════════════════════════════════════════════════════════
--  犀利（5 件齐全）
-- ════════════════════════════════════════════════════════════════════════

-- 100658 犀利发型HC  [帽] slot=33  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100658, '犀利发型HC', 33, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100658);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100658, 8589934592, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100658);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100658, 102, 752, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100658);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920012, 100658, 1, 240, 240, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920012);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920013, 100658, 7, 720, 720, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920013);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015289, 100658, 30, 1250, 1250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015289);

-- 100659 犀利上衣HC  [上衣] slot=39  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100659, '犀利上衣HC', 39, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100659);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100659, 549755813888, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100659);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100659, 108, 751, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100659);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920014, 100659, 1, 210, 210, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920014);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920015, 100659, 7, 620, 620, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920015);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015290, 100659, 30, 1050, 1050, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015290);

-- 100660 犀利下装HC  [裤] slot=40  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100660, '犀利下装HC', 40, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100660);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100660, 1099511627776, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100660);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100660, 109, 750, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100660);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920016, 100660, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920016);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920017, 100660, 7, 320, 320, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920017);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015291, 100660, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015291);

-- 100661 犀利手套HC  [手套] slot=38  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100661, '犀利手套HC', 38, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100661);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100661, 274877906944, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100661);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100661, 105, 749, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100661);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920107, 100661, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920107);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920108, 100661, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920108);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015292, 100661, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015292);

-- 100662 犀利鞋HC  [鞋] slot=42  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100662, '犀利鞋HC', 42, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100662);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100662, 4398046511104, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100662);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100662, 106, 748, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100662);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920109, 100662, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920109);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920110, 100662, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920110);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015293, 100662, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015293);

-- ════════════════════════════════════════════════════════════════════════
--  流光绝影（5 件齐全）
-- ════════════════════════════════════════════════════════════════════════

-- 100790 流光绝影发型HC  [帽] slot=33  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100790, '流光绝影发型HC', 33, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100790);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100790, 8589934592, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100790);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100790, 102, 931, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100790);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016788, 100790, 1, 240, 240, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016788);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920018, 100790, 7, 720, 720, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920018);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015252, 100790, 30, 1250, 1250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015252);

-- 100791 流光绝影上衣HC  [上衣] slot=39  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100791, '流光绝影上衣HC', 39, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100791);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100791, 549755813888, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100791);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100791, 108, 930, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100791);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016789, 100791, 1, 210, 210, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016789);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920019, 100791, 7, 620, 620, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920019);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015253, 100791, 30, 1050, 1050, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015253);

-- 100792 流光绝影下装HC  [裤] slot=40  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100792, '流光绝影下装HC', 40, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100792);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100792, 1099511627776, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100792);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100792, 109, 929, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100792);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016790, 100792, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016790);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920020, 100792, 7, 320, 320, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920020);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015254, 100792, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015254);

-- 100793 流光绝影手套HC  [手套] slot=38  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100793, '流光绝影手套HC', 38, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100793);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100793, 274877906944, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100793);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100793, 105, 928, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100793);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016791, 100793, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016791);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920111, 100793, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920111);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015255, 100793, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015255);

-- 100794 流光绝影鞋HC  [鞋] slot=42  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100794, '流光绝影鞋HC', 42, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100794);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100794, 4398046511104, 271485
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100794);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100794, 106, 927, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100794);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016792, 100794, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016792);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920112, 100794, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920112);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015256, 100794, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015256);

-- ════════════════════════════════════════════════════════════════════════
--  流光艳影（5 件齐全）
-- ════════════════════════════════════════════════════════════════════════

-- 100795 流光艳影发型HC  [帽] slot=33  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100795, '流光艳影发型HC', 33, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100795);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100795, 8589934592, 2690
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100795);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100795, 102, 936, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100795);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016793, 100795, 1, 240, 240, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016793);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920021, 100795, 7, 720, 720, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920021);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015257, 100795, 30, 1250, 1250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015257);

-- 100796 流光艳影上衣HC  [上衣] slot=39  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100796, '流光艳影上衣HC', 39, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100796);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100796, 549755813888, 2690
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100796);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100796, 108, 935, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100796);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016794, 100796, 1, 210, 210, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016794);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920022, 100796, 7, 620, 620, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920022);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015258, 100796, 30, 1050, 1050, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015258);

-- 100797 流光艳影下装HC  [裤] slot=40  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100797, '流光艳影下装HC', 40, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100797);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100797, 1099511627776, 2690
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100797);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100797, 109, 934, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100797);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016795, 100797, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016795);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920023, 100797, 7, 320, 320, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920023);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015259, 100797, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015259);

-- 100798 流光艳影手套HC  [手套] slot=38  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100798, '流光艳影手套HC', 38, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100798);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100798, 274877906944, 2690
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100798);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100798, 105, 933, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100798);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016796, 100798, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016796);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920113, 100798, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920113);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015260, 100798, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015260);

-- 100799 流光艳影鞋HC  [鞋] slot=42  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100799, '流光艳影鞋HC', 42, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100799);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100799, 4398046511104, 2690
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100799);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100799, 106, 932, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100799);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1016797, 100799, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1016797);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920114, 100799, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920114);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1015261, 100799, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1015261);

-- ════════════════════════════════════════════════════════════════════════
--  圣战骑士（5 件齐全）
-- ════════════════════════════════════════════════════════════════════════

-- 100800 圣战骑士头盔HC  [帽] slot=33  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100800, '圣战骑士头盔HC', 33, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100800);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100800, 8589934592, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100800);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100800, 102, 942, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100800);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014747, 100800, 1, 240, 240, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014747);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014757, 100800, 7, 720, 720, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014757);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014767, 100800, 30, 1250, 1250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014767);

-- 100801 圣战骑士上衣HC  [上衣] slot=39  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100801, '圣战骑士上衣HC', 39, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100801);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100801, 549755813888, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100801);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100801, 108, 941, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100801);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014748, 100801, 1, 210, 210, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014748);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014758, 100801, 7, 620, 620, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014758);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014768, 100801, 30, 1050, 1050, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014768);

-- 100802 圣战骑士下装HC  [裤] slot=40  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100802, '圣战骑士下装HC', 40, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100802);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100802, 1099511627776, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100802);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100802, 109, 940, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100802);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014749, 100802, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014749);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014759, 100802, 7, 320, 320, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014759);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014769, 100802, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014769);

-- 100803 圣战骑士手套HC  [手套] slot=38  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100803, '圣战骑士手套HC', 38, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100803);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100803, 274877906944, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100803);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100803, 105, 939, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100803);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014750, 100803, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014750);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014760, 100803, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014760);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014770, 100803, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014770);

-- 100804 圣战骑士靴子HC  [鞋] slot=42  cash_cn
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 100804, '圣战骑士靴子HC', 42, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 100804);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 100804, 4398046511104, 274175
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 100804);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 100804, 106, 938, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'cash_cn'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 100804);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014751, 100804, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014751);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014761, 100804, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014761);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 1014771, 100804, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 1014771);

-- ════════════════════════════════════════════════════════════════════════
--  爱斯基摩（5 件齐全）
-- ════════════════════════════════════════════════════════════════════════

-- 101265 爱斯基摩头发HC  [帽] slot=33  lucci
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 101265, '爱斯基摩头发HC', 33, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 101265);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 101265, 8589934592, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 101265);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 101265, 102, 142, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 101265);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920024, 101265, 1, 240, 240, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920024);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920025, 101265, 7, 720, 720, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920025);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920026, 101265, 30, 1250, 1250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920026);

-- 101266 爱斯基摩上衣HC  [上衣] slot=39  lucci
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 101266, '爱斯基摩上衣HC', 39, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 101266);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 101266, 549755813888, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 101266);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 101266, 108, 143, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 101266);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920027, 101266, 1, 210, 210, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920027);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920028, 101266, 7, 620, 620, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920028);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920029, 101266, 30, 1050, 1050, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920029);

-- 101267 爱斯基摩下装HC  [裤] slot=40  lucci
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 101267, '爱斯基摩下装HC', 40, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 101267);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 101267, 1099511627776, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 101267);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 101267, 109, 144, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 101267);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920030, 101267, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920030);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920031, 101267, 7, 320, 320, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920031);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920032, 101267, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920032);

-- 101268 爱斯基摩手套HC  [手套] slot=38  lucci
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 101268, '爱斯基摩手套HC', 38, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 101268);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 101268, 274877906944, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 101268);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 101268, 105, 145, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 101268);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920115, 101268, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920115);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920116, 101268, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920116);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920117, 101268, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920117);

-- 101269 爱斯基摩靴子HC  [鞋] slot=42  lucci
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 101269, '爱斯基摩靴子HC', 42, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 101269);

INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 101269, 4398046511104, 262911
WHERE NOT EXISTS (SELECT 1 FROM item_equipment_rules WHERE item_id = 101269);

INSERT INTO shop_items (item_id, sub_category_id, display_order, max_duration_days,
                        sale_starts_at, sale_ends_at, is_on_sale, currency)
SELECT 101269, 106, 146, 360, '2009-01-01 00:00:00.000', '2060-01-01 00:00:00.000', 1, 'lucci'
WHERE NOT EXISTS (SELECT 1 FROM shop_items WHERE item_id = 101269);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920118, 101269, 1, 110, 110, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920118);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920119, 101269, 7, 250, 250, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920119);
INSERT INTO shop_stocks (id, item_id, duration_days, show_price, sell_price, is_on_sale)
SELECT 2920120, 101269, 30, 550, 550, 1
WHERE NOT EXISTS (SELECT 1 FROM shop_stocks WHERE id = 2920120);

COMMIT;

-- ---------------------------------------------------------------------------
-- 自检：应该 40 行，每行 tiers = 3，五件齐的套装共 8 组
-- ---------------------------------------------------------------------------
SELECT i.item_id, d.name, d.equipment_slot AS slot, i.sub_category_id, i.currency,
       COUNT(s.id) AS tiers,
       GROUP_CONCAT(CONCAT(s.duration_days, '天/', s.sell_price) ORDER BY s.duration_days) AS prices
FROM shop_items i
JOIN item_definitions d ON d.id = i.item_id
LEFT JOIN shop_stocks s ON s.item_id = i.item_id
WHERE i.item_id IN (100566, 100567, 100568, 100569, 100570, 100571, 100572, 100573, 100574, 100575, 100584, 100585, 100586, 100587, 100588, 100658, 100659, 100660, 100661, 100662, 100790, 100791, 100792, 100793, 100794, 100795, 100796, 100797, 100798, 100799, 100800, 100801, 100802, 100803, 100804, 101265, 101266, 101267, 101268, 101269)
GROUP BY i.item_id, d.name, d.equipment_slot, i.sub_category_id, i.currency
ORDER BY d.equipment_slot, i.display_order;
