package game

import (
	"math/rand/v2"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// The zombie battle (game mode 8).
//
// The 2015 client keeps no rules for this mode: its ZombieModeMgr only reacts to
// the native team-change notice. Every seat starts on the human team (1); the
// first seat the server moves to the zombie team (14) is shown as the zombie
// king, and every later one as a zombini. A dead human's client reports the
// death as that same team-change request, a dead zombini sends it again to come
// back without limit, and the king never returns: its fall (or its departure)
// wins the round for the humans. The zombies win when no human is left; the
// humans win when the round's 180 s run out. Reference: BubbleFighter.Fantasy
// RoomFlow.ZombiePvp / RoundFlow.ZombieKing (kr-812 ZombieModeMgr, sub_776870,
// sub_76C090).
const (
	// zombieModeID is the client's wgtZombie game mode.
	zombieModeID = 8
	// zombieHumanTeam is the humans' team; kr-812 sub_7694F0 counts team 1's
	// living players as the last humans.
	zombieHumanTeam uint8 = 1
	// zombieTeam is the zombies' team, king and zombinis alike; kr-812 sub_776870
	// acts on team 14 only.
	zombieTeam uint8 = 14
	// zombieRoundLimitMs is the mode's fixed round length: the client writes
	// 180000 into the rule record's time-attack slot at create time.
	zombieRoundLimitMs = 180000
)

// zombieOpening is the time between the start of play and the king's pick. It is
// a variable so unit tests can shorten it.
var zombieOpening = 5 * time.Second

// maxZombieReviveExtraMs bounds the configured extra wait.
const maxZombieReviveExtraMs = 30000

// zombieNoticeGrace is how long past the catalog's respawn wait the fallback
// notice waits for a client that never reports its respawn completion. It is a
// variable so unit tests can shorten it.
var zombieNoticeGrace = 2 * time.Second

// ZombieRound reports whether the room plays the zombie battle.
func (r *Room) ZombieRound() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Setting.Game.Mode == zombieModeID
}

// ZombieKingSlot reports the seat that turned into the zombie king, or -1.
func (r *Room) ZombieKingSlot() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.zombieKingSlot
}

// beginZombieRound puts every occupied seat on the human team and returns the
// native team-change notices the caller must broadcast. It runs before the
// start's seat records are built, so those already carry the human team.
func (r *Room) beginZombieRound() []protocol.Packet {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Setting.Game.Mode != zombieModeID {
		return nil
	}
	r.zombieKingSlot = -1
	r.zombieWinner = 0
	r.zombiePendingNotice = [maxRoomSlots]bool{}
	r.zombieReviveExtra = [maxRoomSlots]time.Duration{}
	if r.zombieKingTimer != nil {
		r.zombieKingTimer.Stop()
		r.zombieKingTimer = nil
	}
	var notices []protocol.Packet
	for slot := range r.slots {
		if r.slots[slot].session == nil {
			continue
		}
		r.slots[slot].team = zombieHumanTeam
		if notice, err := protocol.CreateRoomTeamReply(uint32(slot), zombieHumanTeam); err == nil {
			notices = append(notices, notice)
		}
	}
	return notices
}

// scheduleZombieOpening arms the king's pick for the round that just started.
// A later round's generation and the room's shutdown both cancel it.
func (s *Server) scheduleZombieOpening(room *Room, generation int) {
	if !room.ZombieRound() {
		return
	}
	room.mu.Lock()
	if room.zombieKingTimer != nil {
		room.zombieKingTimer.Stop()
	}
	room.zombieKingTimer = time.AfterFunc(zombieOpening, func() {
		s.pickZombieKing(room, generation)
	})
	room.mu.Unlock()
}

// pickZombieKing moves one random human to the zombie team once the opening
// passed. A seat that died before the opening is already the king, so the picker
// then does nothing.
func (s *Server) pickZombieKing(room *Room, generation int) {
	var notice protocol.Packet
	var king int
	room.mu.Lock()
	if !room.Started || room.generation != generation || room.zombieKingSlot >= 0 ||
		room.Setting.Game.Mode != zombieModeID {
		room.mu.Unlock()
		return
	}
	humans := make([]int, 0, maxRoomSlots)
	for slot := range room.slots {
		if room.slots[slot].session != nil && room.slots[slot].team == zombieHumanTeam {
			humans = append(humans, slot)
		}
	}
	if len(humans) == 0 {
		room.mu.Unlock()
		return
	}
	king = humans[rand.IntN(len(humans))]
	room.slots[king].team = zombieTeam
	room.zombieKingSlot = king
	built, err := protocol.CreateRoomTeamReply(uint32(king), zombieTeam)
	room.mu.Unlock()
	if err != nil {
		// Without the notice no client shows the king; the round would run on with
		// a zombie nobody can see.
		s.log.Error("[Zombie] the king's notice could not be encoded", "room", room.ID, "slot", king, "error", err)
		return
	}
	notice = built
	s.log.Info("[Zombie] the king turned", "room", room.ID, "slot", king, "candidates", humans)
	room.Broadcast(notice)
}

// zombieDeathReport answers the client's native team-change request during a
// zombie round. kr-812 sub_76C090 sends it on every death of the local player, so
// it carries no killer: a human who falls to the map turns as well. The request
// names no seat, so only the sender's own seat may turn.
func (s *Server) zombieDeathReport(session *Session) {
	room := session.room
	if room == nil {
		return
	}
	room.mu.RLock()
	owner := session.slot >= 0 && session.slot < maxRoomSlots && room.slots[session.slot].session == session
	slot := session.slot
	room.mu.RUnlock()
	if !owner {
		return
	}
	s.zombieDeathSlot(room, slot)
}

// zombieDeathOfObject is the death the combat stream reports instead of the team
// notice: the event names the victim's object, so that seat turns (not the
// reporter's).
func (s *Server) zombieDeathOfObject(room *Room, victimObject uint32) {
	if protocol.ValidateObject(victimObject) != nil {
		return
	}
	s.zombieDeathSlot(room, int((victimObject-1000000)/100000))
}

// zombieDeathSlot turns the seat that died into a zombie (or decides the round
// when the king died).
func (s *Server) zombieDeathSlot(room *Room, slot int) {
	var (
		finish   uint8
		announce bool
		zombini  bool
		kingSlot int
	)
	room.mu.Lock()
	if !room.Started || room.phase != phasePlaying || room.Setting.Game.Mode != zombieModeID ||
		slot < 0 || slot >= maxRoomSlots || room.slots[slot].session == nil {
		room.mu.Unlock()
		return
	}
	switch team := room.slots[slot].team; {
	case team == zombieTeam && slot == room.zombieKingSlot:
		// The king never comes back: its fall gives the humans the round.
		finish = zombieHumanTeam
		room.zombieWinner = zombieHumanTeam
		room.mu.Unlock()
		s.log.Info("[Zombie] the king fell", "room", room.ID, "slot", slot)
		s.finishRound(room, false)
		return
	case team == zombieHumanTeam:
		// A human who died before the opening is the first seat on the zombie
		// team, so every client shows it as the king.
		if room.zombieKingSlot < 0 {
			room.zombieKingSlot = slot
		}
		room.slots[slot].team = zombieTeam
		announce = true
	case team == zombieTeam:
		// A zombini comes back without limit; its notice follows the respawn wait.
		announce = true
		zombini = true
	default:
		room.mu.Unlock()
		return
	}
	// The king's branch returned above, so only a conversion reaches this: the last
	// human turning hands the round to the zombies.
	if room.zombieKingSlot >= 0 {
		humans := 0
		for index := range room.slots {
			if room.slots[index].session != nil && room.slots[index].team == zombieHumanTeam {
				humans++
			}
		}
		if humans == 0 && room.occupiedLocked() >= 2 {
			// The last human turned: the zombies win.
			finish = zombieTeam
			room.zombieWinner = zombieTeam
		}
	}
	kingSlot = room.zombieKingSlot
	room.mu.Unlock()

	s.log.Info("[Zombie] seat turned", "room", room.ID, "slot", slot, "king", kingSlot)
	if finish != 0 {
		// The last human's turn *is* the round's end, so its notice has to go out
		// here: announced later it would never arrive, and every client would still
		// count a human and show the human-survived banner.
		if announce {
			if notice := zombieNotice(slot); notice != nil {
				s.log.Info("[Zombie] conversion announced at the round end", "room", room.ID, "slot", slot)
				room.Broadcast(*notice)
			}
		}
		s.finishRound(room, false)
		return
	}
	if !announce {
		return
	}
	// The extra wait is chosen by what the seat was: a converted human plays the
	// human bar, a zombini the zombie one, and the measured round put them at 80 %
	// and 50 % of their bars with the catalog wait alone.
	room.mu.Lock()
	if slot >= 0 && slot < maxRoomSlots {
		extra := s.cfg.Zombie.HumanReviveExtraMs
		if zombini {
			extra = s.cfg.Zombie.ZombiniReviveExtraMs
		}
		if extra < 0 {
			extra = 0
		}
		if extra > maxZombieReviveExtraMs {
			extra = maxZombieReviveExtraMs
		}
		room.zombieReviveExtra[slot] = time.Duration(extra) * time.Millisecond
	}
	room.mu.Unlock()
	// The round's own state changes now: hostility and the win conditions must see
	// the seat on the zombie team. The clients are told after the respawn wait, so a
	// converted seat plays its bar and comes back as a zombie instead of turning on
	// the spot.
	if life := room.Life(); life != nil {
		if s.cfg.Zombie.ServerRespawn {
			life.MarkDownAndConvert(uint8(slot), zombieTeam)
		} else {
			life.ConvertTeam(uint8(slot), zombieTeam)
		}
	}
	if s.cfg.Zombie.ServerRespawn {
		s.tryRespawn(room, slot)
	}
	s.scheduleZombieNotice(room, slot)
}

// scheduleZombieNotice arms a seat's conversion notice. The notice is what the
// client's own zombie path reads as "come back", so it must not arrive before the
// bar finished: it goes out when the client completes its respawn, and this timer
// is the fallback for a client that never reports the completion.
func (s *Server) scheduleZombieNotice(room *Room, slot int) {
	room.mu.Lock()
	if slot >= 0 && slot < maxRoomSlots {
		room.zombiePendingNotice[slot] = true
	}
	character, items := room.slots[slot].character, room.slots[slot].items
	generation := room.generation
	room.mu.Unlock()
	// The fallback waits a little longer than the catalog: the completion report is
	// the normal trigger, this only covers a client that never sends it.
	delay := s.respawnDelay(character, items) + zombieNoticeGrace
	if room.ZombieRound() {
		room.mu.RLock()
		extra := room.zombieReviveExtra[slot]
		room.mu.RUnlock()
		delay = s.zombieRespawnDelay(character, items) + extra + zombieNoticeGrace
	}
	time.AfterFunc(delay, func() {
		room.mu.RLock()
		stale := !room.Started || room.generation != generation ||
			room.Setting.Game.Mode != zombieModeID
		room.mu.RUnlock()
		if stale {
			return
		}
		s.announceZombieConversion(room, slot)
	})
}

// announceZombieConversion tells every client that one seat is on the zombie team,
// once. The respawn completion and the fallback timer both land here.
func (s *Server) announceZombieConversion(room *Room, slot int) {
	room.mu.Lock()
	if !room.Started || room.Setting.Game.Mode != zombieModeID ||
		slot < 0 || slot >= maxRoomSlots || !room.zombiePendingNotice[slot] {
		room.mu.Unlock()
		return
	}
	room.zombiePendingNotice[slot] = false
	room.mu.Unlock()
	notice := zombieNotice(slot)
	if notice == nil {
		s.log.Error("[Zombie] the conversion notice could not be encoded", "room", room.ID, "slot", slot)
		return
	}
	s.log.Info("[Zombie] conversion announced", "room", room.ID, "slot", slot)
	room.Broadcast(*notice)
}

// zombieUltraItemKind is the item kind the king's ultra request carries:
// feverDrink 104, sent with no scene object (cn-324 sub_508FC0 -> event 55 ->
// cn-324 sub_667CC0 -> sub_4672B0(0x68, -1)).
const zombieUltraItemKind = 104

// zombieNoObject is the object word the ultra request carries: the client passes
// -1 because the effect belongs to no scene object.
const zombieNoObject = 0xFFFFFFFF

// handleZombieItemRequest answers the king's ultra: the client checks its own
// gauge before it sends the request, so the round only checks that the sender is
// this round's king and confirms the effect to that player alone
// (gspPveItemUsed, cn-717 sub_7F4370 reads seat, kind, unused word).
func (s *Server) handleZombieItemRequest(session *Session, packet protocol.Packet) {
	room := session.room
	if room == nil {
		return
	}
	objectID, kind, err := protocol.ReadSupplyPickup(packet)
	if err != nil {
		return
	}
	if objectID != zombieNoObject || kind != zombieUltraItemKind {
		// Not the ultra: the mode's ammo boxes use the same request, so the box
		// pickup path answers it.
		_ = s.handleEatItem(session, packet)
		return
	}
	room.mu.RLock()
	playing := room.Started && room.phase == phasePlaying && room.Setting.Game.Mode == zombieModeID
	king := room.zombieKingSlot
	room.mu.RUnlock()
	if !playing || king != session.slot {
		s.log.Info("[Zombie] ultra request outside the king's round ignored", "room", room.ID, "slot", session.slot)
		return
	}
	_ = session.Send(protocol.CreatePveItemUsedReply(uint32(session.slot), zombieUltraItemKind))
	s.log.Info("[Zombie] the king used its ultra", "room", room.ID, "slot", session.slot)
}

// zombieNotice builds the team-change notice for one seat, or nil when the
// encoder refused it.
func zombieNotice(slot int) *protocol.Packet {
	notice, err := protocol.CreateRoomTeamReply(uint32(slot), zombieTeam)
	if err != nil {
		return nil
	}
	return &notice
}

// zombieLeave decides what the departure of one zombie-round member means: the
// king's departure wins the round for the humans, and the last human's leaves it
// to the zombies. It reports false when the departure says nothing (the caller
// then cancels the round as in every other mode).
func (s *Server) zombieLeave(room *Room, slot int) (handled bool) {
	room.mu.Lock()
	if !room.Started || room.Setting.Game.Mode != zombieModeID {
		room.mu.Unlock()
		return false
	}
	if slot == room.zombieKingSlot {
		room.zombieWinner = zombieHumanTeam
		room.mu.Unlock()
		s.log.Info("[Zombie] the king left", "room", room.ID, "slot", slot)
		s.finishRound(room, false)
		return true
	}
	if room.occupiedLocked() == 0 {
		room.mu.Unlock()
		return false
	}
	humans := 0
	for index := range room.slots {
		if room.slots[index].session != nil && room.slots[index].team == zombieHumanTeam {
			humans++
		}
	}
	if humans > 0 || room.zombieKingSlot < 0 {
		room.mu.Unlock()
		return false
	}
	room.zombieWinner = zombieTeam
	room.mu.Unlock()
	s.log.Info("[Zombie] the last human left", "room", room.ID)
	s.finishRound(room, false)
	return true
}
