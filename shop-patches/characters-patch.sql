-- ============================================================================
--  泡泡战士 CN0261 · 商店热更新补丁 · 新增两名可售角色（乖小爷 / 葱头）
--
--  背景：客户端 ShopItem2 的 Characters 子类共 20 件，我们只上架了 11 种角色里的 9 件
--        （皮蛋/黑妞免费发放）。本补丁补上此前缺失、且有原生价格的两名：
--          300019 乖小爷（character_type 19，库存三档在售：1天/800、7天/1900、30天/3900）
--          300007 葱头  （character_type 7，客户端库存档全部下架，按原生价自定：
--                        1天/500、7天/1200、30天/2400）
--
--  为什么不止插商品：
--    1. character_appearances 决定「哪张卡 = 哪个角色身体」，缺行则 63 槽拿不到模型
--    2. item_equipment_rules.character_mask 决定「这件东西能被哪些角色穿」。
--       存量 215 行的掩码只有 11967 / 703 / 1727 三种，全都不含 type 7(1<<6=64)
--       与 type 19(1<<18=262144) 的位。不加位的话，换成新角色后每次读背包
--       （NormalizeAppearance 在 Items() 里跑）都会把非 63 槽装备全部脱下。
--
--       **掩码按客户端 allowChr 校正**：客户端数据表明这两名角色能穿的东西比预想的多 ——
--       122 件通用装备（全部武器/饰品/手雷）+ 62 件衣服（企鹅、小熊、夏威夷、中秋喜兔、
--       万圣节、爱斯基摩、超级明星、红玫瑰、BF学校霸王、泡泡自由…）+ BF冒险王五件套，
--       两个角色都能穿；另有 26 件（朋克熊、金星训练、风帽训练、闪闪五星、红玫瑰部分…）
--       只允许葱头穿，乖小爷穿不上（客户端 allowChr 里没有 LXUni）。
--
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
--  幂等：INSERT 带 NOT EXISTS；UPDATE 带「位还没加」的判断，可重复执行
--  回退：mariadb --default-character-set=utf8mb4 <库名> < characters-rollback.sql
--
--  ⚠ 前提（我无法在服务端确认）：客户端里要有 type 7 / 19 的模型资源，
--     否则换装后可能无角色/黑屏。上架前先在客户端确认能看到这两个角色。
-- ============================================================================
SET NAMES utf8mb4;

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
--     · 掩码分布应为 274175×124（122 通用 + 2 新卡）、262911×62、767×26、263935×5
--       （与客户端 allowChr 换算出来的结果一致）
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
  (SELECT COUNT(*) FROM character_appearances)                                           AS appearances;
