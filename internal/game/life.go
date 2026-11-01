package game

import (
	"fmt"
	"sync"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// rescueWindow bounds the server-side correlation between a rescue request and
// its confirmation. It is a server policy value, not a protocol constant.
const rescueWindow = 5 * time.Second

type lifeState int

const (
	stateActive lifeState = iota
	stateStunned
	stateStunRecovering
	stateBubbled
	stateKicked
	stateDying
	stateDead
	stateRespawnWaiting
	stateRespawnOffered
	stateRespawnConfirmed
)

type lifePlayer struct {
	objectID uint32
	team     uint8
	state    lifeState

	bubbleSettled bool
	knockTick     *uint32

	rescueRequests [8]*time.Time
	kickRequests   [8]*time.Time
	kickTarget     *uint32
	kickStyle      uint32
	clearKick      bool

	completedRescuer *uint32
	completionUntil  time.Time

	deadReported bool
	respawnAsked *time.Time
	spawnIndex   uint32
	score        protocol.BattleScore
}

// BattleLife tracks victim-reported knockdown, resuce and kick events for one
// match. The original server never authored health; it correlated the events
// the clients report and tallied the score.
type BattleLife struct {
	// mu guards every player field: life events arrive on the reporting
	// player's goroutine while scores and respawns are read elsewhere.
	mu      sync.RWMutex
	players [8]lifePlayer
	now     func() time.Time
}

// NewBattleLife builds the tracker.
func NewBattleLife(now func() time.Time) *BattleLife {
	if now == nil {
		now = time.Now
	}
	return &BattleLife{now: now}
}

// Bind associates a slot with its round object id and frozen team.
func (l *BattleLife) Bind(slot uint8, objectID uint32, team uint8) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slot >= 8 || objectID < 1000000 || (objectID-1000000)/100000 != uint32(slot) ||
		team < 1 || team > 2 {
		return fmt.Errorf("battle life requires a player object and frozen team")
	}
	player := &l.players[slot]
	if player.objectID != 0 && (player.objectID != objectID || player.team != team) {
		return fmt.Errorf("character initialization changed object or team within the round")
	}
	player.objectID = objectID
	player.team = team
	return nil
}

// AdoptSpawnIndex lets the client's own marker win, as long as it still belongs
// to the team block. Anything outside the block is a protocol error.
func (l *BattleLife) AdoptSpawnIndex(slot uint8, spawnIndex uint32) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slot >= 8 || l.players[slot].objectID == 0 {
		return fmt.Errorf("spawn adoption requires an initialized player")
	}
	player := &l.players[slot]
	if spawnIndex/10 != uint32(player.team) || spawnIndex%10 < 1 || spawnIndex%10 > 8 {
		return fmt.Errorf("respawn confirmation changed the selected spawn")
	}
	player.spawnIndex = spawnIndex
	return nil
}

// SpawnIndex returns the marker the server last offered to one slot.
func (l *BattleLife) SpawnIndex(slot uint8) uint32 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slot >= 8 {
		return 0
	}
	return l.players[slot].spawnIndex
}

// Reset clears the whole tracker for a new round.
func (l *BattleLife) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.players = [8]lifePlayer{}
}

func (l *BattleLife) find(objectID uint32) (uint8, error) {
	if objectID < 1000000 || (objectID-1000000)/100000 >= 8 {
		return 0, fmt.Errorf("life event has an invalid player object")
	}
	slot := uint8((objectID - 1000000) / 100000)
	if l.players[slot].objectID != objectID {
		return 0, fmt.Errorf("life event object was not initialized by this round")
	}
	return slot, nil
}

// CanFire reports whether the player may attack.
func (l *BattleLife) CanFire(slot uint8) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slot >= 8 || l.players[slot].objectID == 0 {
		return false
	}
	return l.players[slot].state == stateActive
}

// ReturnToNormal leaves stun recovery.
func (l *BattleLife) ReturnToNormal(slot uint8) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slot >= 8 || l.players[slot].objectID == 0 {
		return false
	}
	if l.players[slot].state == stateStunRecovering {
		l.players[slot].state = stateActive
	}
	return l.players[slot].state == stateActive
}

// AreOpponents reports the frozen team relation.
func (l *BattleLife) AreOpponents(sourceSlot uint8, targetObject uint32) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if sourceSlot >= 8 || l.players[sourceSlot].objectID == 0 {
		return false, fmt.Errorf("damage source requires an initialized member")
	}
	target, err := l.find(targetObject)
	if err != nil {
		return false, err
	}
	return l.players[sourceSlot].team != l.players[target].team, nil
}

// Score returns a slot's tally.
func (l *BattleLife) Score(slot uint8) (protocol.BattleScore, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slot >= 8 || l.players[slot].objectID == 0 {
		return protocol.BattleScore{}, fmt.Errorf("player score requires an initialized member")
	}
	return l.players[slot].score, nil
}

// Team returns the frozen team of a slot.
func (l *BattleLife) Team(slot uint8) uint8 { return l.players[slot].team }

// ObjectID returns the slot's object id.
func (l *BattleLife) ObjectID(slot uint8) uint32 { return l.players[slot].objectID }

func increment(value *uint16) {
	if *value < 0xFFFF {
		*value++
	}
}

func (l *BattleLife) validateReporter(slot uint8, objectID uint32) error {
	found, err := l.find(objectID)
	if err != nil {
		return err
	}
	if found != slot {
		return fmt.Errorf("peer event does not identify the bound character")
	}
	return nil
}

func (l *BattleLife) clearTransient(player *lifePlayer) {
	player.rescueRequests = [8]*time.Time{}
	player.kickRequests = [8]*time.Time{}
	player.kickTarget = nil
	player.completedRescuer = nil
}

// Apply processes one life event reported by sourceSlot.
func (l *BattleLife) Apply(sourceSlot uint8, event protocol.PeerLifeEvent, allowSelfDamage bool) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	victimSlot, err := l.find(event.VictimObject)
	if err != nil {
		return false, err
	}

	switch event.Kind {
	case protocol.LifeEndStunned:
		if err := l.validateReporter(sourceSlot, event.VictimObject); err != nil {
			return false, err
		}
		victim := &l.players[victimSlot]
		if victim.state != stateStunned {
			return false, nil
		}
		victim.state = stateStunRecovering
		return true, nil

	case protocol.LifeBubbleSettled, protocol.LifeSelfPopOut:
		if err := l.validateReporter(sourceSlot, event.VictimObject); err != nil {
			return false, err
		}
		victim := &l.players[victimSlot]
		if victim.state != stateBubbled {
			return false, nil
		}
		if event.Kind == protocol.LifeBubbleSettled {
			if victim.bubbleSettled {
				return false, nil
			}
			victim.bubbleSettled = true
			return true, nil
		}
		if !victim.bubbleSettled || event.ActorObject != protocol.Unset {
			return false, nil
		}
		victim.state = stateActive
		victim.bubbleSettled = false
		victim.respawnAsked = nil
		l.clearTransient(victim)
		return true, nil

	case protocol.LifeDying, protocol.LifeDead:
		if err := l.validateReporter(sourceSlot, event.VictimObject); err != nil {
			return false, err
		}
		victim := &l.players[victimSlot]
		if event.Kind == protocol.LifeDying {
			if victim.state != stateBubbled {
				return false, nil
			}
			victim.state = stateDying
		} else {
			if victim.deadReported || victim.state == stateDead ||
				victim.state == stateRespawnOffered || victim.state == stateRespawnConfirmed {
				return false, nil
			}
			victim.deadReported = true
			if victim.state != stateRespawnWaiting {
				victim.state = stateDead
			}
		}
		l.clearTransient(victim)
		return true, nil
	}

	actorSlot, err := l.find(event.ActorObject)
	if err != nil {
		return false, err
	}
	reporter := victimSlot
	if event.Kind == protocol.LifeTryRescue {
		reporter = actorSlot
	}
	selfDamage := allowSelfDamage &&
		(event.Kind == protocol.LifeKnocked || event.Kind == protocol.LifeStunned) &&
		victimSlot == actorSlot
	if sourceSlot != reporter || (victimSlot == actorSlot && !selfDamage) {
		return false, fmt.Errorf("life event was not sent by its authenticated reporter")
	}

	victim := &l.players[victimSlot]
	actor := &l.players[actorSlot]

	switch event.Kind {
	case protocol.LifeStunned:
		if (!selfDamage && victim.team == actor.team) || victim.state != stateActive {
			return false, nil
		}
		victim.state = stateStunned
		victim.kickTarget = nil
		return true, nil

	case protocol.LifeKnocked:
		tick := event.Tick & 0xFFFFFF
		elapsed := uint32(1)
		if victim.knockTick != nil {
			elapsed = (tick - *victim.knockTick) & 0xFFFFFF
		}
		opponents := victim.team != actor.team
		if (!selfDamage && !opponents) || victim.state != stateActive ||
			elapsed == 0 || elapsed >= 0x800000 {
			return false, nil
		}
		tickCopy := tick
		victim.knockTick = &tickCopy
		victim.deadReported = false
		if event.DirectDeath {
			victim.state = stateDying
		} else {
			victim.state = stateBubbled
		}
		victim.bubbleSettled = false
		l.clearTransient(victim)
		// A direct death is the hit that took the victim's last life: the client
		// reports it as a knockdown whose victim never enters a bubble, so no
		// kick ever follows and the kill would otherwise go uncounted. Credit the
		// attacker with the pang, which is also what puts the +1 on their team,
		// because a team's score is the sum of its members' pang counts.
		if event.DirectDeath {
			if opponentTeam(victim.team, actor.team) && !selfDamage {
				increment(&actor.score.PangCount)
			}
			return true, nil
		}
		if opponentTeam(victim.team, actor.team) {
			increment(&actor.score.BubbleCount)
		}
		return true, nil
	}

	if victim.team != actor.team {
		return false, fmt.Errorf("rescue requires members of the same frozen team")
	}

	now := l.now()
	switch event.Kind {
	case protocol.LifeTryRescue:
		if victim.state != stateBubbled || actor.state != stateActive {
			return false, nil
		}
		deadline := now.Add(rescueWindow)
		victim.rescueRequests[actorSlot] = &deadline
		return true, nil

	case protocol.LifePopOut:
		request := victim.rescueRequests[actorSlot]
		if victim.state != stateBubbled || actor.state != stateActive ||
			request == nil || !request.After(now) {
			return false, nil
		}
		victim.state = stateActive
		victim.bubbleSettled = false
		victim.respawnAsked = nil
		actorCopy := event.ActorObject
		l.clearTransient(victim)
		victim.completedRescuer = &actorCopy
		victim.completionUntil = now.Add(rescueWindow)
		return true, nil

	case protocol.LifeRescue:
		if event.SpecialRescue || victim.state != stateActive ||
			victim.completedRescuer == nil || *victim.completedRescuer != event.ActorObject ||
			!victim.completionUntil.After(now) {
			return false, nil
		}
		victim.completedRescuer = nil
		increment(&actor.score.RescueCount)
		return true, nil
	}
	return false, nil
}

func opponentTeam(a, b uint8) bool { return a != b }

// ApplyKick processes one kick event.
//
// The original tallies a Pang when the kicker confirms a successful kick on an
// opponent's settled bubble; other kinds only move state.
func (l *BattleLife) ApplyKick(sourceSlot uint8, event protocol.PeerKickEvent) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch event.Kind {
	case protocol.KickHumiliationStart, protocol.KickHumiliationEnd:
		if _, err := l.find(event.Actor); err != nil {
			return false, err
		}
		if _, err := l.find(event.Victim); err != nil {
			return false, err
		}
		return true, nil

	case protocol.KickGotKicked:
		victimSlot, err := l.find(event.Victim)
		if err != nil {
			return false, err
		}
		if err := l.validateReporter(sourceSlot, event.Victim); err != nil {
			return false, err
		}
		victim := &l.players[victimSlot]
		if victim.state != stateBubbled {
			return false, nil
		}
		victim.state = stateKicked
		return true, nil

	case protocol.KickFailed, protocol.KickEnd:
		actorSlot, err := l.find(event.Actor)
		if err != nil {
			return false, err
		}
		if err := l.validateReporter(sourceSlot, event.Actor); err != nil {
			return false, err
		}
		l.players[actorSlot].kickTarget = nil
		return true, nil

	case protocol.KickStart:
		actorSlot, err := l.find(event.Actor)
		if err != nil {
			return false, err
		}
		victimSlot, err := l.find(event.Victim)
		if err != nil {
			return false, err
		}
		if err := l.validateReporter(sourceSlot, event.Actor); err != nil {
			return false, err
		}
		actor := &l.players[actorSlot]
		victim := &l.players[victimSlot]
		if actor.team == victim.team || victim.state != stateBubbled {
			return false, nil
		}
		victimObject := event.Victim
		actor.kickTarget = &victimObject
		actor.kickStyle = event.Style
		return true, nil
	}

	// 32794 try, 32795 confirmed, 32796 protected
	victimSlot, err := l.find(event.Victim)
	if err != nil {
		return false, err
	}
	actorSlot, err := l.find(event.Actor)
	if err != nil {
		return false, err
	}
	// The reference protocol splits reporter duties: contact attempts come from
	// the kicker, while the outcome (confirmed/protected) is reported by the
	// victim. Requiring the actor for every kind silently drops every real kill.
	expected := victimSlot
	if event.Kind == protocol.KickTry {
		expected = actorSlot
	}
	if sourceSlot != expected {
		return false, fmt.Errorf("kick event was not sent by its authenticated reporter")
	}
	victim := &l.players[victimSlot]
	actor := &l.players[actorSlot]
	if actor.team == victim.team {
		return false, fmt.Errorf("kick requires opposing teams")
	}
	if victim.state != stateBubbled || actor.state != stateActive {
		return false, nil
	}
	request := victim.kickRequests[actorSlot]
	pending := request != nil && request.After(l.now())
	switch event.Kind {
	case protocol.KickProtected:
		if !event.Succeeded || event.SpecialKick || !pending {
			return false, nil
		}
		victim.kickRequests[actorSlot] = nil
		return true, nil
	case protocol.KickConfirmed:
		if !event.Succeeded || event.SpecialKick || !pending {
			return false, nil
		}
		victim.state = stateKicked
		victim.kickRequests = [8]*time.Time{}
		victim.rescueRequests = [8]*time.Time{}
		increment(&actor.score.PangCount)
		return true, nil
	default: // KickTry
		deadline := l.now().Add(rescueWindow)
		victim.kickRequests[actorSlot] = &deadline
		victimObject := event.Victim
		actor.kickTarget = &victimObject
		return true, nil
	}
}

// RequestRespawn records a respawn request from either socket.
func (l *BattleLife) RequestRespawn(slot uint8, objectID uint32) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.validateReporter(slot, objectID); err != nil {
		return err
	}
	player := &l.players[slot]
	if player.state == stateRespawnWaiting || player.state == stateRespawnOffered ||
		player.state == stateRespawnConfirmed {
		return nil
	}
	now := l.now()
	if player.respawnAsked == nil || now.Sub(*player.respawnAsked) >= rescueWindow {
		asked := now
		player.respawnAsked = &asked
	}
	return nil
}

// BeginRespawn moves a downed player into the respawn wait.
func (l *BattleLife) BeginRespawn(slot uint8) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slot >= 8 {
		return false
	}
	player := &l.players[slot]
	if (player.state != stateDying && player.state != stateDead) ||
		player.respawnAsked == nil || l.now().Sub(*player.respawnAsked) >= rescueWindow {
		return false
	}
	player.state = stateRespawnWaiting
	return true
}

// OfferRespawn picks the spawn point and returns the victim object id.
func (l *BattleLife) OfferRespawn(slot uint8, spawnIndex uint32) (uint32, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slot >= 8 || l.players[slot].state != stateRespawnWaiting {
		return 0, fmt.Errorf("respawn offer requires a waiting player")
	}
	player := &l.players[slot]
	if spawnIndex/10 != uint32(player.team) || spawnIndex%10 < 1 || spawnIndex%10 > 8 {
		return 0, fmt.Errorf("respawn offer is outside the team spawn group")
	}
	player.spawnIndex = spawnIndex
	player.state = stateRespawnOffered
	return player.objectID, nil
}

// ConfirmRespawn validates the client's confirmation.
func (l *BattleLife) ConfirmRespawn(slot uint8, objectID, spawnIndex uint32) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.validateReporter(slot, objectID); err != nil {
		return false, err
	}
	player := &l.players[slot]
	if player.state != stateRespawnOffered {
		return false, nil
	}
	if spawnIndex != player.spawnIndex {
		// The client answers with the marker it actually used. It is authoritative
		// for its own character, and refusing here would skip the S707 broadcast
		// that tells every observer the respawn finished, so adopt a marker that
		// still belongs to the team block and only reject anything outside it.
		if spawnIndex/10 != uint32(player.team) || spawnIndex%10 < 1 || spawnIndex%10 > 8 {
			return false, fmt.Errorf("respawn confirmation changed the selected spawn")
		}
		player.spawnIndex = spawnIndex
	}
	player.state = stateRespawnConfirmed
	return true, nil
}

// CompleteRespawn finishes the flow and returns the player to active play.
func (l *BattleLife) CompleteRespawn(slot uint8, objectID, spawnIndex uint32) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.validateReporter(slot, objectID); err != nil {
		return false, err
	}
	player := &l.players[slot]
	if player.state != stateRespawnConfirmed {
		return false, nil
	}
	if spawnIndex != player.spawnIndex {
		return false, fmt.Errorf("relayed respawn did not match this authorization")
	}
	player.state = stateActive
	player.bubbleSettled = false
	player.deadReported = false
	player.respawnAsked = nil
	l.clearTransient(player)
	return true, nil
}
