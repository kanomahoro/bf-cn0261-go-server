package protocol

import (
	"fmt"
	"math"
)

// The bomb mission (round mode 19) exchanges one state word and a plant
// position. The client's handler switches on the state word with cases 0..6
// (kr-812 sub_4245B0, cn-717 sub_5606F0), in the order below.
const (
	// BombStateReset is an unplanted bomb: no planter, no defuser, no position.
	BombStateReset uint32 = 0
	// BombStatePlanting is the plant animation holding the bomb.
	BombStatePlanting uint32 = 1
	// BombStateCountdown is an armed bomb, 60 s from detonation.
	BombStateCountdown uint32 = 2
	// BombStateExploded ends the sub-round for the attacking team.
	BombStateExploded uint32 = 3
	// BombStateDefusing is the defuse animation holding the bomb.
	BombStateDefusing uint32 = 4
	// BombStateDefuseInterrupted returns to the armed countdown without
	// restarting it.
	BombStateDefuseInterrupted uint32 = 5
	// BombStateDefused ends the sub-round for the defending team.
	BombStateDefused uint32 = 6
)

// BombOperatorNone is the wire's "no operator" reference: the client sends it in
// the planter and defuser words of a mission-over report that has neither.
const BombOperatorNone uint32 = 0xFFFFFFFF

// BombState is one gcpBombState report or one gspBombState broadcast: the acting
// client's operator reference, the state word and the plant position.
type BombState struct {
	Operator uint32
	State    uint32
	X, Y, Z  float32
}

// ReadBombState decodes C2012: five words, 20 bytes.
func ReadBombState(payload []byte) (BombState, error) {
	var out BombState
	r := NewReader(payload)
	operator, err := r.Unsigned()
	if err != nil {
		return out, err
	}
	state, err := r.Unsigned()
	if err != nil {
		return out, err
	}
	position := [3]float32{}
	for index := range position {
		bits, err := r.Unsigned()
		if err != nil {
			return out, err
		}
		position[index] = math.Float32frombits(bits)
	}
	if err := r.RequireEnd(); err != nil {
		return out, err
	}
	out.Operator, out.State = operator, state
	out.X, out.Y, out.Z = position[0], position[1], position[2]
	return out, nil
}

// CreateBombStateReply builds S973: six words, 24 bytes. The timer word stays
// clear, which makes the client run its own 60000 ms detonation countdown
// instead of aligning to a shared clock this server never publishes.
func CreateBombStateReply(state BombState) Packet {
	w := NewWriter()
	w.Unsigned(state.Operator)
	w.Unsigned(state.State)
	w.Unsigned(math.Float32bits(state.X))
	w.Unsigned(math.Float32bits(state.Y))
	w.Unsigned(math.Float32bits(state.Z))
	w.Boolean(false)
	return Packet{ID: 973, Payload: w.Payload()}
}

// BombMissionOver is one gcpBombMissionOver report: the team the reporting client
// saw win the sub-round, plus the operators that decided it (BombOperatorNone when
// there is none).
type BombMissionOver struct {
	WinningTeam uint32
	Planter     uint32
	Defuser     uint32
}

// ReadBombMissionOver decodes C2011: three words, 12 bytes.
func ReadBombMissionOver(payload []byte) (BombMissionOver, error) {
	var out BombMissionOver
	r := NewReader(payload)
	team, err := r.Unsigned()
	if err != nil {
		return out, err
	}
	planter, err := r.Unsigned()
	if err != nil {
		return out, err
	}
	defuser, err := r.Unsigned()
	if err != nil {
		return out, err
	}
	if err := r.RequireEnd(); err != nil {
		return out, err
	}
	out.WinningTeam, out.Planter, out.Defuser = team, planter, defuser
	return out, nil
}

// CreateObserverReply builds S974: the scene object of the player a respawn-less
// round turns into an observer. The word is a scene object, not a user number.
func CreateObserverReply(actor uint32) Packet {
	w := NewWriter()
	w.Unsigned(actor)
	return Packet{ID: 974, Payload: w.Payload()}
}

// RoundSeries is the sub-round data the client reads out of the entry packet and
// the game setting: which sub-round loads and which team attacks it.
type RoundSeries struct {
	Round     uint32
	Attacking uint32
}

// CreateGameEntrySeriesReply builds S510 for a series entry. The three words
// after the map key are the sub-round index, the attacking team and the room
// state, the same positions CreateGameEntryReply leaves zeroed.
func CreateGameEntrySeriesReply(result uint32, gameMap []uint16, state uint32, series RoundSeries) Packet {
	w := NewWriter()
	w.Unsigned(result)
	w.String(gameMap)
	w.Unsigned(series.Round)
	w.Unsigned(series.Attacking)
	w.Unsigned(state)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	return Packet{ID: 510, Payload: w.Payload()}
}

// BombStateName names one state word for the server log.
func BombStateName(state uint32) string {
	switch state {
	case BombStateReset:
		return "reset"
	case BombStatePlanting:
		return "planting"
	case BombStateCountdown:
		return "armed"
	case BombStateExploded:
		return "exploded"
	case BombStateDefusing:
		return "defusing"
	case BombStateDefuseInterrupted:
		return "defuse-interrupted"
	case BombStateDefused:
		return "defused"
	}
	return fmt.Sprintf("unknown(%d)", state)
}
