-- ============================================================================
--  泡泡战士 CN0261 · 全服补发初始角色卡（维护脚本，不是商店补丁）
--  内容：给**所有角色**补齐「皮蛋」(300001) 和「黑妞」(300002) 两张卡
--        （已有的跳过，缺哪张补哪张 —— 创建时只会送所选的那一张）
--
--  用法：mariadb --default-character-set=utf8mb4 <库名> < 本文件
--  幂等：可重复执行，已持有的不会被重复叠加（quantity 不会 +1）
--  生效：服务端实时读库，玩家重新登录就能在背包看到，不用重启
--
--  说明：
--    · 只写 character_items，**不写 character_item_expirations** —— 与创建角色
--      赠送角色卡完全一致，属于"真永久"（expires_at 为 NULL）
--    · equipped_slot 留 NULL：卡进背包，不强行替换玩家当前角色
--    · 只影响执行这一刻已存在的角色；之后新建的角色仍按创建流程走（只送所选那张）
--    · 两张卡对应的角色 id 是 1 / 2，都在 item_equipment_rules.character_mask
--      覆盖范围内（274175），不会出现"穿它导致其它装备被脱掉"
-- ============================================================================
SET NAMES utf8mb4;

SET @pdan  := 300001;   -- 皮蛋（角色 1）
SET @heiniu := 300002;  -- 黑妞（角色 2）

-- ---------------------------------------------------------------------------
-- 1) 发放前统计
-- ---------------------------------------------------------------------------
SELECT
  (SELECT COUNT(*) FROM characters)                                              AS characters_total,
  (SELECT COUNT(DISTINCT ci.character_id) FROM character_items ci
     WHERE ci.item_id = @pdan)                                                   AS owned_pidan,
  (SELECT COUNT(DISTINCT ci.character_id) FROM character_items ci
     WHERE ci.item_id = @heiniu)                                                 AS owned_heiniu,
  (SELECT COUNT(*) FROM characters c WHERE NOT EXISTS (
      SELECT 1 FROM character_items ci WHERE ci.character_id = c.id AND ci.item_id = @pdan))
                                                                                 AS will_get_pidan,
  (SELECT COUNT(*) FROM characters c WHERE NOT EXISTS (
      SELECT 1 FROM character_items ci WHERE ci.character_id = c.id AND ci.item_id = @heiniu))
                                                                                 AS will_get_heiniu;

-- ---------------------------------------------------------------------------
-- 2) 补发（两张各跑一遍，缺的才插）
-- ---------------------------------------------------------------------------
START TRANSACTION;

INSERT INTO character_items (character_id, item_id, quantity, equipped_slot, created_at, updated_at)
SELECT c.id, @pdan, 1, NULL, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
FROM characters c
WHERE NOT EXISTS (
    SELECT 1 FROM character_items ci
     WHERE ci.character_id = c.id AND ci.item_id = @pdan
);

INSERT INTO character_items (character_id, item_id, quantity, equipped_slot, created_at, updated_at)
SELECT c.id, @heiniu, 1, NULL, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
FROM characters c
WHERE NOT EXISTS (
    SELECT 1 FROM character_items ci
     WHERE ci.character_id = c.id AND ci.item_id = @heiniu
);

COMMIT;

-- ---------------------------------------------------------------------------
-- 3) 自检：owned_* 应等于 characters_total，will_get_* 应为 0
-- ---------------------------------------------------------------------------
SELECT
  (SELECT COUNT(*) FROM characters)                                              AS characters_total,
  (SELECT COUNT(DISTINCT ci.character_id) FROM character_items ci
     WHERE ci.item_id = @pdan)                                                   AS owned_pidan_now,
  (SELECT COUNT(DISTINCT ci.character_id) FROM character_items ci
     WHERE ci.item_id = @heiniu)                                                 AS owned_heiniu_now,
  (SELECT COUNT(*) FROM characters c WHERE NOT EXISTS (
      SELECT 1 FROM character_items ci WHERE ci.character_id = c.id AND ci.item_id = @pdan))
                                                                                 AS still_missing_pidan,
  (SELECT COUNT(*) FROM characters c WHERE NOT EXISTS (
      SELECT 1 FROM character_items ci WHERE ci.character_id = c.id AND ci.item_id = @heiniu))
                                                                                 AS still_missing_heiniu;

-- 仍然缺的情况（正常应为空）
SELECT c.id AS character_id, c.nickname, c.character_type,
       CASE WHEN NOT EXISTS (SELECT 1 FROM character_items ci
                              WHERE ci.character_id = c.id AND ci.item_id = @pdan)   THEN '缺皮蛋' ELSE '' END AS missing_pidan,
       CASE WHEN NOT EXISTS (SELECT 1 FROM character_items ci
                              WHERE ci.character_id = c.id AND ci.item_id = @heiniu) THEN '缺黑妞' ELSE '' END AS missing_heiniu
FROM characters c
WHERE NOT EXISTS (SELECT 1 FROM character_items ci
                   WHERE ci.character_id = c.id AND ci.item_id = @pdan)
   OR NOT EXISTS (SELECT 1 FROM character_items ci
                   WHERE ci.character_id = c.id AND ci.item_id = @heiniu)
ORDER BY c.id;
