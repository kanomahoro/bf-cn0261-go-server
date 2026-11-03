package protocol

import "testing"

// supplyReportBody builds the host's C702 body: one word and fifteen counted
// vectors, the shape kr-812 sub_5312F0 reads.
func supplyReportBody(points []SupplyReportPoint) []byte {
	w := NewWriter()
	w.Unsigned(0)
	// 1: unused words
	w.Unsigned(0)
	// 2: positions
	w.Unsigned(uint32(len(points)))
	for _, point := range points {
		w.Float(point.X)
		w.Float(point.Y)
		w.Float(point.Z)
	}
	// 3: kinds
	w.Unsigned(uint32(len(points)))
	for _, point := range points {
		w.Unsigned(point.Kind)
	}
	// 4: team
	w.Unsigned(uint32(len(points)))
	for _, point := range points {
		w.Unsigned(point.Team)
	}
	// 5: regeneration maximum, 6: minimum
	for _, pick := range []func(SupplyReportPoint) uint32{
		func(p SupplyReportPoint) uint32 { return p.MaxMs },
		func(p SupplyReportPoint) uint32 { return p.MinMs },
	} {
		w.Unsigned(uint32(len(points)))
		for _, point := range points {
			w.Unsigned(pick(point))
		}
	}
	// 7: unused words
	w.Unsigned(0)
	// 8: unused positions
	w.Unsigned(0)
	// 9..11: unused words
	for i := 0; i < 3; i++ {
		w.Unsigned(0)
	}
	// 12: unused positions
	w.Unsigned(0)
	// 13..14: unused words
	w.Unsigned(0)
	w.Unsigned(0)
	// 15: the trailing name vector
	w.Unsigned(1)
	w.String(U16String("supply_point"))
	return w.Payload()
}

func TestReadSupplyReport(t *testing.T) {
	want := []SupplyReportPoint{
		{X: 1.5, Y: -2.25, Z: 3, Kind: 101, MinMs: 15000, MaxMs: 30000, Team: 0},
		{X: 4, Y: 5, Z: 6, Kind: 0, MinMs: 20000, MaxMs: 20000, Team: 2},
	}
	report, err := ReadSupplyReport(Packet{ID: CGameReportMapData, Payload: supplyReportBody(want)}, 64)
	if err != nil {
		t.Fatalf("ReadSupplyReport: %v", err)
	}
	if len(report.Points) != len(want) {
		t.Fatalf("points = %d, want %d", len(report.Points), len(want))
	}
	for i, point := range report.Points {
		if point != want[i] {
			t.Fatalf("point %d = %+v, want %+v", i, point, want[i])
		}
	}
	if !report.IntervalsStated {
		t.Fatal("intervals should be stated")
	}
}

func TestReadSupplyReportRejectsMismatchedVectors(t *testing.T) {
	body := supplyReportBody([]SupplyReportPoint{{Kind: 101, MinMs: 1, MaxMs: 2}})
	// Drop the last word of the team vector so the vector lengths disagree.
	body = body[:len(body)-1]
	if _, err := ReadSupplyReport(Packet{ID: CGameReportMapData, Payload: body}, 64); err == nil {
		t.Fatal("a truncated report must be rejected")
	}
}

func TestSupplyCreateRoundTrip(t *testing.T) {
	point := SupplyReportPoint{X: 10, Y: 20, Z: -30, Kind: 138, Team: 1}
	packet := CreateSupplyCreate(0x63000001, ordinaryCreationClassTest, point)
	objectID, class, read, err := ReadSupplyCreate(packet, 1<<16)
	if err != nil {
		t.Fatalf("ReadSupplyCreate: %v", err)
	}
	if objectID != 0x63000001 || class != ordinaryCreationClassTest {
		t.Fatalf("object/class = %#x/%d", objectID, class)
	}
	if read != point {
		t.Fatalf("point = %+v, want %+v", read, point)
	}
}

const ordinaryCreationClassTest = 10000

func TestReadSupplyDrop(t *testing.T) {
	w := NewWriter()
	for _, v := range []float32{1, 2, 3, 0.5, 0.5, 0.5} {
		w.Float(v)
	}
	w.Unsigned(127)   // kind
	w.Unsigned(0)     // unknown
	w.Unsigned(10000) // creation class
	request, err := ReadSupplyDrop(Packet{ID: CCreateItem, Payload: w.Payload()})
	if err != nil {
		t.Fatalf("ReadSupplyDrop: %v", err)
	}
	if request.Kind != 127 || request.Class != 10000 || request.X != 1 || request.Z != 3 {
		t.Fatalf("request = %+v", request)
	}
}

func TestReadSupplyPickup(t *testing.T) {
	w := NewWriter()
	w.Unsigned(0x63000010)
	w.Unsigned(141)
	objectID, kind, err := ReadSupplyPickup(Packet{ID: CEatItem, Payload: w.Payload()})
	if err != nil {
		t.Fatalf("ReadSupplyPickup: %v", err)
	}
	if objectID != 0x63000010 || kind != 141 {
		t.Fatalf("object/kind = %#x/%d", objectID, kind)
	}
}

func TestSupplySlotChangedBody(t *testing.T) {
	want := []byte{1, 0, 0, 0, 0, 0, 0, 0, 141, 0, 0, 0, 1, 0, 0, 0}
	if got := CreateSupplySlotChanged(1, 0, 141, 1).Payload; string(got) != string(want) {
		t.Fatalf("payload = % x, want % x", got, want)
	}
}

// The zombie battle's team-change notice uses S504 with team 14; the encoder must
// not treat that as a room-team error.
func TestRoomTeamReplyCarriesTheZombieTeam(t *testing.T) {
	packet, err := CreateRoomTeamReply(3, 14)
	if err != nil {
		t.Fatalf("team 14 refused: %v", err)
	}
	if packet.ID != 504 || len(packet.Payload) != 8 {
		t.Fatalf("id = %d, payload = %d bytes", packet.ID, len(packet.Payload))
	}
	reader := NewReader(packet.Payload)
	slot, _ := reader.Unsigned()
	team, _ := reader.Unsigned()
	if slot != 3 || team != 14 {
		t.Fatalf("seat = %d team = %d, want 3 and 14", slot, team)
	}
	if _, err := CreateRoomTeamReply(0, 3); err == nil {
		t.Fatal("team 3 is not a team the client knows")
	}
}
