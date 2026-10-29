package protocol

import "fmt"

// CharacterSlot is the equipped_slot value reserved for the character skin.
// It matches the reference server's kCharacterEquipmentSlot and the
// item_definitions row of every character item.
const CharacterSlot = 63

// InvItem is the wire view of one inventory row.
type InvItem struct {
	ItemID        uint32
	Quantity      uint32
	RemainingDays *float64
	EquippedSlot  uint8
}

// Character is the wire view of a persisted character.
type Character struct {
	AccountID     uint32
	Nickname      []uint16
	Level         uint32
	CharacterType uint32
	Type          uint32
}

// Nick returns the nickname as a Go string.
func (c Character) Nick() string { return FromU16(c.Nickname) }

// SessionMember mirrors the client's UserSlot object.
type SessionMember struct {
	Character      *Character
	Items          []InvItem
	Team           uint8
	Ready          bool
	Closed         bool
	ActivityFlags  [2]bool
	IntrusionState uint32
}

// Unset is the client's "no value" sentinel for several u32 fields.
const Unset uint32 = 0xFFFFFFFF

// WriteSessionMember serializes a UserSlot (0x41B060).
func WriteSessionMember(w *Writer, m SessionMember, ownerSlot uint32) error {
	if m.Character != nil {
		if m.Character.AccountID == 0 || m.Closed || ownerSlot >= 32 ||
			(m.Team != 1 && m.Team != 2 && m.Team != 16) {
			return fmt.Errorf("%w: invalid session member state", ErrInvalidPacket)
		}
	} else if len(m.Items) != 0 || m.Ready || m.IntrusionState != 0 {
		return fmt.Errorf("%w: empty slot has state", ErrInvalidPacket)
	}
	if m.IntrusionState > 2 {
		return fmt.Errorf("%w: intrusion state out of range", ErrInvalidPacket)
	}

	w.Unsigned(bool32(m.Character != nil)) // +4 occupied
	w.Unsigned(bool32(m.Closed))           // +5 closed
	w.Unsigned(0)                          // +6
	if m.Character != nil {
		w.Byte(uint8(ownerSlot))
	} else {
		w.Byte(0xFF)
	}
	if m.Character != nil {
		w.Unsigned(m.Character.AccountID) // +8 identity
		w.String(m.Character.Nickname)    // +12
		w.Short(1)                        // +40
	} else {
		w.Unsigned(0)
		w.String(nil)
		w.Short(0)
	}
	w.Short(0) // +42
	if m.Character != nil {
		w.Byte(m.Team) // +124 team
		w.Unsigned(m.Character.Level)
	} else {
		w.Byte(0)
		w.Unsigned(0)
	}
	w.Boolean(m.Ready)            // +125
	w.Boolean(m.ActivityFlags[0]) // +126
	w.Boolean(m.ActivityFlags[1]) // +127
	for i := 0; i < 4; i++ {
		w.Unsigned(0) // +128, +60, +64, empty vector +68
	}
	for _, v := range [7]uint32{0, 0, Unset, 0, 0, 0, Unset} {
		w.Unsigned(v) // +84, +88, +108, +114, +136, +140, +144
	}
	for i := 0; i < 5; i++ {
		w.Short(0xFFFF) // +48..56
	}
	w.Unsigned(0)     // +156
	w.Unsigned(1)     // +160
	w.Unsigned(0)     // +148
	w.Unsigned(Unset) // +176
	w.String(nil)     // +180
	w.Unsigned(0)     // +208
	w.Unsigned(0)     // +209
	w.Unsigned(0)     // +212
	w.Unsigned(1)     // +216
	w.Unsigned(0)     // +220
	for i := 0; i < 8; i++ {
		w.Unsigned(0) // social system info +228
	}
	w.Unsigned(0) // +132
	w.Unsigned(m.IntrusionState)
	for _, v := range [3]uint32{0, Unset, 0} {
		w.Unsigned(v) // +280, +284, +276
	}
	return nil
}

func bool32(v bool) uint32 {
	if v {
		return 1
	}
	return 0
}

// ResolveEquippedCharacter maps a character type plus its item list to the
// equipped character item id (0xA31510); the wire value is 300000 + id.
func ResolveEquippedCharacter(characterType uint32, items []InvItem) uint32 {
	base := characterType
	if base == 0 {
		base = 1
	}
	for _, item := range items {
		if item.EquippedSlot == CharacterSlot {
			return item.ItemID % 1000
		}
	}
	return base % 1000
}

// CreateEquipmentReply builds S805 (EquippedItemList).
func CreateEquipmentReply(identity, characterType uint32, items []InvItem) (Packet, error) {
	equipped := ResolveEquippedCharacter(characterType, items)
	w := NewWriter()
	w.Unsigned(identity)
	w.Unsigned(300000 + equipped)
	w.Unsigned(0)
	count := uint32(0)
	for _, item := range items {
		if item.EquippedSlot != 0 && item.EquippedSlot != CharacterSlot {
			count++
		}
	}
	w.Unsigned(count)
	for _, item := range items {
		if item.EquippedSlot != 0 && item.EquippedSlot != CharacterSlot {
			w.Unsigned(item.ItemID)
		}
	}
	w.Unsigned(0) // empty byte vector
	w.Unsigned(0) // empty attribute vector
	w.Unsigned(0) // empty auxiliary vector
	return Packet{ID: 805, Payload: w.Payload()}, nil
}

// CreateInventoryReply builds S801.
func CreateInventoryReply(items []InvItem) (Packet, error) {
	if len(items) > 512 {
		return Packet{}, fmt.Errorf("%w: inventory capacity exceeded", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(uint32(len(items)))
	for _, item := range items {
		days := 36500.0
		if item.RemainingDays != nil {
			days = *item.RemainingDays
		}
		w.Unsigned(item.ItemID)
		w.Unsigned(item.Quantity)
		w.Double(days)
		w.Boolean(item.EquippedSlot != 0)
		w.Unsigned(0)
		w.Unsigned(0)
		w.Unsigned(0)
		w.Unsigned(0)
		w.String(nil)
		w.Unsigned(0)
	}
	return Packet{ID: 801, Payload: w.Payload()}, nil
}

// CreateEquipmentResult builds S808/S809.
func CreateEquipmentResult(equipped, success bool) Packet {
	w := NewWriter()
	if success {
		w.Unsigned(0)
	} else if equipped {
		w.Unsigned(3)
	} else {
		w.Unsigned(1)
	}
	id := uint16(809)
	if equipped {
		id = 808
	}
	return Packet{ID: id, Payload: w.Payload()}
}

// ReadEquipmentRequest parses C803/C804.
func ReadEquipmentRequest(p Packet) (uint32, error) {
	if p.ID != 803 && p.ID != 804 {
		return 0, fmt.Errorf("%w: unexpected equip request %d", ErrInvalidPacket, p.ID)
	}
	r := NewReader(p.Payload)
	itemID, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	return itemID, r.RequireEnd()
}

// WeaponPresets is the wire view of the preset selection.
type WeaponPresets struct {
	Selected       uint8
	AvailableCount uint8
	Items          []uint32
}

// CreateWeaponPresetsReply builds S833.
func CreateWeaponPresetsReply(identity uint32, presets WeaponPresets) (Packet, error) {
	if presets.AvailableCount < 2 || presets.AvailableCount > 3 ||
		presets.Selected >= presets.AvailableCount {
		return Packet{}, fmt.Errorf("%w: invalid weapon preset state", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Byte(presets.Selected)
	w.Unsigned(identity)
	// The client always reads five definition ids; missing slots must be zero
	// rather than omitted, otherwise every following field desyncs.
	w.Unsigned(5)
	for i := 0; i < 5; i++ {
		if i < len(presets.Items) {
			w.Unsigned(presets.Items[i])
		} else {
			w.Unsigned(0)
		}
	}
	w.Unsigned(5)
	for i := uint8(0); i < 5; i++ {
		if i < presets.AvailableCount {
			w.Byte(1)
		} else {
			w.Byte(0)
		}
	}
	w.Unsigned(5)
	w.Bytes(make([]byte, 5))
	return Packet{ID: 833, Payload: w.Payload()}, nil
}

// CreateWeaponPresetResult builds S832.
func CreateWeaponPresetResult(index uint8, success bool) Packet {
	w := NewWriter()
	w.Byte(index)
	if success {
		w.Unsigned(0)
	} else {
		w.Unsigned(1)
	}
	return Packet{ID: 832, Payload: w.Payload()}
}

// ReadWeaponPresetRequest parses C830.
func ReadWeaponPresetRequest(p Packet) (uint8, error) {
	if p.ID != 830 {
		return 0, fmt.Errorf("%w: unexpected preset request", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	if _, err := r.Unsigned(); err != nil {
		return 0, err
	}
	index, err := r.Byte()
	if err != nil {
		return 0, err
	}
	return index, r.RequireEnd()
}
