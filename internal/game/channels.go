package game

import (
	"fmt"
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

// normalizeGameSetting reproduces RoomDirectory::NormalizeGameSetting: it keeps
// only confirmed ordinary-room inputs and resolves the map, theme, time limit
// and score limit.
//
// Only the normal mode (1) is accepted. The reference implementation also lets
// the item mode (3) through, but it never implements a single item-mode rule —
// no box spawning, no pickup or use validation, and C2015/C2017 are dropped on
// purpose — so a mode-3 room would be a client-authoritative free-for-all with
// the server scoring it as an ordinary match. Gate it off rather than advertise
// a mode this server does not run.
//
// To bring it back, port the item-mode round logic first (the cn-717 dialect
// has the reference tables: map-supplies.yaml for box/pickup payloads and
// mode-control.yaml for the rules) and then accept mode 3 here again;
// world.Maps.Candidates already filters maps by their itemMode flag.
func normalizeGameSetting(request protocol.GameSetting, maps world.Maps) (protocol.GameSetting, error) {
	var setting protocol.GameSetting
	if request.Mode != 1 {
		return setting, fmt.Errorf("unsupported room mode")
	}
	switch request.SubMode {
	case 0, 1, 3, 4:
	default:
		return setting, fmt.Errorf("unsupported room sub mode")
	}
	if len(request.Strings16[0]) > protocol.SessionStringLimit {
		return setting, fmt.Errorf("room title exceeds the supported length")
	}

	resource := request.Strings16[1]
	if len(resource) == 0 {
		resource = request.Strings16[3]
	}
	resourceName := protocol.FromU16(resource)
	random := resourceName == "" || protocol.FromU16(request.Strings16[3]) == "Random" || resourceName == "Random"
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
		if third := protocol.FromU16(request.Strings16[3]); third != "" && third != "Random" {
			return setting, fmt.Errorf("random rooms cannot pin a resource")
		}
		chosen = candidates[0]
		found = true
	} else {
		entry, ok := maps.Find(resourceName)
		if !ok {
			return setting, fmt.Errorf("unknown map resource")
		}
		if !entry.NormalMode {
			return setting, fmt.Errorf("map does not support the normal mode")
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
	if limit < 60000 || limit > 1800000 || limit%1000 != 0 || score > 200 {
		return setting, fmt.Errorf("room rules are outside the supported range")
	}

	setting.Mode = request.Mode
	setting.SubMode = request.SubMode
	setting.Flag4 = request.Flag4
	setting.Flag5 = request.Flag5
	setting.Flag444 = request.Flag444
	if random {
		setting.Strings16 = [4][]uint16{
			protocol.U16String("Random"), {}, protocol.U16String(theme), protocol.U16String("Random"),
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
