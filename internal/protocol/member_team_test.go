package protocol

import "testing"

// tutorialSeatMember is a seated record with the given team.
func tutorialSeatMember(team uint8) SessionMember {
	return SessionMember{
		Character: &Character{AccountID: 7, Nickname: U16String("tester"), CharacterType: 1, Level: 1},
		Team:      team,
	}
}

// The single-player tutorial's seat carries team 0, the client's own "no team"
// value; the room teams and the cooperative team 16 still serialize as before.
func TestWriteSessionMemberAcceptsTheTutorialTeam(t *testing.T) {
	for _, team := range []uint8{0, 1, 2, 16} {
		w := NewWriter()
		if err := WriteSessionMember(w, tutorialSeatMember(team), 0); err != nil {
			t.Fatalf("team %d must serialize: %v", team, err)
		}
	}
}

// The zombie battle's team 14 is written by its own team packet, never by a seat
// record: adding team 0 above must not admit it here.
func TestWriteSessionMemberStillRefusesTheZombieTeam(t *testing.T) {
	w := NewWriter()
	if err := WriteSessionMember(w, tutorialSeatMember(14), 0); err == nil {
		t.Fatal("team 14 must still be refused in a seat record")
	}
	w = NewWriter()
	if err := WriteSessionMember(w, tutorialSeatMember(17), 0); err == nil {
		t.Fatal("an unknown team must still be refused")
	}
}
