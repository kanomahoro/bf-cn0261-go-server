-- ============================================================================
-- target-revision: 2
--  升级 001：老库 → schema revision 2（商店内容收敛）
--
--  适用：用 14 份历史迁移建过、商店/掩码处于任意中间状态的库（revision 1）。
--  内容：① shop_items.duration_type 列（原先由服务端启动时 ALTER，现在只在这里）
--        ② 五个商店补丁（衣服 / 武器 / 杂项 / 投掷物 / 角色）逐条幂等导入
--        ③ 掩码兜底：旧「11 种全开放」11967 → 13 种 274175
--        ④ 清理 38 件「只展示不售卖」的预览商品（保留 item_definitions 与玩家背包）
--        ⑤ schema_version 建表并写入 revision = 2
--
--  幂等：INSERT 带 NOT EXISTS、ALTER 带 IF NOT EXISTS、清理走临时表清单，
--        重复执行无副作用。内容由 shop-patches 的 6 个文件拼接生成，不要手改。
--  用法：bash database/upgrade.sh（先备份），或服务端 --auto-upgrade。
--  生成：2026-11-17
-- ============================================================================
SET NAMES utf8mb4;

-- schema_version：老库没有这张表，先建出来
CREATE TABLE IF NOT EXISTS schema_version (
  id TINYINT UNSIGNED NOT NULL,
  revision INT UNSIGNED NOT NULL,
  note VARCHAR(128) NOT NULL,
  applied_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT ck_schema_version_single_row CHECK (id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ① 原先由服务端启动时补的列
ALTER TABLE shop_items
  ADD COLUMN IF NOT EXISTS duration_type ENUM('day','count') NOT NULL DEFAULT 'day'
  COMMENT 'day: shop_stocks.duration_days is days; count: it is a piece count'
  AFTER max_duration_days;


-- ==========================================================================
--  ② 衣服补丁：HC 五件套 8 套 / 40 件（含掩码校正 35 件） ← shop-patches/clothes-patch.sql
-- ==========================================================================
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

-- ==========================================================================
--  ② 武器补丁：31 件 ← shop-patches/weapons-patch.sql
-- ==========================================================================
SET NAMES utf8mb4;

START TRANSACTION;

-- 207037 龙鳞战锤  [Near]  definitionId=7037  slot=20  普通档
INSERT INTO item_definitions (id, name, equipment_slot, starter_equipped)
SELECT 207037, '龙鳞战锤', 20, 0
WHERE NOT EXISTS (SELECT 1 FROM item_definitions WHERE id = 207037);
INSERT INTO item_equipment_rules (item_id, occupied_slots, character_mask)
SELECT 207037, CAST(1 AS UNSIGNED) << 20, 274175
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
SELECT 207055, CAST(1 AS UNSIGNED) << 20, 274175
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
SELECT 207922, CAST(1 AS UNSIGNED) << 20, 274175
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
SELECT 207926, CAST(1 AS UNSIGNED) << 20, 274175
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
SELECT 201108, CAST(1 AS UNSIGNED) << 19, 274175
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
SELECT 201110, CAST(1 AS UNSIGNED) << 19, 274175
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
SELECT 201342, CAST(1 AS UNSIGNED) << 19, 274175
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
SELECT 202015, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 202207, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 202308, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 202312, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 202315, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 202327, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 202333, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 202342, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 202376, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 202378, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 203014, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 203103, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 203206, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 203209, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 203220, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 203333, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 203342, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 203372, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 203373, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 204107, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 204342, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 204365, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 204367, CAST(1 AS UNSIGNED) << 18, 274175
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
SELECT 204368, CAST(1 AS UNSIGNED) << 18, 274175
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

-- ==========================================================================
--  ② 杂项补丁：13 件 ← shop-patches/misc-items-patch.sql
-- ==========================================================================
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
SELECT 100412, 32, 274175
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
SELECT 100431, 32, 274175
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
SELECT 100500, 32, 274175
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
SELECT 100546, 32, 274175
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
SELECT 100224, 16777216, 274175
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
SELECT 100310, 65536, 274175
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
SELECT 101552, 134283264, 274175
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
SELECT 110140, 65536, 274175
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
SELECT 100172, 134217728, 274175
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
SELECT 100311, 134217728, 274175
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
SELECT 110172, 134217728, 274175
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
SELECT 100085, 16384, 274175
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
SELECT 100377, 65536, 274175
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

-- ==========================================================================
--  ② 投掷物补丁：7 件（按个数卖） ← shop-patches/grenades-patch.sql
-- ==========================================================================
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

-- ==========================================================================
--  ② 角色补丁：乖小爷 / 葱头 + 装备掩码 128 条 ← shop-patches/characters-patch.sql
-- ==========================================================================
SET NAMES utf8mb4;

-- 服务端启动时也会跑这条（EnsureShopUnits），这里重复一次保证本补丁可单独执行、
-- 不依赖 grenades-patch 先导。MariaDB 的 DDL 会隐式提交，所以它必须在 START TRANSACTION 之前。
ALTER TABLE shop_items
  ADD COLUMN IF NOT EXISTS duration_type ENUM('day','count') NOT NULL DEFAULT 'day'
  COMMENT 'day: shop_stocks.duration_days is days; count: it is a piece count'
  AFTER max_duration_days;

START TRANSACTION;

-- ---------------------------------------------------------------------------
--  1) 掩码：按客户端 ShopItem2 的 allowChr/denyChr 逐件校正
--     客户端每件装备都写明「允许哪些角色穿」，把 allowChr 里的名字映射成角色类型
--     （Dao=1 Dizni=2 Bazzi=3 Kephi=4 Uni=5 Mos=6 Ethi=7 Marid=8 LXMarid=10
--       Leo=11 Lavi=12 Twotop=14 LXUni=19，只保留本服务端支持的 13 种），
--     与库里原掩码比对后只有 93 件需要改，分三组：
--       · 62 件：葱头 + 乖小爷 都能穿
--       · 26 件：只有葱头能穿（allowChr 不含 LXUni）
--       · 5  件：BF冒险王五件套，两个角色都能穿
--     其余 122 件（全部武器、饰品、手雷等）原本就是「13 种身体全开放」，不用动。
--     这一步决定「换成新角色后哪些装备不会被 NormalizeAppearance 脱掉」。
-- ---------------------------------------------------------------------------
-- 703 → 262911：62 件（两个角色都能穿（allowChr 含 Ethi 与 LXUni））
UPDATE item_equipment_rules SET character_mask = 262911
 WHERE item_id IN (
  100004, 100007, 100047, 100048, 100049, 100072, 100073, 100074,
  100096, 100105, 100106, 100108, 100109, 100121, 100122, 100124,
  100125, 100126, 100128, 100133, 100134, 100147, 100148, 100149,
  100150, 100151, 100330, 100331, 100332, 100333, 100334, 100413,
  100414, 100415, 100416, 100417, 100418, 100419, 100420, 100421,
  100422, 100433, 100434, 100435, 100436, 100437, 100449, 100450,
  100451, 100452, 100453, 100510, 100511, 100512, 100513, 100514,
  100530, 100531, 100532, 100533, 100534, 120335
 );

-- 703 → 767：26 件（只有葱头(7)能穿，乖小爷(19)不行（allowChr 不含 LXUni））
UPDATE item_equipment_rules SET character_mask = 767
 WHERE item_id IN (
  110072, 110073, 110101, 110103, 110129, 110130, 119999, 120101,
  120103, 120129, 120130, 120156, 120157, 120158, 120159, 120160,
  120232, 120233, 120234, 120235, 120236, 120237, 120238, 120239,
  120240, 120241
 );

-- 1727 → 263935：5 件（两个角色都能穿（allowChr 含 Ethi、LXUni、Leo））
UPDATE item_equipment_rules SET character_mask = 263935
 WHERE item_id IN (
  100290, 100291, 100292, 100293, 100294
 );

-- ---------------------------------------------------------------------------
--  1b) 昨天那套 HC 衣服补丁（clothes-patch.sql，8 套 40 件）的掩码同步校正
--      它上线时我们只有 11 种角色，40 件里有 35 件只写了 11 种的位，
--      按同一份客户端 allowChr 校对后这 35 件都允许 葱头(7) 与 乖小爷(19)：
--      需要给它们补上这两位，否则换成新角色后这些 HC 会被 NormalizeAppearance 脱掉。
--      剩下 5 件「流光艳影HC」客户端本身就只允许 黑妞/蓝蓝/蓝妹妹/拉比，保持不动。
-- ---------------------------------------------------------------------------
-- 274175：10 件 —— 僵尸王 / 圣战骑士 HC 五件套（客户端 allowChr 含 Ethi、LXUni）
UPDATE item_equipment_rules SET character_mask = 274175
 WHERE item_id IN (
  100566, 100567, 100568, 100569, 100570, 100800, 100801, 100802,
  100803, 100804
 );

-- 262911：10 件 —— 法老王 / 爱斯基摩 HC 五件套（allowChr 含 Ethi、LXUni）
UPDATE item_equipment_rules SET character_mask = 262911
 WHERE item_id IN (
  100571, 100572, 100573, 100574, 100575, 101265, 101266, 101267,
  101268, 101269
 );

-- 272127：5 件 —— 恶魔战士 HC 五件套（allowChr 含 Ethi、LXUni）
UPDATE item_equipment_rules SET character_mask = 272127
 WHERE item_id IN (
  100584, 100585, 100586, 100587, 100588
 );

-- 271485：10 件 —— 犀利 / 流光绝影 HC 五件套（allowChr 含 Ethi、LXUni）
UPDATE item_equipment_rules SET character_mask = 271485
 WHERE item_id IN (
  100658, 100659, 100660, 100661, 100662, 100790, 100791, 100792,
  100793, 100794
 );

-- ---------------------------------------------------------------------------
--  1.1) 兜底：把「旧版 11 种全开放」（11967）的规则统一抬到 13 种（274175）
--       需要它的原因：迁移种子和早期补丁写的都是 11967，而上面的逐组 UPDATE
--       只覆盖按客户端 allowChr 换算过的 128 件，剩下的武器/饰品/投掷物/杂项
--       会留在 11967 —— 全新库上新角色一读背包就会被全部脱掉。
--       现有库（被旧版「按位或」刷过）这行匹配 0 条，属于空操作；
--       只动 11967，不碰 3 / 703 / 1727 / 2690 这些受客户端 allowChr 限制的组。
-- ---------------------------------------------------------------------------
UPDATE item_equipment_rules SET character_mask = 274175
 WHERE character_mask = 11967;

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
--     · 掩码分布（全部补丁导完的空库实测）：274175×178（166 通用 + 10 HC + 2 张新卡）、
--       262911×72、767×26、271485×10、263935×5、272127×5、2690×5；
--       旧库因为历史原因数字会不同，**只要 mask_legacy_left = 0 就对了**
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
  (SELECT COUNT(*) FROM item_equipment_rules WHERE character_mask = 274175)              AS mask_all13,
  (SELECT COUNT(*) FROM item_equipment_rules WHERE character_mask = 262911)              AS mask_no_leo_family,
  (SELECT COUNT(*) FROM item_equipment_rules WHERE character_mask = 767)                 AS mask_no_lxuni,
  (SELECT COUNT(*) FROM item_equipment_rules WHERE character_mask = 263935)              AS mask_adventurer,
  (SELECT COUNT(*) FROM item_equipment_rules WHERE character_mask = 11967)               AS mask_legacy_left,
  (SELECT COUNT(*) FROM character_appearances)                                           AS appearances;

-- ==========================================================================
--  ③ 掩码兜底：旧「11 种身体全开放」抬到 13 种（含葱头 7 / 乖小爷 19）
--     只动 11967，不碰 3 / 703 / 1727 / 2690 这些受客户端 allowChr 限制的组
-- ==========================================================================
UPDATE item_equipment_rules SET character_mask = 274175 WHERE character_mask = 11967;

-- ============================================================================
--  数量类商品标记（原先由服务端启动时 EnsureShopUnits 回填，2026-11-17 移到这里）
--  shop_items.duration_type = 'count' 的商品，shop_stocks.duration_days 是「个数」，
--  购买时按个数累加、不写过期行；漏标记会被当成「租 N 天」→ 买 50 个只到手 1 个。
--  清单取自客户端 ShopItem2.xml 的 durationType=count（90 条），只影响店里已有的行。
--  幂等：已标 count 的行不会被重复处理。
-- ============================================================================
UPDATE shop_items SET duration_type = 'count'
 WHERE duration_type <> 'count' AND item_id IN (
  100001, 100002, 100064, 100065, 100066, 100067, 100079, 100080,
  100081, 100082, 100090, 100091, 100104, 100178, 100179, 100180,
  100251, 100252, 100382, 100383, 100384, 100385, 100386, 100425,
  100443, 100473, 100482, 100483, 100484, 100485, 100507, 100508,
  100509, 100547, 100548, 100598, 100622, 100623, 100624, 100625,
  100626, 100627, 100628, 100629, 100640, 100641, 100643, 100644,
  100650, 100652, 100657, 100819, 100976, 101050, 101062, 101164,
  101291, 120520, 190051, 190069, 190071, 190074, 190075, 190083,
  190085, 190086, 205002, 205003, 205004, 205005, 205006, 205009,
  205010, 205012, 205015, 205017, 205029, 205030, 205031, 205034,
  205035, 205040, 205041, 205044, 205045, 205048, 205050, 205051,
  205052, 205054
);

-- ==========================================================================
--  ④ 清理 38 件「只展示不售卖」的预览商品 ← shop-patches/preview-cleanup.sql
--     删除顺序按外键：shop_purchases → shop_permanent_stocks → shop_stocks
--                    → shop_preview_items → shop_items；item_definitions 保留
-- ==========================================================================
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

-- ==========================================================================
--  删掉被取代的旧版本记录表：14 行历史只说明"当年跑过哪些脚本"，
--  现在由 schema_version.revision 表达。删掉它，升级后的库与「新装基线」结构一致。
-- ==========================================================================
DROP TABLE IF EXISTS schema_migrations;

-- ==========================================================================
--  ⑤ 版本落章：revision = 2
-- ==========================================================================
INSERT INTO schema_version (id, revision, note, applied_at)
VALUES (1, 2, 'upgrade 001: shop convergence', UTC_TIMESTAMP(6))
ON DUPLICATE KEY UPDATE revision = VALUES(revision), note = VALUES(note), applied_at = VALUES(applied_at);

-- 自检：mask_legacy_left / preview_left 应为 0，revision 应为 2
SELECT
  (SELECT COUNT(*) FROM item_equipment_rules WHERE character_mask = 11967) AS mask_legacy_left,
  (SELECT COUNT(*) FROM shop_preview_items)                                 AS preview_left,
  (SELECT revision FROM schema_version WHERE id = 1)                        AS revision;
