package protocol

import (
	"fmt"
	"math"
)

// Supply and item-slot packets of the cn-717 (KR2015 family) wire layout.
//
// The map report carries one word and fifteen counted vectors; the first three
// vectors a round needs are the point positions, the box kinds and the owner
// team of each point, followed by the regeneration bounds the host states. The
// layouts come from the 2015 client's own readers: the report reader is
// kr-812 sub_5312F0 (cn-717's play dispatcher reads the same object), the
// creation envelope is kr-812 sub_530570 / cn-717 sub_5677F0, the pickup is
// cn-324 sub_4672B0's object reference plus item kind, the drop request is
// kr-812 sub_515320's six floats and three words, and the item-slot family is
// the three-word gspAddItemToSlot group that cn-717 reads in sub_7F4370.
const (
	// CGameReportMapData is the host's map supply report (C2S 702).
	CGameReportMapData uint16 = 702
	// CEatItem asks to pick up a map box (C2S 707).
	CEatItem uint16 = 707
	// CUseItem uses the held item (C2S 708, empty body).
	CUseItem uint16 = 708
	// CCreateItem drops or throws a box (C2S 728).
	CCreateItem uint16 = 728
	// CSwitchItemSlot switches the selected item slot (C2S 744, empty body).
	CSwitchItemSlot uint16 = 744

	// SGameCreateGameItem creates one box for every round member (S2C 711).
	SGameCreateGameItem uint16 = 711
	// SItemEaten reports a box that a seat consumed (S2C 712).
	SItemEaten uint16 = 712
	// SAddItemToSlot grants the picked item to a slot (S2C 713).
	SAddItemToSlot uint16 = 713
	// SItemUsed reports the item a seat used (S2C 714).
	SItemUsed uint16 = 714
	// SSwitchItemSlot answers a slot switch (S2C 747).
	SSwitchItemSlot uint16 = 747
)

// supplyReportVectors is the number of counted vectors after the report's
// leading word.
const supplyReportVectors = 15

// maxSupplyVectorEntries bounds a counted vector that does not describe the
// supply points themselves: the report's earlier vectors describe the map's
// other objects, which can outnumber the points.
const maxSupplyVectorEntries = 4096

// maxMapCoordinate rejects a report whose "positions" cannot be map positions.
// The 2015 maps fit in a few thousand units, so anything past this is a layout
// mismatch rather than a real point.
const maxMapCoordinate = 100000

// SupplyReportPoint is one map supply point the host reported.
type SupplyReportPoint struct {
	X, Y, Z float32
	Kind    uint32
	MinMs   uint32
	MaxMs   uint32
	Team    uint32
}

// SupplyReport is the host's map supply report.
type SupplyReport struct {
	Points []SupplyReportPoint
	// IntervalsStated is false when the report states no regeneration bounds, in
	// which case the round applies its configured defaults.
	IntervalsStated bool
}

// SupplyDropRequest is a box a seat dropped or threw.
type SupplyDropRequest struct {
	X, Y, Z float32
	Kind    uint32
	Class   uint32
}

// Float reads one IEEE-754 single-precision field.
func (r *Reader) Float() (float32, error) {
	bits, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(bits), nil
}

// Float appends one IEEE-754 single-precision field.
func (w *Writer) Float(v float32) { w.Unsigned(math.Float32bits(v)) }

// readUnsignedList reads one counted u32 vector.
func (r *Reader) readUnsignedList(max int) ([]uint32, error) {
	count, err := r.Unsigned()
	if err != nil {
		return nil, err
	}
	if int(count) > max || int(count) > r.Remaining()/4 {
		return nil, fmt.Errorf("%w: supply vector of %d entries", ErrInvalidPacket, count)
	}
	out := make([]uint32, count)
	for i := range out {
		if out[i], err = r.Unsigned(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// readPositionList reads one counted vector of three-float positions.
func (r *Reader) readPositionList(max int) ([]SupplyReportPoint, error) {
	count, err := r.Unsigned()
	if err != nil {
		return nil, err
	}
	if int(count) > max || int(count) > r.Remaining()/12 {
		return nil, fmt.Errorf("%w: supply position vector of %d entries", ErrInvalidPacket, count)
	}
	out := make([]SupplyReportPoint, count)
	for i := range out {
		if out[i].X, err = r.Float(); err != nil {
			return nil, err
		}
		if out[i].Y, err = r.Float(); err != nil {
			return nil, err
		}
		if out[i].Z, err = r.Float(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// readTextList reads one counted wide-string vector. The report's last vector
// names the map's other objects; only its length matters here, so the values are
// dropped.
func (r *Reader) readTextList(max int) error {
	count, err := r.Unsigned()
	if err != nil {
		return err
	}
	if int(count) > max {
		return fmt.Errorf("%w: supply name vector of %d entries", ErrInvalidPacket, count)
	}
	for i := uint32(0); i < count; i++ {
		if _, err := r.String(maxTextUnits); err != nil {
			return err
		}
	}
	return nil
}

// maxTextUnits bounds one wide string inside a supply report.
const maxTextUnits = 256

// ReadSupplyReport decodes C702. Extra vectors are read and dropped so a build
// that states more of them still parses.
func ReadSupplyReport(packet Packet, maxPoints int) (SupplyReport, error) {
	var report SupplyReport
	if packet.ID != CGameReportMapData {
		return report, fmt.Errorf("%w: not a map supply report", ErrInvalidPacket)
	}
	if maxPoints <= 0 {
		maxPoints = 1
	}
	r := NewReader(packet.Payload)
	if _, err := r.Unsigned(); err != nil {
		return report, err
	}
	// Vector 0: unused words.
	if _, err := r.readUnsignedList(maxSupplyVectorEntries); err != nil {
		return report, err
	}
	positions, err := r.readPositionList(maxPoints)
	if err != nil {
		return report, err
	}
	kinds, err := r.readUnsignedList(maxPoints)
	if err != nil {
		return report, err
	}
	teams, err := r.readUnsignedList(maxPoints)
	if err != nil {
		return report, err
	}
	maxima, err := r.readUnsignedList(maxPoints)
	if err != nil {
		return report, err
	}
	minima, err := r.readUnsignedList(maxPoints)
	if err != nil {
		return report, err
	}
	// The remaining vectors describe the map's other objects.
	for vector := 6; vector < supplyReportVectors; vector++ {
		switch vector {
		case 7, 11: // two more position vectors
			if _, err := r.readPositionList(maxSupplyVectorEntries); err != nil {
				return report, err
			}
		case 14: // the trailing name vector
			if err := r.readTextList(maxSupplyVectorEntries); err != nil {
				return report, err
			}
		default:
			if _, err := r.readUnsignedList(maxSupplyVectorEntries); err != nil {
				return report, err
			}
		}
	}
	if len(kinds) != len(positions) || len(teams) != len(positions) ||
		len(maxima) != len(positions) || len(minima) != len(positions) {
		return report, fmt.Errorf("%w: supply vectors have different lengths", ErrInvalidPacket)
	}
	report.Points = make([]SupplyReportPoint, len(positions))
	for i := range positions {
		if !finite(positions[i].X) || !finite(positions[i].Y) || !finite(positions[i].Z) ||
			absGreater(positions[i].X, maxMapCoordinate) || absGreater(positions[i].Y, maxMapCoordinate) ||
			absGreater(positions[i].Z, maxMapCoordinate) {
			return report, fmt.Errorf("%w: supply positions are not map coordinates", ErrInvalidPacket)
		}
		if kinds[i] > 0xFFFF {
			return report, fmt.Errorf("%w: supply kind %d", ErrInvalidPacket, kinds[i])
		}
		report.Points[i] = SupplyReportPoint{
			X: positions[i].X, Y: positions[i].Y, Z: positions[i].Z,
			Kind: kinds[i], Team: teams[i], MaxMs: maxima[i], MinMs: minima[i],
		}
	}
	// A build that states no interval reports zero bounds; the round then applies
	// its own defaults.
	report.IntervalsStated = len(minima) > 0 && len(maxima) == len(minima)
	return report, nil
}

// ReadSupplyPickup decodes C707: the box object id and the item kind.
func ReadSupplyPickup(packet Packet) (objectID, kind uint32, err error) {
	if packet.ID != CEatItem {
		return 0, 0, fmt.Errorf("%w: not an item pickup", ErrInvalidPacket)
	}
	r := NewReader(packet.Payload)
	if objectID, err = r.Unsigned(); err != nil {
		return 0, 0, err
	}
	if kind, err = r.Unsigned(); err != nil {
		return 0, 0, err
	}
	// Trailing words are tolerated: a mismatch in the tail must not cost the
	// round its pickups, only the fields above are read.
	return objectID, kind, nil
}

// ReadSupplyDrop decodes C728: six floats and three words.
func ReadSupplyDrop(packet Packet) (SupplyDropRequest, error) {
	var request SupplyDropRequest
	if packet.ID != CCreateItem {
		return request, fmt.Errorf("%w: not a supply drop", ErrInvalidPacket)
	}
	r := NewReader(packet.Payload)
	for _, axis := range []*float32{&request.X, &request.Y, &request.Z} {
		value, err := r.Float()
		if err != nil {
			return request, err
		}
		if !finite(value) {
			return request, fmt.Errorf("%w: dropped item position is not finite", ErrInvalidPacket)
		}
		*axis = value
	}
	// The velocity triple is not used by the round.
	if err := r.Skip(12); err != nil {
		return request, err
	}
	kind, err := r.Unsigned()
	if err != nil {
		return request, err
	}
	request.Kind = kind
	if _, err := r.Unsigned(); err != nil {
		return request, err
	}
	class, err := r.Unsigned()
	if err != nil {
		return request, err
	}
	request.Class = class
	return request, nil
}

// ReadSupplyCreate decodes the ten-word creation envelope the clients exchange
// for a box the host scheduled (gppCreateMapItem 32832). The server creates its
// own boxes with the same body, so both directions share this layout.
func ReadSupplyCreate(packet Packet, maxKind uint32) (objectID, class uint32, point SupplyReportPoint, err error) {
	r := NewReader(packet.Payload)
	if objectID, err = r.Unsigned(); err != nil {
		return 0, 0, point, err
	}
	if class, err = r.Unsigned(); err != nil {
		return 0, 0, point, err
	}
	for _, axis := range []*float32{&point.X, &point.Y, &point.Z} {
		value, readErr := r.Float()
		if readErr != nil {
			return 0, 0, point, readErr
		}
		if !finite(value) {
			return 0, 0, point, fmt.Errorf("%w: created box position is not finite", ErrInvalidPacket)
		}
		*axis = value
	}
	if point.Kind, err = r.Unsigned(); err != nil {
		return 0, 0, point, err
	}
	if err := r.Skip(12); err != nil {
		return 0, 0, point, err
	}
	if point.Team, err = r.Unsigned(); err != nil {
		return 0, 0, point, err
	}
	if maxKind > 0 && point.Kind > maxKind {
		return 0, 0, point, fmt.Errorf("%w: created box kind %d", ErrInvalidPacket, point.Kind)
	}
	return objectID, class, point, nil
}

// CreateSupplyCreate builds S711 with the ten-word creation envelope.
func CreateSupplyCreate(objectID, class uint32, point SupplyReportPoint) Packet {
	w := NewWriter()
	w.Unsigned(objectID)
	w.Unsigned(class)
	w.Float(point.X)
	w.Float(point.Y)
	w.Float(point.Z)
	w.Unsigned(point.Kind)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(point.Team)
	return Packet{ID: SGameCreateGameItem, Payload: w.Payload()}
}

// CreateSupplyPicked builds S712: the seat and the box it consumed.
func CreateSupplyPicked(seat, objectID uint32) Packet {
	w := NewWriter()
	w.Unsigned(seat)
	w.Unsigned(objectID)
	return Packet{ID: SItemEaten, Payload: w.Payload()}
}

// CreateSupplyGranted builds S713: the seat, the granted kind and its slot.
func CreateSupplyGranted(seat, kind, slot uint32) Packet {
	w := NewWriter()
	w.Unsigned(seat)
	w.Unsigned(kind)
	w.Unsigned(slot)
	return Packet{ID: SAddItemToSlot, Payload: w.Payload()}
}

// CreateSupplyUsed builds S714: the seat, the used kind and its slot.
func CreateSupplyUsed(seat, kind, slot uint32) Packet {
	w := NewWriter()
	w.Unsigned(seat)
	w.Unsigned(kind)
	w.Unsigned(slot)
	return Packet{ID: SItemUsed, Payload: w.Payload()}
}

// CreateSupplySlotChanged builds S747: a nonzero result leaves the selection
// unchanged.
func CreateSupplySlotChanged(seat, slot, kind, result uint32) Packet {
	w := NewWriter()
	w.Unsigned(seat)
	w.Unsigned(slot)
	w.Unsigned(kind)
	w.Unsigned(result)
	return Packet{ID: SSwitchItemSlot, Payload: w.Payload()}
}

// absGreater reports whether a coordinate is outside the map's usable range.
func absGreater(v float32, limit float64) bool {
	return math.Abs(float64(v)) > limit
}

// finite reports whether a float is usable as a coordinate.
func finite(v float32) bool {
	f := float64(v)
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}
