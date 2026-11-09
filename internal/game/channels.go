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
		request.Mode != captainModeID && request.Mode != bombModeID {
		return setting, fmt.Errorf("unsupported room mode")
	}
	switch request.SubMode {
	case 0, 1, 3, 4:
	default:
		// The zombie and bomb battles carry no rules in the client, so a sub mode
		// they send cannot change the round; it is echoed back instead of costing
		// the room its create. Any other mode keeps the audited values only.
		if (request.Mode != zombieModeID && request.Mode != bombModeID) || request.SubMode > 15 {
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

	var chosen world.Map
	found := false
	if random {
		candidates := maps.Candidates(request.Mode, theme)
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
		case bombModeID:
			// The bomb mission's own pool is the authority: two of its six maps
			// carry neither the normal nor the item flag in MapMeta, and refusing
			// them would leave a room that can only be built at random.
		case itemModeID:
			if !entry.ItemMode {
				return setting, fmt.Errorf("map does not support the item mode")
			}
		default:
			if !entry.NormalMode {
				return setting, fmt.Errorf("map does not support the normal mode")
			}
		}
		if first, third := protocol.FromU16(request.Strings16[1]), protocol.FromU16(request.Strings16[3]); first != "" && third != "" && first != third {
			return setting, fmt.Errorf("map resource mismatch")
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
	if limit < 60000 || limit > 1800000 || limit%1000 != 0 || score > 200 {
		return setting, fmt.Errorf("room rules are outside the supported range")
	}

	setting.Mode = request.Mode
	setting.SubMode = request.SubMode
	setting.Flag4 = request.Flag4
	setting.Flag5 = request.Flag5
	setting.Flag444 = request.Flag444
	if random {
		// The marker the client sent is kept in the fourth slot: every round reads
		// it to tell "never chosen" from a pinned name, while the display slot keeps
		// the all-random entry the room card draws.
		marker := protocol.FromU16(request.Strings16[3])
		if !strings.HasPrefix(marker, "Random") {
			marker = "Random"
		}
		setting.Strings16 = [4][]uint16{
			protocol.U16String("Random"), {}, protocol.U16String(theme), protocol.U16String(marker),
		}
	} else {
		label := protocol.FromU16(request.Strings16[0])
		if label == "" {
			label = chosen.ResourceName
		}
		setting.Strings16 = [4][]uint16{
			protocol.U16String(label),
			protocol.U16String(chosen.ResourceName),
			protocol.U16String(chosen.Theme),
			protocol.U16String(chosen.ResourceName),
		}
	}
	setting.Values128[0] = score
	setting.Values128[2] = limit
	return setting, nil
}
