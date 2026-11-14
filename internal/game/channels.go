package game

import (
	"fmt"
	"strings"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// channelTransferTTL bounds how long a prepared channel transfer can be
// consumed. The original uses 15 seconds.
const channelTransferTTL = 15 * time.Second

// lobbyChannelType is the only channel type the original lobby toolbar selects.
const lobbyChannelType = 60

// lobbyChannelCount is the number of lobby channels. The client's toolbar
// still offers two, but this server runs a single lobby: C12/C13 is answered
// with a failure so the client shows its own "cannot enter" prompt instead of
// reconnecting into a channel that would partition players and rooms.
const lobbyChannelCount = 1

// ChannelContains reports whether a channel pair is selectable.
func ChannelContains(channelType, channelIndex uint32) bool {
	return channelType == lobbyChannelType && channelIndex < lobbyChannelCount
}

// DefaultChannel is the only channel this server runs. Every session lives in
// it; a client that asks for another one is refused rather than moved.
const (
	DefaultChannelType  = lobbyChannelType
	DefaultChannelIndex = 0
)

// ChannelTransfer carries the identity that survives a channel change.
type ChannelTransfer struct {
	AccountID     uint32
	CharacterID   uint32
	SourceSession uint64
	ChannelType   uint32
	ChannelIndex  uint32
	PeerAddress   string
	Character     store.Character
}

type pendingTransfer struct {
	transfer  ChannelTransfer
	peer      string
	expiresAt time.Time
}

// ChannelDirectory holds pending channel switches.
type ChannelDirectory struct {
	pending map[uint32]pendingTransfer
	now     func() time.Time
}

// NewChannelDirectory builds the directory.
func NewChannelDirectory(now func() time.Time) *ChannelDirectory {
	if now == nil {
		now = time.Now
	}
	return &ChannelDirectory{pending: map[uint32]pendingTransfer{}, now: now}
}

func (d *ChannelDirectory) expire(now time.Time) {
	for account, entry := range d.pending {
		if !entry.expiresAt.After(now) {
			delete(d.pending, account)
		}
	}
}

// Prepare stores a transfer for the account.
func (d *ChannelDirectory) Prepare(transfer ChannelTransfer) bool {
	now := d.now()
	d.expire(now)
	if transfer.AccountID == 0 || transfer.CharacterID == 0 ||
		transfer.SourceSession == 0 || transfer.PeerAddress == "" ||
		!ChannelContains(transfer.ChannelType, transfer.ChannelIndex) {
		return false
	}
	if _, exists := d.pending[transfer.AccountID]; exists {
		return false
	}
	d.pending[transfer.AccountID] = pendingTransfer{
		transfer:  transfer,
		peer:      transfer.PeerAddress,
		expiresAt: now.Add(channelTransferTTL),
	}
	return true
}

// Consume redeems a transfer for the account from the same peer.
func (d *ChannelDirectory) Consume(accountID uint32, peerAddress string, destinationSession uint64) (ChannelTransfer, bool) {
	now := d.now()
	d.expire(now)
	entry, exists := d.pending[accountID]
	if !exists || entry.peer != peerAddress || destinationSession == 0 ||
		destinationSession == entry.transfer.SourceSession {
		return ChannelTransfer{}, false
	}
	delete(d.pending, accountID)
	return entry.transfer, true
}

// HasPending reports whether the session has a prepared transfer.
func (d *ChannelDirectory) HasPending(sessionID uint64) bool {
	d.expire(d.now())
	for _, entry := range d.pending {
		if entry.transfer.SourceSession == sessionID {
			return true
		}
	}
	return false
}

// Cancel drops any transfer owned by the session or its account.
func (d *ChannelDirectory) Cancel(sessionID uint64, accountID uint32) {
	for account, entry := range d.pending {
		if entry.transfer.SourceSession == sessionID || account == accountID {
			delete(d.pending, account)
		}
	}
}

// sniperModeID is the client's sniper battle (wgtSniper). The round is the
// ordinary competitive one -- two teams, the create request's clock and score
// limit, no boxes -- and the client restricts the weapon set on its own, so the
// server has no rule of its own for the mode.
const sniperModeID = 12

// captainModeID is the client's Captain battle (wgtCaptain). The round is the
// ordinary competitive one -- two teams, the create request's clock and score
// limit -- and the server's only rules are the Captain appointment and the
// Captain's ULTRA confirmation (see internal/game/captain.go). Everything else,
// the Captain's extra health and its gauge included, is the client's.
const captainModeID = 20

// pirateModeID is the client's pirate battle (wgtHorde): one to four players
// against the waves the room master's own client spawns. The server admits the
// room, runs its map pool, repeats the host's wave and NPC reports to the seats
// and settles the round -- the waves, the AI and the spawn points stay in the
// client (see internal/game/pirate.go).
const pirateModeID = 101

// The two hot-blood team battles: teamSurvivalModeID is wgtTeamSurvival and
// sniperTeamSurvivalModeID is wgtSniperTeamSurvival. Both are the competitive
// round with the bubble-and-rescue stage removed, so a hit that takes the last
// life kills outright: the victim reports the knockdown with the direct-death
// flag and BattleLife credits the attacker's pang from it (see
// internal/game/life.go). That is the whole of the mode on the server; the
// weapons and the respawn wait stay the client's.
const (
	teamSurvivalModeID       = 14
	sniperTeamSurvivalModeID = 18
)

// singlePvpModeID is the individual hot-blood battle (wgtSinglePvp). It is the
// same round again, but has no teams at all: every seat is its own side, the
// spawn point is picked across the whole map on each respawn, and the round ends
// as a draw -- the retail client ranks the seats itself from the score rows.
const singlePvpModeID = 13

// normalizeGameSetting reproduces RoomDirectory::NormalizeGameSetting: it keeps
// only confirmed ordinary-room inputs and resolves the map, theme, time limit
// and score limit.
//
// The normal mode (1), the item battle (3), the sniper battle (12), the two
// hot-blood battles (13 individual, 14/18 team), the zombie battle (8) and the
// Captain battle (20) are accepted. The
// 2015 client owns the item mode's rules; the server owns the box kinds the
// build offers, the box ownership it reports and the refresh of the host's map
// points (see internal/game/supplies.go). The sniper battle is the ordinary
// competitive round with a different weapon set, which the client restricts
// itself: nothing here changes for it beyond admitting the mode. The hot-blood
// team battles (14, 18) are the same round without the bubble stage, counted
// from the direct-death knockdown report. The zombie battle keeps no rules in
// the client at all: the server picks the king, converts the dead and decides
// the round (see internal/game/zombie.go). The Captain battle is the ordinary
// competitive round with a Captain per team, named by the server and handed on
// when it dies (see internal/game/captain.go). world.Maps.Candidates already
// filters maps by their itemMode/normalMode flags.
func normalizeGameSetting(request protocol.GameSetting, maps world.Maps) (protocol.GameSetting, error) {
	var setting protocol.GameSetting
	if request.Mode != 1 && request.Mode != itemModeID && request.Mode != sniperModeID &&
		request.Mode != singlePvpModeID && request.Mode != teamSurvivalModeID &&
		request.Mode != sniperTeamSurvivalModeID && request.Mode != zombieModeID &&
		request.Mode != captainModeID && request.Mode != bombModeID &&
		request.Mode != pirateModeID && request.Mode != pveBossModeID {
		return setting, fmt.Errorf("unsupported room mode")
	}
	switch request.SubMode {
	case 0, 1, 3, 4:
	default:
		// The zombie and bomb battles carry no rules in the client, so a sub mode
		// they send cannot change the round; it is echoed back instead of costing
		// the room its create. Any other mode keeps the audited values only.
		if (request.Mode != zombieModeID && request.Mode != bombModeID &&
			request.Mode != pirateModeID && request.Mode != pveBossModeID) || request.SubMode > 15 {
			return setting, fmt.Errorf("unsupported room sub mode")
		}
	}
	if len(request.Strings16[0]) > protocol.SessionStringLimit {
		return setting, fmt.Errorf("room title exceeds the supported length")
	}

	resource := request.Strings16[1]
	if len(resource) == 0 {
		resource = request.Strings16[3]
	}
	resourceName := protocol.FromU16(resource)
	// Every map entry the client does not name a map with is a Random-family
	// marker: Random is the all-random entry, Random_Hot the popular pool and
	// Random_Hot_DifN a difficulty pool. Reading the prefix keeps a mode's own
	// marker (the pirate battle's Random_Hot_Dif1) from being taken for a map.
	random := resourceName == "" || strings.HasPrefix(resourceName, "Random") ||
		strings.HasPrefix(protocol.FromU16(request.Strings16[3]), "Random")
	theme := protocol.FromU16(request.Strings16[2])
	if theme == "" {
		theme = "All"
	}

	// A pirate pick arrives the way the client's own change request writes it:
	// {display name, scene, theme, mission key}. The pool is keyed by the mission key
	// in the fourth slot, which is also the key the client resolves its stage script
	// from; the second slot is what the stage entry loads and what it reports back.
	if request.Mode == pirateModeID && !random {
		if third := protocol.FromU16(request.Strings16[3]); third != "" {
			resourceName = third
		}
	}

	var chosen world.Map
	found := false
	if random {
		candidates := maps.Candidates(request.Mode, theme)
		if request.Mode == pirateModeID {
			// The client's pirate entry offers a difficulty pool on top of the theme:
			// "Random" enters every npc_* row of the theme and Random_Hot_DifN the
			// three rows of that difficulty.
			candidates = maps.HordeCandidates(theme, pirateDifficulty(protocol.FromU16(request.Strings16[3])))
		}
		if len(candidates) == 0 {
			return setting, fmt.Errorf("no map candidates for the requested theme")
		}
		if third := protocol.FromU16(request.Strings16[3]); third != "" && !strings.HasPrefix(third, "Random") {
			return setting, fmt.Errorf("random rooms cannot pin a resource")
		}
		chosen = candidates[0]
		found = true
	} else {
		entry, ok := maps.Find(resourceName)
		if !ok {
			return setting, fmt.Errorf("unknown map resource")
		}
		switch request.Mode {
		case pirateModeID:
			// The pirate battle has its own 33 npc_* rows; the create dialog starts on
			// the startup map, so a key outside the pool is replaced instead of
			// refusing the room (the bomb mission does the same).
			if !maps.IsHordeMap(entry.ResourceName) {
				candidates := maps.HordeCandidates(theme, 0)
				if len(candidates) == 0 {
					return setting, fmt.Errorf("no pirate map candidates for the requested theme")
				}
				entry = candidates[0]
			}
		case bombModeID:
			// The bomb mission's own pool is the authority: two of its six maps
			// carry neither the normal nor the item flag in MapMeta. The client's
			// create dialog starts on the startup map, which is an ordinary
			// competitive one, so a map outside the pool is replaced with one the
			// mode can actually run instead of refusing the room outright.
			if !maps.IsBombMap(entry.ResourceName) {
				candidates := maps.Candidates(bombModeID, theme)
				if len(candidates) == 0 {
					return setting, fmt.Errorf("no bomb map candidates for the requested theme")
				}
				entry = candidates[0]
			}
		case itemModeID:
			if !entry.ItemMode {
				return setting, fmt.Errorf("map does not support the item mode")
			}
		case pveBossModeID:
			// The raid's own seven rows carry no competitive flag, and its create dialog
			// starts on the startup map; a key outside the pool is replaced the way the
			// pirate room replaces one.
			if !maps.IsPveMap(entry.ResourceName) {
				candidates := maps.PveCandidates(theme)
				if len(candidates) == 0 {
					return setting, fmt.Errorf("no raid map candidates for the requested theme")
				}
				entry = candidates[0]
			}
		default:
			// Modes 8/12/13 also list one map that carries neither competitive flag
			// (the client's isZombieModeMap / isSniperModeMap / isSinglePvpModeMap
			// tables are 58 against the catalog's 57), so the ordinary-map test is
			// relaxed for exactly those rows.
			if !entry.NormalMode && !maps.HasModeMap(request.Mode, entry.ResourceName) {
				return setting, fmt.Errorf("map does not support the normal mode")
			}
		}
		// A pirate room states the scene and the mission in different slots on purpose.
		if request.Mode != pirateModeID {
			if first, third := protocol.FromU16(request.Strings16[1]), protocol.FromU16(request.Strings16[3]); first != "" && third != "" && first != third {
				return setting, fmt.Errorf("map resource mismatch")
			}
		}
		chosen = entry
		found = true
	}
	if !found {
		return setting, fmt.Errorf("map could not be resolved")
	}

	limit := request.Values128[2]
	if limit == 0 {
		limit = chosen.TimeLimitMilliseconds
	}
	score := request.Values128[0]
	if score == 0 {
		score = chosen.ScoreLimit
	}
	// The zombie battle always runs the mode's own length: the client forces 180 s
	// into the rule record at create time and keeps no rule of its own.
	if request.Mode == zombieModeID {
		limit = zombieRoundLimitMs
	}
	// The pirate battle runs the 300 s its MapMeta row states, and its round is
	// decided by the waves rather than by a kill limit.
	if request.Mode == pirateModeID {
		limit = chosen.TimeLimitMilliseconds
		score = 0
	}
	// The raid runs its maps' TimeAttack limit (600 s) and no kill limit: the round
	// is decided by the stage word, never by a pang count.
	if request.Mode == pveBossModeID {
		limit = chosen.TimeLimitMilliseconds
		score = 0
	}
	if limit < 60000 || limit > 1800000 || limit%1000 != 0 || score > 200 {
		return setting, fmt.Errorf("room rules are outside the supported range")
	}

	setting.Mode = request.Mode
	setting.SubMode = request.SubMode
	setting.Flag4 = request.Flag4
	setting.Flag5 = request.Flag5
	setting.Flag444 = request.Flag444
	if request.Mode == pirateModeID && !random {
		// The client's own pirate change request states, in wire order (packets/room.yaml
		// GameSetting offsets 16/44/72/100): the localized map name, the map table's
		// Directory (the scene), the theme, and the map key the player chose (npc_*).
		// current_map is the string gspRoomEnterGame overwrites at match start, and it is
		// what feeds the NPC Lua map name, so the scene belongs there; the mission key stays
		// in the fourth slot -- the author's "有一个要发 npc_xxxx, 要跟它的 lua 对应".
		display := protocol.FromU16(request.Strings16[0])
		if display == "" || maps.IsHordeMap(display) {
			display = chosen.ResourceName
		}
		scene := world.HordeStageKey(chosen.ResourceName)
		if scene == "" {
			scene = protocol.FromU16(request.Strings16[1])
		}
		setting.Strings16 = [4][]uint16{
			protocol.U16String(display),
			protocol.U16String(scene),
			protocol.U16String(theme),
			protocol.U16String(chosen.ResourceName),
		}
	} else if random {
		// The marker the client sent is kept in the fourth slot: every round reads
		// it to tell "never chosen" from a pinned name, while the display slot keeps
		// the all-random entry the room card draws. A pirate room keeps the marker in
		// all three map strings, which is the shape its own create writes ("a mode-101
		// create carries Random_Hot_Dif1 in the three map strings and All as the
		// theme", packets/builds.yaml); the round then replaces them with the map it drew.
		marker := protocol.FromU16(request.Strings16[3])
		if !strings.HasPrefix(marker, "Random") {
			marker = "Random"
		}
		if request.Mode == pirateModeID {
			if marker == "Random" {
				marker = "Random_Hot_Dif1"
			}
			setting.Strings16 = [4][]uint16{
				protocol.U16String(marker),
				protocol.U16String(marker),
				protocol.U16String(theme),
				protocol.U16String(marker),
			}
		} else {
			setting.Strings16 = [4][]uint16{
				protocol.U16String("Random"), {}, protocol.U16String(theme), protocol.U16String(marker),
			}
		}
	} else {
		label := protocol.FromU16(request.Strings16[0])
		if label == "" {
			label = chosen.ResourceName
		}
		// The theme slot is what the client's map dialog opens on. The bomb mission
		// keeps the requested theme (normally "All") instead of the chosen map's own:
		// rewriting it to, say, Village left the dialog on that tab and the host saw
		// three maps until the "all" tab refreshed the six.
		themeSlot := chosen.Theme
		if request.Mode == bombModeID || request.Mode == pirateModeID {
			themeSlot = theme
		}
		setting.Strings16 = [4][]uint16{
			protocol.U16String(label),
			protocol.U16String(chosen.ResourceName),
			protocol.U16String(themeSlot),
			protocol.U16String(chosen.ResourceName),
		}
	}
	setting.Values128[0] = score
	setting.Values128[2] = limit
	if request.Mode == pirateModeID {
		// Values128 is the reference server's own block for this map: the kill limit (20,
		// the map's DeathMatchKillLimit) followed by four time limits of 240000. This
		// server wrote a zero kill limit and a single 300000 word instead.
		setting.Values128 = [5]uint32{20, pirateBlockLimitMS, pirateBlockLimitMS,
			pirateBlockLimitMS, pirateBlockLimitMS}
		// GameSetting +164: the Lua difficulty cn-717 sub_AE8A60 copies into its HordeModeMgr.
		// The client uses it twice: as the level of the map's WaveTest table (TableWave2Level)
		// and as the mobs' HP rate. It follows the mission the player picked (npc_*_01/02/03
		// are difficulties 1/2/3), so a solo round on a 01 mission plays at difficulty 1.
		difficulty := chosen.Difficulty
		if difficulty == 0 {
			difficulty = 1
		}
		setting.Values148[4] = difficulty
		// GameSetting +168: the reference's own default for this word is -1 (packets/room.yaml,
		// unknown_168). A zero here reached the pirate wave script as a zero time base, and every
		// scheduled spawn group of the map's lua then counted as overdue: all 26 mobs of wave 1
		// appeared inside one second, the wave wrote them off and the round ended five seconds in.
		setting.Values148[5] = pirateSettingWord168
		// The reference's own block carries the client's all-random marker in the first
		// three trailing strings ("Random" x3). This server wrote them empty, which is
		// the last byte-level difference between the two blocks.
		for i := 0; i < 3; i++ {
			setting.Strings180[i] = protocol.U16String("Random")
		}
	}
	return setting, nil
}

// pirateDifficulty reads the difficulty out of the client's pirate random marker:
// Random_Hot_Dif1..Dif3 pick that difficulty, everything else keeps all three.
func pirateDifficulty(marker string) uint32 {
	const prefix = "Random_Hot_Dif"
	if !strings.HasPrefix(marker, prefix) {
		return 0
	}
	digits := marker[len(prefix):]
	if digits == "" {
		return 0
	}
	value := uint32(0)
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return 0
		}
		value = value*10 + uint32(digit-'0')
	}
	if value > 3 {
		return 0
	}
	return value
}
