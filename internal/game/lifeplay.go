package game

import (
	"fmt"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

// bindLife prepares the life tracker for a fresh round.
func (s *Server) bindLife(room *Room) error {
	room.mu.Lock()
	defer room.mu.Unlock()
	life := NewBattleLife(nil, room.Setting.Game.Mode == singlePvpModeID)
	// Only the sniper battle reports a bubble popped by a weapon as a special
	// kick; the report scores a pang there and is refused everywhere else.
	life.specialKickScores = room.Setting.Game.Mode == sniperModeID
	// The pirate battle's bubbles sit on its mobs, and its mobs knock the seats: both
	// reports name objects this tracker never binds, so that round needs the NPC
	// branches (see BattleLife.npcRounds). The zombie raid needs them for the same
	// reason: its zombies, guardians, boss and mission NPCs are all mobs, and both a
	// mob victim and a mob attacker would otherwise look like a protocol error and
	// cost the connection.
	life.npcRounds = room.Setting.Game.Mode == pirateModeID || room.Setting.Game.Mode == pveBossModeID
	for slot := range room.slots {
		entry := room.slots[slot]
		if entry.session == nil {
			continue
		}
		if err := life.Bind(uint8(slot), objectFor(slot), entry.team); err != nil {
			return err
		}
	}
	room.life = life
	room.published = [maxRoomSlots]protocol.BattleScore{}
	room.teamScores = [2]uint16{}
	return nil
}

// objectFor returns the round object id of a slot.
func objectFor(slot int) uint32 { return uint32(1000000 + slot*100000) }

// broadcastScores publishes player and team score changes and reports whether a
// team reached the configured score limit.
func (s *Server) broadcastScores(room *Room) bool {
	room.mu.Lock()
	life := room.life
	if life == nil {
		room.mu.Unlock()
		return false
	}
	var teamScores [2]uint32
	limit := room.Setting.Game.Values128[0]
	if limit == 0 {
		limit = 20
	}
	teamless := room.Setting.Game.Mode == singlePvpModeID
	limitReached := false
	type update struct {
		packet protocol.Packet
	}
	var updates []update
	for slot := range room.slots {
		entry := room.slots[slot]
		if entry.session == nil {
			continue
		}
		score, err := life.Score(uint8(slot))
		if err != nil {
			continue
		}
		teamIndex := -1
		if entry.team == 1 {
			teamIndex = 0
		} else if entry.team == 2 {
			teamIndex = 1
		}
		// A team outside the two battle teams (the zombie battle's team 14) keeps
		// its personal score but adds nothing to a team total. A teamless round has
		// no team total at all: the retail individual mode shows each seat's own
		// count, so nothing is accumulated or published either.
		if teamIndex >= 0 && !teamless {
			teamScores[teamIndex] += uint32(score.PangCount)
		}
		// A teamless round has no team total to race: each seat reaches the limit
		// on its own pang count, which is what ends the individual round.
		if teamless && uint32(score.PangCount) >= limit {
			limitReached = true
		}
		if score != room.published[slot] {
			personal := uint32(score.BubbleCount)*s.prog.ResultAwards.ScoreWeights.Bubble +
				uint32(score.PangCount)*s.prog.ResultAwards.ScoreWeights.Pang +
				uint32(score.RescueCount)*s.prog.ResultAwards.ScoreWeights.Rescue
			if reply, err := protocol.CreatePlayerScoreReply(uint8(slot), score, personal); err == nil {
				updates = append(updates, update{packet: reply})
			}
			room.published[slot] = score
		}
	}
	for team := 0; team < 2; team++ {
		clamped := teamScores[team]
		if clamped > 200 {
			clamped = 200
		}
		if uint16(clamped) != room.teamScores[team] {
			room.teamScores[team] = uint16(clamped)
			if reply, err := protocol.CreateTeamScoreReply(uint8(team+1), uint16(clamped)); err == nil {
				updates = append(updates, update{packet: reply})
			}
		}
		if !teamless && clamped >= limit {
			limitReached = true
		}
	}
	room.mu.Unlock()

	for _, entry := range updates {
		room.Broadcast(entry.packet)
	}
	return limitReached
}

func (s *Server) handlePeerLife(session *Session, packet protocol.Packet, fromRelay bool) error {
	room := session.room
	if room == nil {
		return nil
	}
	life := room.Life()
	if life == nil {
		return nil
	}
	event, err := protocol.ReadPeerLife(packet)
	if err != nil {
		return err
	}
	accepted, err := life.Apply(uint8(session.slot), event, false)
	if err != nil {
		return err
	}
	// Kept at debug level: a hit the round refuses (a bubble popped by a weapon, a
	// stale report) is otherwise invisible, and "the kill did not count" has to be
	// told apart from "the report never arrived".
	s.log.Debug("peer life", "slot", session.slot, "id", packet.ID, "kind", event.Kind,
		"accepted", accepted, "victim", event.VictimObject,
		"victimState", life.StateOf(uint8((event.VictimObject-1000000)/100000)),
		"actor", event.ActorObject, "direct", event.DirectDeath, "room", room.ID)
	if !accepted {
		// Debug only: a dropped hit is invisible otherwise, and "the kill did not
		// count" has to be told from "the kill was never accepted".
		victimSlot := uint8((event.VictimObject - 1000000) / 100000)
		actorSlot := uint8((event.ActorObject - 1000000) / 100000)
		s.log.Debug("[Life] event dropped", "room", room.ID, "from", session.slot,
			"kind", event.Kind, "victim", event.VictimObject, "actor", event.ActorObject,
			"victimState", life.StateOf(victimSlot), "actorState", life.StateOf(actorSlot),
			"direct", event.DirectDeath, "tick", event.Tick)
	}
	// Relay traffic is already forwarded to the other peers by the relay itself;
	// echoing it again on the game connection makes the client show every combat
	// message twice.
	if !fromRelay {
		for _, member := range room.View() {
			if member.Occupied && member.Slot != session.slot {
				_ = member.Session.Send(packet)
			}
		}
	}
	if accepted && (event.Kind == protocol.LifeDying || event.Kind == protocol.LifeDead) {
		// Diagnostics only: the original asks the victim for its damage trace and
		// never blocks settlement on the answer.
		_ = session.Send(protocol.CreateHealthTraceRequest())
	}
	if accepted {
		// The zombie battle's primary death report is the team-change request, but a
		// client that reports the death in the combat stream instead turns here: the
		// conversion is idempotent, so a second report decides nothing new.
		if event.Kind == protocol.LifeDead && room.ZombieRound() {
			// The event names the victim, which need not be the reporter: the
			// killer's client reports the death it caused.
			s.zombieDeathOfObject(room, event.VictimObject)
		}
		// A Captain that just went down hands the Captaincy on; the check ignores
		// every event that left both appointments standing.
		s.captainsAfterChange(room)
		// The zombie raid's deaths turn the seat that fell; the report names the victim,
		// which need not be the reporter.
		if event.Kind == protocol.LifeDead && room.PveBossRound() {
			s.pveBossDeathOfObject(room, event.VictimObject)
		}
		// The bomb mission is decided by the objective, never by a kill total: its
		// score limit must not end a sub-round, the series decides the match.
		if s.broadcastScores(room) && !room.BombRound() && !room.PirateRound() && !room.PveBossRound() {
			s.finishRound(room, false)
			return nil
		}
		s.tryRespawn(room, session.slot)
	}
	return nil
}

func (s *Server) handlePeerKick(session *Session, packet protocol.Packet, fromRelay bool) error {
	room := session.room
	if room == nil {
		return nil
	}
	life := room.Life()
	if life == nil {
		return nil
	}
	event, err := protocol.ReadPeerKick(packet)
	if err != nil {
		return err
	}
	accepted, err := life.ApplyKick(uint8(session.slot), event)
	if err != nil {
		return err
	}
	// Relay traffic is already forwarded to the other peers by the relay itself;
	// echoing it again on the game connection makes the client show every combat
	// message twice.
	if !fromRelay {
		for _, member := range room.View() {
			if member.Occupied && member.Slot != session.slot {
				_ = member.Session.Send(packet)
			}
		}
	}
	victimSlot := uint8((event.Victim - 1000000) / 100000)
	actorSlot := uint8((event.Actor - 1000000) / 100000)
	s.log.Debug("peer kick", "slot", session.slot, "kind", event.Kind, "accepted", accepted,
		"victim", event.Victim, "actor", event.Actor,
		"victimState", life.StateOf(victimSlot), "actorState", life.StateOf(actorSlot),
		"succeeded", event.Succeeded, "special", event.SpecialKick, "style", event.Style,
		"room", room.ID)
	if accepted {
		if s.broadcastScores(room) && !room.BombRound() && !room.PirateRound() && !room.PveBossRound() {
			s.log.Debug("score limit reached", "room", room.ID)
			s.finishRound(room, false)
		}
		// A kick can end a Captain's life too, and the report may arrive before the
		// combat stream's death frame.
		s.captainsAfterChange(room)
	}
	return nil
}

func (s *Server) handleLifePoint(session *Session, packet protocol.Packet, fromRelay bool) error {
	room := session.room
	if room == nil {
		return nil
	}
	if _, err := protocol.ReadPeerLifePoint(packet, uint8(session.slot)); err != nil {
		return err
	}
	// The report is presentation only; the original never derives HP or score
	// from it.
	// Relay traffic is already forwarded to the other peers by the relay itself;
	// echoing it again on the game connection makes the client show every combat
	// message twice.
	if !fromRelay {
		for _, member := range room.View() {
			if member.Occupied && member.Slot != session.slot {
				_ = member.Session.Send(packet)
			}
		}
	}
	return nil
}

func (s *Server) handleReturnToNormal(session *Session, packet protocol.Packet, fromRelay bool) error {
	room := session.room
	if room == nil {
		return nil
	}
	life := room.Life()
	if life == nil {
		return nil
	}
	r := protocol.NewReader(packet.Payload)
	objectID, err := r.Unsigned()
	if err != nil {
		return err
	}
	if err := protocol.ValidateObject(objectID); err != nil {
		return err
	}
	if err := r.RequireEnd(); err != nil {
		return err
	}
	if objectID != objectFor(session.slot) {
		// The raid's client reports an NPC's recovery on the same id; the round only
		// tracks seats, so a foreign object is ignored rather than refused.
		if room.PveBossRound() {
			s.log.Debug("[PveBoss] return-to-normal for a foreign object", "room", room.ID,
				"slot", session.slot, "object", objectID)
			return nil
		}
		return fmt.Errorf("%w: return-to-normal object mismatch", protocol.ErrInvalidPacket)
	}
	life.ReturnToNormal(uint8(session.slot))
	// Relay traffic is already forwarded to the other peers by the relay itself;
	// echoing it again on the game connection makes the client show every combat
	// message twice.
	if !fromRelay {
		for _, member := range room.View() {
			if member.Occupied && member.Slot != session.slot {
				_ = member.Session.Send(packet)
			}
		}
	}
	return nil
}

// tryRespawn offers a respawn when the player's request is still valid.
// respawnDelay evaluates the respawn catalog for the body the player actually
// wears. ResolveEquippedCharacter is what the original calls before
// respawns_.Evaluate, so a player who switched bodies waits the new body's
// delay instead of the creation type's.
func (s *Server) respawnDelay(character store.Character, items []store.Item) time.Duration {
	itemIDs := make([]uint32, 0, len(items))
	for _, item := range items {
		// Only worn gear counts: a bagged necklace keeps its catalog entry, so
		// passing the whole inventory lets a player keep the reduction after
		// taking it off.
		if item.EquippedSlot == 0 || item.Quantity == 0 {
			continue
		}
		itemIDs = append(itemIDs, item.ItemID)
	}
	equipped := protocol.ResolveEquippedCharacter(character.CharacterType, s.svc.WireItems(items))
	return s.respawn.Delay(equipped, itemIDs)
}

func (s *Server) tryRespawn(room *Room, slot int) {
	life := room.Life()
	if life == nil || slot < 0 || slot >= maxRoomSlots {
		return
	}
	var target *Session
	for _, member := range room.View() {
		if member.Occupied && member.Slot == slot {
			target = member.Session
			break
		}
	}
	if target == nil {
		return
	}
	if !life.BeginRespawn(uint8(slot)) {
		return
	}
	// The spawn offer is withheld for the catalog delay so the victim's client
	// can play its respawn progress bar; offering immediately revives instantly.
	room.mu.RLock()
	character, items := room.slots[slot].character, room.slots[slot].items
	generation := room.generation
	room.mu.RUnlock()
	delay := s.respawnDelay(character, items)
	// The map catalog owns the number of markers per team; cycling stays inside
	// what the map actually defines.
	teamSpawnCount := uint32(4)
	if gameMap, ok := s.maps.Find(protocol.FromU16(room.SettingCopy().Game.Strings16[1])); ok && gameMap.TeamSpawnCount > 0 {
		teamSpawnCount = gameMap.TeamSpawnCount
	}
	offer := func() {
		room.mu.RLock()
		stale := room.generation != generation || !room.Started
		room.mu.RUnlock()
		if stale {
			return
		}
		current := room.Life()
		if current == nil {
			return
		}
		spawnIndex := room.NextSpawnIndex(slot, teamSpawnCount)
		if spawnIndex == 0 {
			// The fallback follows the seat's own marker group. A teamless seat has
			// none, so it takes the first point of the red block instead of a
			// group-zero marker that no map defines.
			spawnIndex = uint32(room.Team(slot))*10 + 1
			if spawnIndex < 11 {
				spawnIndex = 11
			}
		}
		objectID, err := current.OfferRespawn(uint8(slot), spawnIndex)
		if err != nil {
			s.log.Debug("[Respawn] offer refused", "room", room.ID, "slot", slot,
				"marker", spawnIndex, "error", err)
			return
		}
		s.log.Debug("[Respawn] offered", "room", room.ID, "slot", slot, "marker", spawnIndex,
			"team", room.Team(slot), "mode", room.SettingCopy().Game.Mode)
		_ = target.Send(protocol.CreateRespawnOffer(objectID, spawnIndex))
	}
	if delay <= 0 {
		offer()
		return
	}
	s.log.Debug("respawn delay", "room", room.ID, "slot", slot, "delay", delay)
	time.AfterFunc(delay, offer)
}
