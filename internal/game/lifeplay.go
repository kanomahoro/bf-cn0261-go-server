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
	life := NewBattleLife(nil)
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
	room.published = [8]protocol.BattleScore{}
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
		teamIndex := 0
		if entry.team == 2 {
			teamIndex = 1
		}
		teamScores[teamIndex] += uint32(score.PangCount)
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
	limitReached := false
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
		if clamped >= limit {
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
		if s.broadcastScores(room) {
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
	s.log.Debug("peer kick", "slot", session.slot, "kind", event.Kind, "accepted", accepted,
		"victim", event.Victim, "actor", event.Actor, "limit", room.SettingCopy().Game.Values128[0])
	if accepted {
		if s.broadcastScores(room) {
			s.log.Debug("score limit reached", "room", room.ID)
			s.finishRound(room, false)
		}
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
		itemIDs = append(itemIDs, item.ItemID)
	}
	equipped := protocol.ResolveEquippedCharacter(character.CharacterType, s.svc.WireItems(items))
	return s.respawn.Delay(equipped, itemIDs)
}

func (s *Server) tryRespawn(room *Room, slot int) {
	life := room.Life()
	if life == nil || slot < 0 || slot >= 8 {
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
	spawnIndex := uint32(room.Team(slot))*10 + 1
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
		objectID, err := current.OfferRespawn(uint8(slot), spawnIndex)
		if err != nil {
			return
		}
		_ = target.Send(protocol.CreateRespawnOffer(objectID, spawnIndex))
	}
	if delay <= 0 {
		offer()
		return
	}
	s.log.Debug("respawn delay", "room", room.ID, "slot", slot, "delay", delay)
	time.AfterFunc(delay, offer)
}
