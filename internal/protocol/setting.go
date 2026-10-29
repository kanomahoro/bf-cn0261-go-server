package protocol

import "fmt"

// SessionStringLimit bounds every attacker-controlled session string.
const SessionStringLimit = 256

// GameStringCountLimit bounds the variable-length game setting list.
const GameStringCountLimit = 16

// GameSetting mirrors the client's GameSetting object. Field names containing an
// offset refer to the original x86 object, never to a wire offset.
type GameSetting struct {
	Mode       uint32
	SubMode    uint32
	Strings16  [4][]uint16
	Values128  [5]uint32
	Flag4      bool
	Flag5      bool
	Values148  [8]uint32
	Strings180 [6][]uint16
	Value348   uint32
	Strings352 []uint16Slice
	Flag368    bool
	Value372   uint32
	Flag376    bool
	Flag377    bool
	Value380   uint32
	Value384   uint32
	String388  []uint16
	String416  []uint16
	Flag444    bool
	Value448   uint32
}

// uint16Slice exists so Strings352 stays a slice of strings rather than of
// byte slices.
type uint16Slice = []uint16

// DefaultGameSetting returns the client constructor defaults.
func DefaultGameSetting() GameSetting {
	return GameSetting{
		Mode:      1,
		SubMode:   0,
		Flag4:     true,
		Flag5:     true,
		Values148: [8]uint32{0, 0, 0, 0, 1, 0xFFFFFFFF, 0, 0},
	}
}

// SessionSetting mirrors the client's SessionSetting (room card) object.
type SessionSetting struct {
	State            uint32
	RoomID           uint16
	Title            []uint16
	MemberCount      uint16
	Value42          uint16
	Capacity         uint16
	PasswordRequired bool
	Password         []uint16
	Kind             uint32
	Game             GameSetting
	Value532         uint32
	Value536         uint64
	String544        []uint16
	Values572        [4]uint32
	Flag588          bool
}

// DefaultSessionSetting returns defaults matching the client constructor.
func DefaultSessionSetting() SessionSetting {
	return SessionSetting{
		Game:     DefaultGameSetting(),
		Value532: 0xFFFFFFFF,
		Value536: 0xFFFFFFFFFFFFFFFF,
	}
}

func writeSettingString(w *Writer, value []uint16) error {
	if len(value) > SessionStringLimit {
		return fmt.Errorf("%w: session string exceeds %d units", ErrInvalidPacket, SessionStringLimit)
	}
	w.String(value)
	return nil
}

// ReadGameSetting parses a GameSetting from the reader.
func ReadGameSetting(r *Reader) (GameSetting, error) {
	s := DefaultGameSetting()
	var err error
	if s.Mode, err = r.Unsigned(); err != nil {
		return s, err
	}
	if s.SubMode, err = r.Unsigned(); err != nil {
		return s, err
	}
	for i := range s.Strings16 {
		if s.Strings16[i], err = r.String(SessionStringLimit); err != nil {
			return s, err
		}
	}
	for i := range s.Values128 {
		if s.Values128[i], err = r.Unsigned(); err != nil {
			return s, err
		}
	}
	if s.Flag4, err = r.Boolean(); err != nil {
		return s, err
	}
	if s.Flag5, err = r.Boolean(); err != nil {
		return s, err
	}
	for i := range s.Values148 {
		if s.Values148[i], err = r.Unsigned(); err != nil {
			return s, err
		}
	}
	for i := range s.Strings180 {
		if s.Strings180[i], err = r.String(SessionStringLimit); err != nil {
			return s, err
		}
	}
	if s.Value348, err = r.Unsigned(); err != nil {
		return s, err
	}
	count, err := r.Unsigned()
	if err != nil {
		return s, err
	}
	if count > GameStringCountLimit || int(count) > r.Remaining()/4 {
		return s, fmt.Errorf("%w: game string count %d", ErrInvalidPacket, count)
	}
	s.Strings352 = make([]uint16Slice, 0, count)
	for i := uint32(0); i < count; i++ {
		value, err := r.String(SessionStringLimit)
		if err != nil {
			return s, err
		}
		s.Strings352 = append(s.Strings352, value)
	}
	if s.Flag368, err = r.Boolean(); err != nil {
		return s, err
	}
	if s.Value372, err = r.Unsigned(); err != nil {
		return s, err
	}
	if s.Flag376, err = r.Boolean(); err != nil {
		return s, err
	}
	if s.Flag377, err = r.Boolean(); err != nil {
		return s, err
	}
	if s.Value380, err = r.Unsigned(); err != nil {
		return s, err
	}
	if s.Value384, err = r.Unsigned(); err != nil {
		return s, err
	}
	if s.String388, err = r.String(SessionStringLimit); err != nil {
		return s, err
	}
	if s.String416, err = r.String(SessionStringLimit); err != nil {
		return s, err
	}
	if s.Flag444, err = r.Boolean(); err != nil {
		return s, err
	}
	if s.Value448, err = r.Unsigned(); err != nil {
		return s, err
	}
	return s, nil
}

// ReadSessionSetting parses a SessionSetting from the reader.
func ReadSessionSetting(r *Reader) (SessionSetting, error) {
	s := DefaultSessionSetting()
	var err error
	if s.State, err = r.Unsigned(); err != nil {
		return s, err
	}
	if s.RoomID, err = r.Short(); err != nil {
		return s, err
	}
	if s.Title, err = r.String(SessionStringLimit); err != nil {
		return s, err
	}
	if s.MemberCount, err = r.Short(); err != nil {
		return s, err
	}
	if s.Value42, err = r.Short(); err != nil {
		return s, err
	}
	if s.Capacity, err = r.Short(); err != nil {
		return s, err
	}
	if s.PasswordRequired, err = r.Boolean(); err != nil {
		return s, err
	}
	if s.Password, err = r.String(SessionStringLimit); err != nil {
		return s, err
	}
	if s.Kind, err = r.Unsigned(); err != nil {
		return s, err
	}
	if s.Game, err = ReadGameSetting(r); err != nil {
		return s, err
	}
	if s.Value532, err = r.Unsigned(); err != nil {
		return s, err
	}
	low, err := r.Unsigned()
	if err != nil {
		return s, err
	}
	high, err := r.Unsigned()
	if err != nil {
		return s, err
	}
	s.Value536 = uint64(low) | uint64(high)<<32
	if s.String544, err = r.String(SessionStringLimit); err != nil {
		return s, err
	}
	for i := range s.Values572 {
		if s.Values572[i], err = r.Unsigned(); err != nil {
			return s, err
		}
	}
	if s.Flag588, err = r.Boolean(); err != nil {
		return s, err
	}
	return s, nil
}

// WriteGameSetting serializes a GameSetting.
func WriteGameSetting(w *Writer, s GameSetting) error {
	if len(s.Strings352) > GameStringCountLimit {
		return fmt.Errorf("%w: game string count %d", ErrInvalidPacket, len(s.Strings352))
	}
	w.Unsigned(s.Mode)
	w.Unsigned(s.SubMode)
	for _, value := range s.Strings16 {
		if err := writeSettingString(w, value); err != nil {
			return err
		}
	}
	for _, value := range s.Values128 {
		w.Unsigned(value)
	}
	w.Boolean(s.Flag4)
	w.Boolean(s.Flag5)
	for _, value := range s.Values148 {
		w.Unsigned(value)
	}
	for _, value := range s.Strings180 {
		if err := writeSettingString(w, value); err != nil {
			return err
		}
	}
	w.Unsigned(s.Value348)
	w.Unsigned(uint32(len(s.Strings352)))
	for _, value := range s.Strings352 {
		if err := writeSettingString(w, value); err != nil {
			return err
		}
	}
	w.Boolean(s.Flag368)
	w.Unsigned(s.Value372)
	w.Boolean(s.Flag376)
	w.Boolean(s.Flag377)
	w.Unsigned(s.Value380)
	w.Unsigned(s.Value384)
	if err := writeSettingString(w, s.String388); err != nil {
		return err
	}
	if err := writeSettingString(w, s.String416); err != nil {
		return err
	}
	w.Boolean(s.Flag444)
	w.Unsigned(s.Value448)
	return nil
}

// WriteSessionSetting serializes a SessionSetting.
func WriteSessionSetting(w *Writer, s SessionSetting) error {
	w.Unsigned(s.State)
	w.Short(s.RoomID)
	if err := writeSettingString(w, s.Title); err != nil {
		return err
	}
	w.Short(s.MemberCount)
	w.Short(s.Value42)
	w.Short(s.Capacity)
	w.Boolean(s.PasswordRequired)
	if err := writeSettingString(w, s.Password); err != nil {
		return err
	}
	w.Unsigned(s.Kind)
	if err := WriteGameSetting(w, s.Game); err != nil {
		return err
	}
	w.Unsigned(s.Value532)
	w.Unsigned(uint32(s.Value536))
	w.Unsigned(uint32(s.Value536 >> 32))
	if err := writeSettingString(w, s.String544); err != nil {
		return err
	}
	for _, value := range s.Values572 {
		w.Unsigned(value)
	}
	w.Boolean(s.Flag588)
	return nil
}
