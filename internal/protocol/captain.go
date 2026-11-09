package protocol

import "fmt"

// The Captain battle's exchanges (game mode 20).
//
// The mode has three packets on the 2015 client. Two of them are the server's:
// S982 gspCreateCaptain names one seat per team, and the client marks that seat's
// character as its Captain on its own (cn-717 dispatches case 982 in sub_87EFE0 to
// sub_881080, whose handler sub_435330 marks the seat). The third, C2018
// gcpDropCaptain, is the client's local-death report: the dying client sends the
// seat of the character that died, whether or not that seat was the Captain
// (cn-717 sub_431430 builds the seat, sub_43FB90 stamps 2018, sub_88E6C0 writes
// the single word).
//
// The Captain's ULTRA needs no packet of its own. With its gauge full the client
// requests the ordinary pickup with no scene object and item kind 104, and the
// round answers with S715 gspPveItemUsed -- the same confirmation the zombie
// king's ULTRA already uses (see CreatePveItemUsedReply).
//
// Reference: BubbleFighter.Fantasy packets/mode-control.yaml (gspCreateCaptain,
// gcpDropCaptain) and RoundFlow.Captain.cs.

// CreateCaptainReply builds S982 gspCreateCaptain: the seat the server names as
// one team's Captain, one word.
//
// The client reads the character at the per-seat table index this word names and
// does nothing when that seat has no character yet, so the announcement has to
// follow the characters' creation; the round sends it once the countdown is over.
func CreateCaptainReply(seat uint32) (Packet, error) {
	if seat >= 32 {
		return Packet{}, fmt.Errorf("%w: captain seat", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(seat)
	return Packet{ID: 982, Payload: w.Payload()}, nil
}

// ReadDropCaptain decodes C2018 gcpDropCaptain: the seat of the local character
// that just died, one word.
//
// Trailing words are tolerated the way the pickup decoder tolerates them: the
// report is only a trigger to re-check the appointments, and a tail this server
// does not know must not cost the player its connection.
func ReadDropCaptain(packet Packet) (uint32, error) {
	if packet.ID != 2018 {
		return 0, fmt.Errorf("%w: not a captain drop report", ErrInvalidPacket)
	}
	r := NewReader(packet.Payload)
	seat, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	if seat >= 32 {
		return 0, fmt.Errorf("%w: captain drop seat", ErrInvalidPacket)
	}
	return seat, nil
}
