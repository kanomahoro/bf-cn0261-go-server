package protocol

import "fmt"

// The cooperative zombie raid's own packets (mode 16, wgtZombiePve).
//
// The numbers and the field order are cn-717's own room dispatcher, not a
// name match against another build:
//
//	S2C 924 gspMapMissionInfo   sub_87EFE0 route 924  -> sub_881DE0
//	S2C 925 gspResMissionStart  sub_87EFE0 route 925  -> sub_8821C0
//	S2C 926 gspResMissionTimeOut sub_87EFE0 route 926 -> sub_8822E0
//	S2C 927 gspResMissionOK     sub_87EFE0 route 927  -> sub_882400
//	S2C 928 gspNotifyGamePoint  sub_87EFE0 case 0x3A0 -> sub_882530
//	S2C 929 gspNotifyRankInfo   sub_87EFE0 case 0x3A1 -> sub_8825E0
//	S2C 930 gspPveItemInfo      sub_87EFE0 case 930   -> sub_882980
//	C2S 921 gcpPveUseCountItem  sender sub_542F10
//	C2S 923 gcpPvePangUnit      sender sub_543030
//	C2S 925 gcpPveUseRevival    sender sub_445350 (empty)
//	C2S 927 gcpMissionProcess   sender sub_5430C0
//
// The layouts are the ones BubbleFighter.Fantasy audited for this build
// (packets/pve-rescue-missions.yaml, packets/pve-counted-items.yaml and
// packets/pve-points.yaml): every reader the tables name for cn-717 is the
// client function that consumes the same words.

// PveMissionLimit is the client's own cap on a map's mission list: MapMeta's
// ZombiePve resources state MAXINGAMEMISSIONCNT = 6, and the list reader walks
// at most that many entries.
const PveMissionLimit = 6

// PveMissionArgumentLimit is the client's own cap on one mission's arguments.
const PveMissionArgumentLimit = 4

// PveItemSlotLimit is the counted-item slot bound the mode serves (slots 1..3:
// heal, bazooka, air bomb).
const PveItemSlotLimit = 3

// PveMission is one entry of the 924 mission list.
//
// The client matches MissionID against its own MissionInfo_Clt.xml for the
// presentation row, then picks the map's mission spawner group by the entry's
// list index and resolves the arguments against it ("killmonster", "1", the
// hostile count and the citizen's CHR_PROPERTY name).
type PveMission struct {
	// ID is the presentation row's mission id.
	ID uint32
	// StartTimeMs is the mission's start on the round clock.
	StartTimeMs uint32
	// DurationMs is how long the team has to finish it.
	DurationMs uint32
	// Arguments are the counted wide strings the client reads.
	Arguments []string
}

// PveRankRow is one row of the 929 rank table.
type PveRankRow struct {
	Seat   uint32
	Points uint32
	Rank   uint32
}

// CreateMapMissionInfoReply builds S924 gspMapMissionInfo: a counted list of
// missions. The three words between the id and the start time are consumed by
// no reader the audit found; the reference writes zeros there and so does this.
func CreateMapMissionInfoReply(missions []PveMission) (Packet, error) {
	if len(missions) > PveMissionLimit {
		return Packet{}, fmt.Errorf("%w: mission list", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(uint32(len(missions)))
	for _, mission := range missions {
		if len(mission.Arguments) > PveMissionArgumentLimit {
			return Packet{}, fmt.Errorf("%w: mission arguments", ErrInvalidPacket)
		}
		w.Unsigned(mission.ID)
		w.Unsigned(0)
		w.Unsigned(0)
		w.Unsigned(0)
		w.Unsigned(mission.StartTimeMs)
		w.Unsigned(mission.DurationMs)
		w.Unsigned(uint32(len(mission.Arguments)))
		for _, argument := range mission.Arguments {
			w.String(U16String(argument))
		}
		// The trailing bool32 is consumed by no reader either.
		w.Unsigned(0)
	}
	return Packet{ID: 924, Payload: w.Payload()}, nil
}

// CreateResMissionStartReply builds S925 gspResMissionStart: the list index of
// the mission that just began. The client selects that map spawner group.
func CreateResMissionStartReply(index uint32) Packet {
	w := NewWriter()
	w.Unsigned(index)
	return Packet{ID: 925, Payload: w.Payload()}
}

// CreateResMissionTimedOutReply builds S926 gspResMissionTimeOut: the index the
// client should move on to once the active mission failed. The reference's own
// failure path states the failed index plus one.
func CreateResMissionTimedOutReply(nextIndex uint32) Packet {
	w := NewWriter()
	w.Unsigned(nextIndex)
	return Packet{ID: 926, Payload: w.Payload()}
}

// CreateResMissionSucceededReply builds S927 gspResMissionOK: a zero flag (only
// zero runs the native success path) and the index that succeeded.
func CreateResMissionSucceededReply(index uint32) Packet {
	w := NewWriter()
	w.Unsigned(0)
	w.Unsigned(index)
	return Packet{ID: 927, Payload: w.Payload()}
}

// CreateNotifyGamePointReply builds S928 gspNotifyGamePoint: the seat, a
// subtraction flag (zero for an award) and the amount.
func CreateNotifyGamePointReply(seat, subtract, amount uint32) (Packet, error) {
	if seat >= 32 {
		return Packet{}, fmt.Errorf("%w: game point seat", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(seat)
	w.Unsigned(subtract)
	w.Unsigned(amount)
	return Packet{ID: 928, Payload: w.Payload()}, nil
}

// CreateNotifyRankInfoReply builds S929 gspNotifyRankInfo: a state word, an
// empty u32 vector and the counted rank rows. The client sums the row's three
// score words locally and keeps Rank separately; this server writes the points
// into the first one and zeros the other two.
func CreateNotifyRankInfoReply(rows []PveRankRow) (Packet, error) {
	if len(rows) > 32 {
		return Packet{}, fmt.Errorf("%w: rank rows", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(0) // state
	w.Unsigned(0) // empty u32 vector
	w.Unsigned(uint32(len(rows)))
	for _, row := range rows {
		if row.Seat >= 32 || row.Rank == 0 || row.Rank > 32 {
			return Packet{}, fmt.Errorf("%w: rank row", ErrInvalidPacket)
		}
		w.Unsigned(row.Seat)
		w.Unsigned(row.Points)
		w.Unsigned(0)
		w.Unsigned(0)
		w.Unsigned(row.Rank)
	}
	return Packet{ID: 929, Payload: w.Payload()}, nil
}

// CreatePveItemsReply builds S930 gspPveItemInfo: the round's counted-item
// slots. The native list is indexed by slot, so entry zero is the reserved
// "no item" slot and the slots the mode uses start at one.
func CreatePveItemsReply(kinds []uint32) (Packet, error) {
	if len(kinds) > PveItemSlotLimit {
		return Packet{}, fmt.Errorf("%w: counted item kinds", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(uint32(len(kinds) + 1))
	w.Unsigned(0) // the reserved slot zero
	w.Unsigned(0)
	w.Unsigned(0)
	for index, kind := range kinds {
		w.Unsigned(0)
		w.Unsigned(kind)
		w.Unsigned(uint32(index + 1))
	}
	return Packet{ID: 930, Payload: w.Payload()}, nil
}

// ReadPveItemUseRequest decodes C921 gcpPveUseCountItem: the UI slot the player
// picked.
func ReadPveItemUseRequest(packet Packet) (uint32, error) {
	r := NewReader(packet.Payload)
	slot, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	if err := r.RequireEnd(); err != nil {
		return 0, err
	}
	return slot, nil
}

// ReadPvePangUnit decodes C923 gcpPvePangUnit: the score unit of the NPC that
// just died. The unit selects the server's point value; a unit of -1 means the
// NPC carried none.
func ReadPvePangUnit(packet Packet) (uint32, error) {
	r := NewReader(packet.Payload)
	unit, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	if err := r.RequireEnd(); err != nil {
		return 0, err
	}
	return unit, nil
}

// ReadMissionProgress decodes C927 gcpMissionProcess: the mission's list index,
// the character type that died (100 the rescued citizen, 101 a zombie, 102 a
// poison sac) and a trailing word the native writers hardcode to zero.
func ReadMissionProgress(packet Packet) (index, characterType, trailing uint32, err error) {
	r := NewReader(packet.Payload)
	if index, err = r.Unsigned(); err != nil {
		return 0, 0, 0, err
	}
	if characterType, err = r.Unsigned(); err != nil {
		return 0, 0, 0, err
	}
	if trailing, err = r.Unsigned(); err != nil {
		return 0, 0, 0, err
	}
	if err = r.RequireEnd(); err != nil {
		return 0, 0, 0, err
	}
	return index, characterType, trailing, nil
}

// ReadPveRevival decodes C925 gcpPveUseRevival: the request is empty and the
// identity comes from the connection.
func ReadPveRevival(packet Packet) error {
	if len(packet.Payload) != 0 {
		return fmt.Errorf("%w: expected empty PvE revival", ErrInvalidPacket)
	}
	return nil
}
