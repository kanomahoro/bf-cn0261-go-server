package game

import (
	"fmt"
	"math/rand"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// defenseSeedCount is the number of sampled defense values per match.
const defenseSeedCount = 1000

// cancelSeedWords is the number of 32-bit words in the critical-cancel bitmap.
const cancelSeedWords = 32

// defenseProfile mirrors DefenseProfile: the sampled mitigation range plus the
// critical-cancel probability.
type defenseProfile struct {
	Minimum             float64
	Maximum             float64
	CriticalCancelRatio float64
	BackReduction       float64
	WallReduction       float64
}

// evaluateDefense reproduces GameDefenseCatalog::Evaluate: additive minimum and
// maximum, assignment for the critical-cancel ratio, additive reductions.
func evaluateDefense(catalog world.Defense, items []store.Item) (defenseProfile, error) {
	var profile defenseProfile
	seen := map[uint32]bool{}
	for _, item := range items {
		if item.Quantity == 0 || item.EquippedSlot == 0 || seen[item.ItemID] {
			continue
		}
		seen[item.ItemID] = true
		for _, entry := range catalog.Items {
			if entry.ItemID != item.ItemID {
				continue
			}
			for _, modifier := range entry.Modifiers {
				switch modifier.Attribute {
				case "minimum":
					profile.Minimum += modifier.Ratio
				case "maximum":
					profile.Maximum += modifier.Ratio
				case "criticalCancelRatio":
					profile.CriticalCancelRatio = modifier.Ratio
				case "backReduction":
					profile.BackReduction += modifier.Ratio
				case "wallReduction":
					profile.WallReduction += modifier.Ratio
				}
			}
		}
	}
	if profile.Minimum > profile.Maximum || profile.Maximum > 1 {
		return profile, fmt.Errorf("equipped defense range is inconsistent")
	}
	return profile, nil
}

// defenseSeeds is the per-member seed table pair.
type defenseSeeds struct {
	Defense []byte
	Cancel  []uint32
}

// generateDefenseSeeds samples the mitigation table and the cancel bitmap.
func generateDefenseSeeds(profile defenseProfile) (defenseSeeds, error) {
	if profile.Minimum < 0 || profile.Minimum > profile.Maximum ||
		profile.Maximum > 1 || profile.CriticalCancelRatio < 0 ||
		profile.CriticalCancelRatio > 1 {
		return defenseSeeds{}, fmt.Errorf("defense profile is out of range")
	}
	minimum := int(profile.Minimum * 100)
	maximum := int(profile.Maximum * 100)
	if maximum <= minimum {
		maximum = minimum + 1
	}
	defense := make([]byte, defenseSeedCount)
	cancel := make([]uint32, cancelSeedWords)
	for index := 0; index < defenseSeedCount; index++ {
		value := minimum + rand.Intn(maximum-minimum)
		defense[index] = byte(value)
		if rand.Float64() < profile.CriticalCancelRatio {
			cancel[index/32] |= 1 << (index % 32)
		}
	}
	return defenseSeeds{Defense: defense, Cancel: cancel}, nil
}

// sendDefenseSeeds publishes every member's tables to every recipient, matching
// SynchronizeDefenseSeeds (S5/S6 per member, then the two empty markers).
func (s *Server) sendDefenseSeeds(room *Room) {
	type entry struct {
		objectID uint32
		seeds    defenseSeeds
	}
	room.mu.Lock()
	entries := make([]entry, 0, 8)
	for slot := range room.slots {
		if room.slots[slot].session == nil {
			continue
		}
		seeds, ok := room.defenseSeeds[slot]
		if !ok {
			continue
		}
		entries = append(entries, entry{objectID: objectFor(slot), seeds: seeds})
	}
	room.mu.Unlock()
	if len(entries) == 0 {
		return
	}
	for _, member := range room.View() {
		if !member.Occupied {
			continue
		}
		for _, value := range entries {
			// Both tables must reach the member: the cancel bitmap is not optional.
			if table, err := protocol.CreateDefenseSeedReply(value.objectID, value.seeds.Defense); err == nil {
				if s.relay == nil || !s.relay.SendToSlot(room.ID, uint8(member.Slot), table) {
					_ = member.Session.Send(table)
				}
			}
			if table, err := protocol.CreateCancelSeedReply(value.objectID, value.seeds.Cancel); err == nil {
				if s.relay == nil || !s.relay.SendToSlot(room.ID, uint8(member.Slot), table) {
					_ = member.Session.Send(table)
				}
			}
		}
		// The markers belong to the same ordered relay stream as the tables. On
		// the game connection S11 means "channel move reply", so sending it there
		// makes the client abort with a channel error.
		for _, marker := range []uint16{11, 12} {
			packet := protocol.Packet{ID: marker}
			if s.relay == nil || !s.relay.SendToSlot(room.ID, uint8(member.Slot), packet) {
				_ = member.Session.Send(packet)
			}
		}
	}
}

// prepareDefenseSeeds evaluates every member before the tables are published.
func (s *Server) prepareDefenseSeeds(room *Room) {
	room.mu.Lock()
	room.defenseSeeds = map[int]defenseSeeds{}
	members := make([]struct {
		slot  int
		items []store.Item
	}, 0, 8)
	for slot := range room.slots {
		if room.slots[slot].session != nil {
			members = append(members, struct {
				slot  int
				items []store.Item
			}{slot: slot, items: room.slots[slot].items})
		}
	}
	room.mu.Unlock()
	for _, member := range members {
		profile, err := evaluateDefense(s.defense, member.items)
		if err != nil {
			s.log.Debug("defense evaluation skipped", "slot", member.slot, "error", err)
			continue
		}
		seeds, err := generateDefenseSeeds(profile)
		if err != nil {
			continue
		}
		room.mu.Lock()
		room.defenseSeeds[member.slot] = seeds
		room.mu.Unlock()
	}
}

// handleDefenseSeedReport validates C20/C21 against the installed tables.
func (s *Server) handleDefenseSeedReport(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil || session.slot < 0 {
		return nil
	}
	room.mu.RLock()
	seeds, ok := room.defenseSeeds[session.slot]
	room.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: defense seeds were not requested", protocol.ErrInvalidPacket)
	}
	if err := protocol.ValidateDefenseSeedReport(packet, seeds.Defense, seeds.Cancel); err != nil {
		return err
	}
	room.mu.Lock()
	if packet.ID == 20 {
		room.defenseConfirmed[session.slot] = true
	} else {
		room.cancelConfirmed[session.slot] = true
	}
	room.mu.Unlock()
	return nil
}
