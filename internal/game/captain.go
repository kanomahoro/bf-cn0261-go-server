package game

import (
	"math/rand/v2"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// The Captain battle (game mode 20).
//
// The 2015 client keeps no rules for this mode either: its Captain manager only
// reacts to the native seat announcement. Every team is given one Captain when
// the round starts; the client then marks that seat's character, gives it the
// Captain's health and gauge, and sends its ULTRA request when the player presses
// the key with the gauge full. When the Captain dies the Captaincy passes to a
// standing teammate, which the server announces again. Everything else -- the
// score, the clock, the respawn wait and the settlement -- is the ordinary
// competitive round.
//
// The ULTRA needs no packet of its own: the client asks with the ordinary pickup
// carrying no scene object and item kind 104, and the round confirms with S715
// gspPveItemUsed to that player alone. The zombie king's ULTRA is the same
// exchange on this client (see zombie.go), so the confirmation path is already
// proven against a real client.
//
// Reference: BubbleFighter.Fantasy RoundFlow.Captain.cs and
// packets/mode-control.yaml (gspCreateCaptain 982, gcpDropCaptain 2018); the
// cn-717 addresses are in that file's header.
const (
	// captainNone marks a team without a Captain: nobody is standing, or the
	// round has not appointed one yet.
	captainNone = -1
	// captainUltraItemKind is the kind the Captain's ULTRA request carries. With
	// its gauge ready the client raises the Captain manager's event 55, whose
	// handler sends the no-object request for kind 104 (feverDrink).
	captainUltraItemKind = 104
	// captainNoObject is the object word that request carries: the client passes
	// no scene object, so the whole word is set.
	captainNoObject = 0xFFFFFFFF
	// captainAppointDelay is the wait between the round start and the seat
	// announcement. The client's handler reads the per-seat character table and
	// does nothing for a seat that has no character yet, so the announcement has to
	// follow the characters' creation. A second covers that with room to spare and
	// is invisible in a round this long: measured on a real client, 800 ms and
	// 1000 ms look the same, and the wait only ever applies to the opening -- every
	// later hand-over is event driven and goes out immediately.
	//
	// The reference sends both announcements in the round-start operation itself,
	// right after the start packet (BubbleFighter.Fantasy RoundFlow.cs), which
	// assumes the clients build their characters before the round reaches playing.
	// This server keeps the second as insurance for its own start order rather than
	// risk losing the appointment for a whole round.
	captainAppointDelay = 1000 * time.Millisecond
	// captainTeamCount is the number of battle teams that get a Captain: team 1
	// and team 2.
	captainTeamCount = 2
)

// CaptainRound reports whether the room plays the Captain battle.
func (r *Room) CaptainRound() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Setting.Game.Mode == captainModeID
}

// CaptainSlot reports the Captain of one battle team, or captainNone.
func (r *Room) CaptainSlot(team uint8) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if team < 1 || team > captainTeamCount {
		return captainNone
	}
	return r.captains[team]
}

// scheduleCaptainAppointment arms the opening announcement for the round that
// just started. A later round's generation and the room's shutdown both cancel
// it, the same way the zombie battle's opening is fenced.
func (s *Server) scheduleCaptainAppointment(room *Room, generation int) {
	if !room.CaptainRound() {
		return
	}
	team1, team2 := room.teamSeats()
	s.log.Debug("[Captain] opening armed", "room", room.ID, "team1", team1, "team2", team2,
		"delayMs", captainAppointDelay.Milliseconds())
	room.mu.Lock()
	if room.captainTimer != nil {
		room.captainTimer.Stop()
		room.captainTimer = nil
	}
	room.captainTimer = time.AfterFunc(captainAppointDelay, func() {
		s.refreshCaptains(room, generation)
	})
	room.mu.Unlock()
}

// teamSeats counts the occupied seats of each battle team, for the round's own
// log line.
func (r *Room) teamSeats() (int, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	first, second := 0, 0
	for slot := range r.slots {
		if r.slots[slot].session == nil {
			continue
		}
		if r.slots[slot].team == 2 {
			second++
		} else if r.slots[slot].team == 1 {
			first++
		}
	}
	return first, second
}

// captainsAfterChange re-checks both teams after a death, a respawn or a
// departure. The round's state is read here so the call sites staged outside the
// room lock stay readable.
func (s *Server) captainsAfterChange(room *Room) {
	if room == nil || !room.CaptainRound() {
		return
	}
	room.mu.RLock()
	generation := room.generation
	room.mu.RUnlock()
	s.refreshCaptains(room, generation)
}

// refreshCaptains names a Captain for every team that has none standing. It is
// idempotent: a team whose Captain is still on its feet keeps it, so a repeated
// death report for a seat that already handed the Captaincy on changes nothing.
func (s *Server) refreshCaptains(room *Room, generation int) {
	life := room.Life()
	if life == nil {
		return
	}
	var appointed []int
	var members []memberView
	room.mu.Lock()
	if !room.Started || room.phase != phasePlaying || room.generation != generation ||
		room.Setting.Game.Mode != captainModeID {
		room.mu.Unlock()
		return
	}
	for team := uint8(1); team <= captainTeamCount; team++ {
		current := room.captains[team]
		if current != captainNone && life.Standing(uint8(current)) {
			continue
		}
		pick, ok := pickCaptain(room, life, team, current)
		if !ok {
			// Nobody on that side can hold the Captaincy right now. The team is
			// vacated rather than left pointing at a fallen seat: a respawned
			// character is a new one on the client, so the Captain has to be
			// announced again, and the check that runs after the respawn does that
			// as soon as somebody stands. (The reference vacates the same way:
			// RoundFlow.Captain.cs ReplaceDepartedCaptainsAsync.)
			room.captains[team] = captainNone
			continue
		}
		room.captains[team] = pick
		appointed = append(appointed, pick)
	}
	members = room.membersLocked()
	room.mu.Unlock()

	for _, slot := range appointed {
		s.announceCaptain(room, members, slot)
	}
}

// pickCaptain chooses a teammate able to take the Captaincy over, preferring
// anybody but the seat that just lost it. A seat caught in a bubble is skipped:
// the client's own succession does not promote one either, and announcing it puts
// a second Captain mark on the team. The room lock is held by the caller.
func pickCaptain(room *Room, life *BattleLife, team uint8, exclude int) (int, bool) {
	candidates := make([]int, 0, maxRoomSlots)
	for slot := range room.slots {
		if room.slots[slot].session == nil || room.slots[slot].team != team {
			continue
		}
		if !life.Active(uint8(slot)) {
			continue
		}
		candidates = append(candidates, slot)
	}
	if len(candidates) == 0 {
		return captainNone, false
	}
	if len(candidates) > 1 && exclude != captainNone {
		rest := make([]int, 0, len(candidates))
		for _, slot := range candidates {
			if slot != exclude {
				rest = append(rest, slot)
			}
		}
		if len(rest) > 0 {
			candidates = rest
		}
	}
	return candidates[rand.IntN(len(candidates))], true
}

// announceCaptain sends the seat announcement to every seated member: the client
// marks the character on its own, and every team's Captain is shown to the whole
// room.
func (s *Server) announceCaptain(room *Room, members []memberView, slot int) {
	reply, err := protocol.CreateCaptainReply(uint32(slot))
	if err != nil {
		return
	}
	for _, member := range members {
		if member.Occupied && member.Session != nil {
			_ = member.Session.Send(reply)
		}
	}
	s.log.Info("[Captain] announced", "room", room.ID, "slot", slot,
		"team", room.Team(slot))
}

// handleCaptainDrop answers the client's own death report (C2018). The report
// names the seat that died and is sent for every local death, not only the
// Captain's, so it is only a trigger to re-check the appointments.
//
// A body this server cannot read is logged and ignored rather than refused: the
// report carries no state of its own, and a handler error ends the player's
// connection for a packet the round can do without.
func (s *Server) handleCaptainDrop(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || !room.CaptainRound() {
		return nil
	}
	seat, err := protocol.ReadDropCaptain(packet)
	if err != nil {
		s.log.Debug("[Captain] drop report ignored", "room", room.ID, "slot", session.slot,
			"len", len(packet.Payload), "error", err)
		return nil
	}
	// The reference reads this body strictly (one word, no tail). The layout is
	// only a trigger for us, so a longer body is tolerated, but the length is
	// worth seeing while the exchange is still unverified on a real client.
	if len(packet.Payload) != 4 {
		s.log.Debug("[Captain] drop report has a tail", "room", room.ID, "slot", session.slot,
			"len", len(packet.Payload))
	}
	if int(seat) != session.slot {
		s.log.Debug("[Captain] drop report names another seat", "room", room.ID,
			"reporter", session.slot, "seat", seat)
		return nil
	}
	// A report about the seat that currently holds the Captaincy is the case the
	// real-client test needs to see; every other death in the round is noise.
	if current := room.CaptainSlot(room.Team(session.slot)); current == session.slot {
		s.log.Debug("[Captain] the Captain reported its own fall", "room", room.ID,
			"slot", session.slot)
	}
	s.captainsAfterChange(room)
	return nil
}

// handleCaptainUltra answers the Captain's ULTRA request. It reports whether the
// packet was the ULTRA request at all, so an ordinary box pickup in the same mode
// still takes the normal path.
//
// The client checks its own gauge and its own Captain identity before it sends
// the request, so the round only checks that the sender still holds its team's
// appointment and confirms the effect to that player alone: the confirmation's
// consumer raises the client's own direct effect, which the round feed passes on
// to the other players.
func (s *Server) handleCaptainUltra(session *Session, packet protocol.Packet) bool {
	objectID, kind, err := protocol.ReadSupplyPickup(packet)
	if err != nil || objectID != captainNoObject || kind != captainUltraItemKind {
		return false
	}
	room := session.room
	if room == nil {
		return true
	}
	captain := false
	room.mu.RLock()
	if room.Started && room.phase == phasePlaying && room.Setting.Game.Mode == captainModeID {
		for team := uint8(1); team <= captainTeamCount; team++ {
			if room.captains[team] == session.slot {
				captain = true
				break
			}
		}
	}
	room.mu.RUnlock()
	if !captain {
		s.log.Debug("[Captain] ultra request outside the Captain's round ignored",
			"room", room.ID, "slot", session.slot)
		return true
	}
	_ = session.Send(protocol.CreatePveItemUsedReply(uint32(session.slot), captainUltraItemKind))
	s.log.Debug("[Captain] the Captain used its ultra", "room", room.ID, "slot", session.slot)
	return true
}
