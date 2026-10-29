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
func CreateGameResultReply(teamScores [2]uint16, players []GameResultPlayer) (Packet, error) {
	if teamScores[0] > 200 || teamScores[1] > 200 || len(players) == 0 || len(players) > 8 {
		return Packet{}, fmt.Errorf("%w: game result capacity", ErrInvalidPacket)
	}
	// The client's ScoreData record has sixteen slots; only eight can be
	// occupied, the rest must still be written as empty rows or the client reads
	// past the payload and crashes on the result screen.
	slots := make([]*GameResultPlayer, 16)
	for index := range players {
		player := &players[index]
		if player.Slot >= 8 || slots[player.Slot] != nil || player.AccountID == 0 ||
			(player.Team != 1 && player.Team != 2) ||
			CheckNickname(player.Nickname) != NicknameOK ||
			player.Level < 1 || player.Level > 200 ||
			player.LevelStart > player.Experience ||
			player.Experience > player.NextLevel ||
			player.AwardScore > 0x7FFFFFFF ||
			(player.AwardGrade > 8 && player.AwardGrade != Unset) ||
			(player.AwardMedal != 0 && (player.AwardMedal < 4 || player.AwardMedal > 9)) {
			return Packet{}, fmt.Errorf("%w: game result identity", ErrInvalidPacket)
		}
		slots[player.Slot] = player
	}

	w := NewWriter()
	w.Unsigned(17) // team table capacity
	winner := uint32(0)
	if teamScores[0] != teamScores[1] {
		if teamScores[0] > teamScores[1] {
			winner = 1
		} else {
			winner = 2
		}
	}
	w.Unsigned(winner)
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
