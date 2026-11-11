package protocol

import "fmt"

// GameResultPlayer is one row of the S729 result screen.
type GameResultPlayer struct {
	Slot           uint8
	AccountID      uint32
	Nickname       []uint16
	Team           uint8
	Bubbles        uint16
	Pangs          uint16
	Rescues        uint16
	Level          uint32
	LevelStart     uint32
	Experience     uint32
	NextLevel      uint32
	ExperienceGain uint32
	LucciGain      uint32
	LevelUp        bool
	AwardScore     uint32
	AwardGrade     uint32
	AwardMedal     uint32
}

// CreateGameResultReply builds S729 (ScoreData team table plus per-slot rows).
//
// winner names the winning team the header carries: 0 asks it to be derived from
// the two team totals, and any other value is written as given. The zombie battle
// (team 14) and the cooperative families need that.
func CreateGameResultReply(teamScores [2]uint16, winner uint8, players []GameResultPlayer) (Packet, error) {
	if teamScores[0] > 200 || teamScores[1] > 200 {
		return Packet{}, fmt.Errorf("%w: game result capacity", ErrInvalidPacket)
	}
	if winner > 17 {
		return Packet{}, fmt.Errorf("%w: game result winner", ErrInvalidPacket)
	}
	slots, err := resultSlots(players)
	if err != nil {
		return Packet{}, err
	}

	w := NewWriter()
	writeScoreData(w, teamScores, winner, slots)
	for _, player := range slots {
		if player == nil {
			w.Unsigned(0)
			continue
		}
		w.Unsigned(player.AwardMedal)
	}
	for range slots {
		w.Unsigned(Unset)
	}
	return Packet{ID: 729, Payload: w.Payload()}, nil
}

// resultSlots validates the result rows and places them in the client's
// sixteen-slot table. The client's ScoreData record has sixteen slots; only eight
// can be occupied, the rest must still be written as empty rows or the client
// reads past the payload and crashes on the result screen.
func resultSlots(players []GameResultPlayer) ([]*GameResultPlayer, error) {
	if len(players) == 0 || len(players) > PlayerSlots {
		return nil, fmt.Errorf("%w: game result capacity", ErrInvalidPacket)
	}
	slots := make([]*GameResultPlayer, 16)
	for index := range players {
		player := &players[index]
		if player.Slot >= PlayerSlots || slots[player.Slot] != nil || player.AccountID == 0 ||
			!validResultTeam(player.Team) ||
			CheckNickname(player.Nickname) != NicknameOK ||
			player.Level < 1 || player.Level > 200 ||
			player.LevelStart > player.Experience ||
			player.Experience > player.NextLevel ||
			player.AwardScore > 0x7FFFFFFF ||
			(player.AwardGrade > 8 && player.AwardGrade != Unset) ||
			(player.AwardMedal != 0 && (player.AwardMedal < 4 || player.AwardMedal > 9)) {
			return nil, fmt.Errorf("%w: game result identity", ErrInvalidPacket)
		}
		slots[player.Slot] = player
	}
	return slots, nil
}

// writeScoreData writes the bare ScoreData block: the team table the client reads
// with its sixteen rows. A caller that needs the result screen's own reward tail
// appends it afterwards.
func writeScoreData(w *Writer, teamScores [2]uint16, winner uint8, slots []*GameResultPlayer) {
	w.Unsigned(17) // team table capacity
	if winner == 0 && teamScores[0] != teamScores[1] {
		if teamScores[0] > teamScores[1] {
			winner = 1
		} else {
			winner = 2
		}
	}
	w.Unsigned(uint32(winner))
	w.Unsigned(0)
	for team := 1; team <= 17; team++ {
		w.Unsigned(0)
		if team <= 2 {
			w.Unsigned(uint32(teamScores[team-1]))
		} else {
			w.Unsigned(0)
		}
		w.Unsigned(0)
		w.Unsigned(0)
	}
	for _, player := range slots {
		writeResultPlayer(w, player)
	}
}

func writeResultPlayer(w *Writer, player *GameResultPlayer) {
	unsignedOrZero := func(value uint32) {
		if player == nil {
			w.Unsigned(0)
			return
		}
		w.Unsigned(value)
	}
	if player == nil {
		w.Unsigned(Unset)
		w.Unsigned(0)
		w.String(nil)
	} else {
		w.Unsigned(uint32(player.Slot))
		w.Unsigned(player.AccountID)
		w.String(player.Nickname)
	}
	unsignedOrZero(uint32(teamOf(player)))
	unsignedOrZero(levelOf(player))
	unsignedOrZero(bubblesOf(player))
	unsignedOrZero(pangsOf(player))
	w.Unsigned(0)
	unsignedOrZero(rescuesOf(player))
	for offset := 60; offset <= 96; offset += 4 {
		if player != nil && offset == 64 {
			w.Unsigned(player.AwardScore)
			continue
		}
		w.Unsigned(0)
	}
	for offset := 112; offset <= 140; offset += 4 {
		w.Unsigned(0)
	}
	unsignedOrZero(levelStartOf(player))
	unsignedOrZero(experienceOf(player))
	unsignedOrZero(nextLevelOf(player))
	unsignedOrZero(experienceGainOf(player))
	unsignedOrZero(lucciGainOf(player))
	if player != nil {
		w.Unsigned(1)
	} else {
		w.Unsigned(0)
	}
	if player != nil && player.LevelUp {
		w.Unsigned(1)
	} else {
		w.Unsigned(0)
	}
	for index := 0; index < 4; index++ {
		w.Unsigned(0)
	}
	w.Byte(0)
	w.Unsigned(0)
	w.Unsigned(0)
	w.String(nil)
	for index := 0; index < 6; index++ {
		w.Unsigned(0)
	}
	w.Unsigned(0)
	w.Unsigned(0)
	if player != nil {
		w.Unsigned(player.AwardGrade)
	} else {
		w.Unsigned(Unset)
	}
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	for offset := 340; offset <= 360; offset += 4 {
		w.Unsigned(0)
	}
	w.Short(0)
	w.Unsigned(0)
}

// validResultTeam reports whether a result row may carry this team id: the two
// battle teams, the zombie battle's team (14) and the cooperative zombies' (15).
func validResultTeam(team uint8) bool {
	switch team {
	// Team 0 is the client's "no team" value: the tutorial's live score update
	// carries it. Every played round names one of the four below.
	case 0, 1, 2, 14, 15:
		return true
	default:
		return false
	}
}

func teamOf(player *GameResultPlayer) uint8 {
	if player == nil {
		return 0
	}
	return player.Team
}

func levelOf(player *GameResultPlayer) uint32 {
	if player == nil {
		return 0
	}
	return player.Level
}

func levelStartOf(player *GameResultPlayer) uint32 {
	if player == nil {
		return 0
	}
	return player.LevelStart
}

func experienceOf(player *GameResultPlayer) uint32 {
	if player == nil {
		return 0
	}
	return player.Experience
}

func nextLevelOf(player *GameResultPlayer) uint32 {
	if player == nil {
		return 0
	}
	return player.NextLevel
}

func experienceGainOf(player *GameResultPlayer) uint32 {
	if player == nil {
		return 0
	}
	return player.ExperienceGain
}

func lucciGainOf(player *GameResultPlayer) uint32 {
	if player == nil {
		return 0
	}
	return player.LucciGain
}

func bubblesOf(player *GameResultPlayer) uint32 {
	if player == nil {
		return 0
	}
	return uint32(player.Bubbles)
}

func pangsOf(player *GameResultPlayer) uint32 {
	if player == nil {
		return 0
	}
	return uint32(player.Pangs)
}

func rescuesOf(player *GameResultPlayer) uint32 {
	if player == nil {
		return 0
	}
	return uint32(player.Rescues)
}
