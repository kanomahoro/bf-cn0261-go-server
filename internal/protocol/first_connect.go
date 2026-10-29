package protocol

import "fmt"

// ProtocolVersion is the CN0261 wire protocol revision.
const ProtocolVersion = 717

// FirstConnect is C1, the packet every CN0261 client sends right after the
// transport greeting. Seven leading strings and fifteen trailing strings carry
// client state whose meaning is not confirmed; only the version is validated,
// exactly like the reference server.
type FirstConnect struct {
	Version uint32
}

// ReadFirstConnect parses C1 and enforces the same shape the original does.
func ReadFirstConnect(p Packet) (FirstConnect, error) {
	if p.ID != 1 {
		return FirstConnect{}, fmt.Errorf("%w: expected FirstConnect", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	version, err := r.Unsigned()
	if err != nil {
		return FirstConnect{}, err
	}
	if version != ProtocolVersion {
		return FirstConnect{}, fmt.Errorf("%w: unsupported client protocol version %d", ErrInvalidPacket, version)
	}
	// The reference reader uses its 32767-code-unit default here; a stricter
	// limit would disconnect clients that send long diagnostic strings.
	for i := 0; i < 7; i++ {
		if _, err := r.String(32767); err != nil {
			return FirstConnect{}, err
		}
	}
	if _, err := r.Unsigned(); err != nil {
		return FirstConnect{}, err
	}
	for i := 0; i < 15; i++ {
		if _, err := r.String(32767); err != nil {
			return FirstConnect{}, err
		}
	}
	if err := r.RequireEnd(); err != nil {
		return FirstConnect{}, err
	}
	return FirstConnect{Version: version}, nil
}

// CreateContentFlag builds S960: a content-tree key/value pair. The client only
// inserts missing keys, so 1004 disables the official login stage.
func CreateContentFlag(key, value uint32) Packet {
	w := NewWriter()
	w.Unsigned(key)
	w.Unsigned(value)
	return Packet{ID: 960, Payload: w.Payload()}
}

// CreatePatchReply builds S2 (gspRepGamePatch): 32-bit false + empty string.
func CreatePatchReply() Packet {
	w := NewWriter()
	w.Unsigned(0)
	w.String(nil)
	return Packet{ID: 2, Payload: w.Payload()}
}

// CreateCenterList builds S17. The client treats an empty list as one center and
// leaves the startup loading screen only after receiving it.
func CreateCenterList() Packet {
	w := NewWriter()
	w.Unsigned(0)
	return Packet{ID: 17, Payload: w.Payload()}
}
