package protocol

import (
	"math"
	"testing"
)

// bombWord reads one little-endian word out of a payload.
func bombWord(payload []byte, index int) uint32 {
	return uint32(payload[index*4]) | uint32(payload[index*4+1])<<8 |
		uint32(payload[index*4+2])<<16 | uint32(payload[index*4+3])<<24
}

// TestReadBombState checks the five-word report the client sends.
func TestReadBombState(t *testing.T) {
	w := NewWriter()
	w.Unsigned(1234567)
	w.Unsigned(BombStateDefusing)
	w.Unsigned(math.Float32bits(1.5))
	w.Unsigned(math.Float32bits(-2.25))
	w.Unsigned(math.Float32bits(0))
	report, err := ReadBombState(w.Payload())
	if err != nil {
		t.Fatalf("report refused: %v", err)
	}
	if report.Operator != 1234567 || report.State != BombStateDefusing {
		t.Fatalf("fields decoded wrong: %+v", report)
	}
	if report.X != 1.5 || report.Y != -2.25 || report.Z != 0 {
		t.Fatalf("position decoded wrong: %+v", report)
	}
	if _, err := ReadBombState(append(w.Payload(), 0)); err == nil {
		t.Fatal("a report with a trailing byte was accepted")
	}
	if _, err := ReadBombState(w.Payload()[:19]); err == nil {
		t.Fatal("a truncated report was accepted")
	}
}

// TestCreateBombStateReply checks the six-word broadcast, including the clear
// timer word that selects the client's own 60000 ms countdown.
func TestCreateBombStateReply(t *testing.T) {
	packet := CreateBombStateReply(BombState{Operator: 42, State: BombStateCountdown, X: 1, Y: 2, Z: 3})
	if packet.ID != 973 {
		t.Fatalf("wrong packet id: %d", packet.ID)
	}
	if len(packet.Payload) != 24 {
		t.Fatalf("wrong body length: %d", len(packet.Payload))
	}
	want := []uint32{42, BombStateCountdown,
		math.Float32bits(1), math.Float32bits(2), math.Float32bits(3), 0}
	for index, value := range want {
		if got := bombWord(packet.Payload, index); got != value {
			t.Fatalf("word %d is %d, want %d", index, got, value)
		}
	}
}

// TestReadBombMissionOver checks the three-word sub-round report.
func TestReadBombMissionOver(t *testing.T) {
	w := NewWriter()
	w.Unsigned(2)
	w.Unsigned(BombOperatorNone)
	w.Unsigned(900)
	report, err := ReadBombMissionOver(w.Payload())
	if err != nil {
		t.Fatalf("report refused: %v", err)
	}
	if report.WinningTeam != 2 || report.Planter != BombOperatorNone || report.Defuser != 900 {
		t.Fatalf("fields decoded wrong: %+v", report)
	}
	if _, err := ReadBombMissionOver(w.Payload()[:8]); err == nil {
		t.Fatal("a truncated report was accepted")
	}
}

// TestCreateObserverReply checks the single scene-object word.
func TestCreateObserverReply(t *testing.T) {
	packet := CreateObserverReply(1200000)
	if packet.ID != 974 || len(packet.Payload) != 4 {
		t.Fatalf("wrong observer packet: id %d length %d", packet.ID, len(packet.Payload))
	}
	if got := bombWord(packet.Payload, 0); got != 1200000 {
		t.Fatalf("wrong actor: %d", got)
	}
}

// TestCreateGameEntrySeriesReply checks that the series words land in the three
// positions the plain entry packet leaves zeroed.
func TestCreateGameEntrySeriesReply(t *testing.T) {
	packet := CreateGameEntrySeriesReply(StartSuccess, U16String("rel_Village01_Byway"), 3,
		RoundSeries{Round: 2, Attacking: 1})
	if packet.ID != 510 {
		t.Fatalf("wrong packet id: %d", packet.ID)
	}
	r := NewReader(packet.Payload)
	if result, err := r.Unsigned(); err != nil || result != StartSuccess {
		t.Fatalf("result word: %v %v", result, err)
	}
	name, err := r.String(64)
	if err != nil || FromU16(name) != "rel_Village01_Byway" {
		t.Fatalf("map key: %q %v", FromU16(name), err)
	}
	want := []uint32{2, 1, 3, 0, 0, 0}
	for index, value := range want {
		got, err := r.Unsigned()
		if err != nil {
			t.Fatalf("word %d: %v", index, err)
		}
		if got != value {
			t.Fatalf("word %d is %d, want %d", index, got, value)
		}
	}
	if err := r.RequireEnd(); err != nil {
		t.Fatalf("trailing bytes after the series entry: %v", err)
	}
}

// TestPlainEntryStillZeroesTheSeriesWords checks that the packets of every other
// mode keep their shape.
func TestPlainEntryStillZeroesTheSeriesWords(t *testing.T) {
	series := CreateGameEntrySeriesReply(StartSuccess, U16String("rel_Village01_Byway"), 3, RoundSeries{})
	plain := CreateGameEntryReply(StartSuccess, U16String("rel_Village01_Byway"), 3)
	if len(series.Payload) != len(plain.Payload) {
		t.Fatalf("series entry changed the body length: %d vs %d", len(series.Payload), len(plain.Payload))
	}
	for index := range plain.Payload {
		if series.Payload[index] != plain.Payload[index] {
			t.Fatalf("byte %d differs between a zero series and the plain entry", index)
		}
	}
}
