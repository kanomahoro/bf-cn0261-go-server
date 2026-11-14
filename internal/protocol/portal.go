package protocol

import (
	"fmt"
	"math"
)

// Portal ids: the map-contact exchange. The client walks into a map's portal and
// asks the server for permission; the reply says who may move. The numbers come
// from cn-717's own table (config/cn-717: C2S 948 gcpPortalMove, S2C 958
// gspPortalMove), never from another build: 958 is gspRepRecheckMap on cn-774 and
// tw-454, so a reply looked up by number alone would be wrong there.
const (
	// CPortalMove is the game connection's map-contact request.
	CPortalMove uint16 = 948
	// SPortalMove is its reply.
	SPortalMove uint16 = 958
)

// portalMoveSize is the request's body: three position components and the
// region key (12 + 2 bytes).
const portalMoveSize = 14

// portalMoveReplySize is the reply's body: the movement word, the region key and
// the seat (4 + 2 + 4 bytes, with no padding).
const portalMoveReplySize = 10

// PortalMove is one map-contact request.
type PortalMove struct {
	X        float32
	Y        float32
	Z        float32
	PortalID uint16
}

// ReadPortalMove parses C948 gcpPortalMove. The reference refuses a vector that is
// not finite before it answers anything, so the same check lives here.
func ReadPortalMove(p Packet) (PortalMove, error) {
	if p.ID != CPortalMove {
		return PortalMove{}, fmt.Errorf("%w: unsupported portal move", ErrInvalidPacket)
	}
	if len(p.Payload) != portalMoveSize {
		return PortalMove{}, fmt.Errorf("%w: portal move is %d bytes", ErrInvalidPacket, len(p.Payload))
	}
	r := NewReader(p.Payload)
	var move PortalMove
	var err error
	if move.X, err = r.Float(); err != nil {
		return PortalMove{}, err
	}
	if move.Y, err = r.Float(); err != nil {
		return PortalMove{}, err
	}
	if move.Z, err = r.Float(); err != nil {
		return PortalMove{}, err
	}
	if move.PortalID, err = r.Short(); err != nil {
		return PortalMove{}, err
	}
	if err := r.RequireEnd(); err != nil {
		return PortalMove{}, err
	}
	for _, value := range []float32{move.X, move.Y, move.Z} {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return PortalMove{}, fmt.Errorf("%w: portal move vector is not finite", ErrInvalidPacket)
		}
	}
	return move, nil
}

// CreatePortalMoveReply builds S958 gspPortalMove: the movement word (zero enters
// the client's own region path, -1 would skip it), the region key and the seat the
// move belongs to.
func CreatePortalMoveReply(portalID uint16, seat uint32) Packet {
	w := NewWriter()
	w.Unsigned(0)
	w.Short(portalID)
	w.Unsigned(seat)
	return Packet{ID: SPortalMove, Payload: w.Payload()}
}
