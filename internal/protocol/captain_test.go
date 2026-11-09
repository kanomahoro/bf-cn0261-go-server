package protocol

import "testing"

// The Captain announcement is one seat word with the mode's own opcode.
func TestCreateCaptainReplyRoundTrip(t *testing.T) {
	packet, err := CreateCaptainReply(3)
	if err != nil {
		t.Fatalf("CreateCaptainReply: %v", err)
	}
	if packet.ID != 982 {
		t.Fatalf("opcode = %d, want 982", packet.ID)
	}
	if len(packet.Payload) != 4 {
		t.Fatalf("payload = %d bytes, want 4", len(packet.Payload))
	}
	r := NewReader(packet.Payload)
	seat, err := r.Unsigned()
	if err != nil || seat != 3 {
		t.Fatalf("seat = %d, %v; want 3", seat, err)
	}
	if _, err := CreateCaptainReply(32); err == nil {
		t.Fatal("a seat past the client's table must be refused")
	}
}

// The drop report is one seat word; a foreign opcode, a short body or a seat
// past the table is refused.
func TestReadDropCaptain(t *testing.T) {
	packet, err := CreateCaptainReply(5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDropCaptain(packet); err == nil {
		t.Fatal("a 982 announcement is not a drop report")
	}
	dropped := Packet{ID: 2018, Payload: packet.Payload}
	seat, err := ReadDropCaptain(dropped)
	if err != nil || seat != 5 {
		t.Fatalf("seat = %d, %v; want 5", seat, err)
	}
	if _, err := ReadDropCaptain(Packet{ID: 2018, Payload: []byte{1, 0}}); err == nil {
		t.Fatal("a short body must be refused")
	}
	if _, err := ReadDropCaptain(Packet{ID: 2018, Payload: []byte{32, 0, 0, 0}}); err == nil {
		t.Fatal("a seat past the table must be refused")
	}
	// A tail this server does not know is tolerated: the report is a trigger, not
	// state, so it must not cost the player its connection.
	if seat, err := ReadDropCaptain(Packet{ID: 2018, Payload: []byte{5, 0, 0, 0, 9, 9, 9, 9}}); err != nil || seat != 5 {
		t.Fatalf("seat = %d, %v; want 5 with the tail ignored", seat, err)
	}
}
