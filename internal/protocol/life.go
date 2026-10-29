package protocol

import (
	"fmt"
	"math"
)

// Peer life kinds.
type PeerLifeKind int

// Peer life event kinds.
const (
	LifeKnocked PeerLifeKind = iota
	LifeTryRescue
	LifePopOut
	LifeRescue
	LifeDying
	LifeDead
	LifeBubbleSettled
	LifeSelfPopOut
	LifeStunned
	LifeEndStunned
)

// PeerLifeEvent is one victim/actor reported life event.
type PeerLifeEvent struct {
	Kind          PeerLifeKind
	VictimObject  uint32
	ActorObject   uint32
	WeaponDef     uint32
	Tick          uint32
	DirectDeath   bool
	SpecialRescue bool
}

// IsPeerLife reports whether the id is a life event packet.
func IsPeerLife(id uint16) bool {
	switch id {
	case 32787, 32793, 32903, 32807, 32805, 32836, 32801, 32806, 32788, 32790:
		return true
	}
	return false
}

// IsPeerKick reports whether the id is a kick event packet.
func IsPeerKick(id uint16) bool {
	switch id {
	case 32791, 32792, 32794, 32795, 32796, 32835, 32837, 32815, 32816:
		return true
	}
	return false
}

// PeerKickKind enumerates the kick event kinds.
type PeerKickKind int

// Peer kick event kinds.
const (
	KickStart PeerKickKind = iota
	KickTry
	KickConfirmed
	KickProtected
	KickGotKicked
	KickEnd
	KickFailed
	KickHumiliationStart
	KickHumiliationEnd
)

// PeerKickEvent is one kick-related event.
type PeerKickEvent struct {
	Kind        PeerKickKind
	Victim      uint32
	Actor       uint32
	Style       uint32
	Succeeded   bool
	SpecialKick bool
}

// PeerLifePoint is the client's own remaining life-point report (C32920).
type PeerLifePoint struct {
	CharacterObject uint32
	RemainingPoints uint32
	Effect          uint32
}

// ValidateObject checks the per-slot object partition.
func ValidateObject(object uint32) error {
	if object < 1000000 || (object-1000000)/100000 >= 8 {
		return fmt.Errorf("%w: object outside the player partitions", ErrInvalidPacket)
	}
	return nil
}

func readCoordinate(r *Reader, minimum float64) error {
	bits, err := r.Unsigned()
	if err != nil {
		return err
	}
	value := math.Float32frombits(bits)
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) ||
		float64(value) < minimum || float64(value) > 10000000 {
		return fmt.Errorf("%w: peer life coordinate", ErrInvalidPacket)
	}
	return nil
}

// ReadPeerLife parses one peer life packet.
func ReadPeerLife(p Packet) (PeerLifeEvent, error) {
	if !IsPeerLife(p.ID) {
		return PeerLifeEvent{}, fmt.Errorf("%w: unsupported peer life %d", ErrInvalidPacket, p.ID)
	}
	r := NewReader(p.Payload)
	first, err := r.Unsigned()
	if err != nil {
		return PeerLifeEvent{}, err
	}
	var event PeerLifeEvent

	switch p.ID {
	case 32788, 32790:
		if err := ValidateObject(first); err != nil {
			return event, err
		}
		event.VictimObject = first
		event.Kind = LifeEndStunned
		if p.ID == 32788 {
			event.Kind = LifeStunned
			actor, err := r.Unsigned()
			if err != nil {
				return event, err
			}
			if err := ValidateObject(actor); err != nil {
				return event, err
			}
			event.ActorObject = actor
			for axis := 0; axis < 3; axis++ {
				if err := readCoordinate(r, -10000000); err != nil {
					return event, err
				}
			}
			if event.WeaponDef, err = r.Unsigned(); err != nil {
				return event, err
			}
		}
		return event, r.RequireEnd()
	case 32805, 32836, 32801:
		if err := ValidateObject(first); err != nil {
			return event, err
		}
		event.VictimObject = first
		switch p.ID {
		case 32801:
			event.Kind = LifeBubbleSettled
		case 32805:
			event.Kind = LifeDying
		default:
			event.Kind = LifeDead
		}
		return event, r.RequireEnd()
	}

	second, err := r.Unsigned()
	if err != nil {
		return event, err
	}
	if err := ValidateObject(first); err != nil {
		return event, err
	}
	if p.ID == 32806 {
		if err := r.RequireEnd(); err != nil {
			return event, err
		}
		if second != Unset && second != 0xFFFFFFFE {
			return event, fmt.Errorf("%w: self escape sentinel", ErrInvalidPacket)
		}
		event.Kind = LifeSelfPopOut
		event.VictimObject = first
		event.ActorObject = second
		return event, nil
	}
	if err := ValidateObject(second); err != nil {
		return event, err
	}
	event.VictimObject = first
	event.ActorObject = second
	if p.ID == 32807 {
		event.VictimObject = second
		event.ActorObject = first
	}

	switch p.ID {
	case 32787:
		for axis := 0; axis < 3; axis++ {
			if err := readCoordinate(r, -10000000); err != nil {
				return event, err
			}
		}
		if event.WeaponDef, err = r.Unsigned(); err != nil {
			return event, err
		}
		if event.DirectDeath, err = r.Boolean(); err != nil {
			return event, err
		}
		if _, err := r.Boolean(); err != nil {
			return event, err
		}
		tick, err := r.Unsigned()
		if err != nil {
			return event, err
		}
		event.Tick = tick & 0xFFFFFF
		authority, err := r.Boolean()
		if err != nil {
			return event, err
		}
		if authority {
			return event, fmt.Errorf("%w: ordinary rooms require victim-reported knockdown", ErrInvalidPacket)
		}
		event.Kind = LifeKnocked
	case 32793:
		event.Kind = LifeTryRescue
	case 32903:
		event.Kind = LifePopOut
		if err := readCoordinate(r, 0); err != nil {
			return event, err
		}
	default: // 32807
		event.Kind = LifeRescue
		if event.SpecialRescue, err = r.Boolean(); err != nil {
			return event, err
		}
	}
	return event, r.RequireEnd()
}

// ReadPeerLifePoint parses C32920.
func ReadPeerLifePoint(p Packet, sourceSlot uint8) (PeerLifePoint, error) {
	if p.ID != 32920 || sourceSlot >= 8 {
		return PeerLifePoint{}, fmt.Errorf("%w: unsupported life point packet", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var report PeerLifePoint
	var err error
	if report.CharacterObject, err = r.Unsigned(); err != nil {
		return report, err
	}
	if err := ValidateObject(report.CharacterObject); err != nil {
		return report, err
	}
	if report.RemainingPoints, err = r.Unsigned(); err != nil {
		return report, err
	}
	if report.Effect, err = r.Unsigned(); err != nil {
		return report, err
	}
	if err := r.RequireEnd(); err != nil {
		return report, err
	}
	if (report.CharacterObject-1000000)/100000 != uint32(sourceSlot) {
		return report, fmt.Errorf("%w: life point report sender mismatch", ErrInvalidPacket)
	}
	if report.RemainingPoints > math.MaxInt32 || report.Effect < 1 || report.Effect > 2 {
		return report, fmt.Errorf("%w: life point count or effect", ErrInvalidPacket)
	}
	return report, nil
}

// ReadPeerKick parses one peer kick packet.
func ReadPeerKick(p Packet) (PeerKickEvent, error) {
	if !IsPeerKick(p.ID) {
		return PeerKickEvent{}, fmt.Errorf("%w: unsupported peer kick %d", ErrInvalidPacket, p.ID)
	}
	r := NewReader(p.Payload)
	var event PeerKickEvent
	readObject := func() (uint32, error) {
		object, err := r.Unsigned()
		if err != nil {
			return 0, err
		}
		if err := ValidateObject(object); err != nil {
			return 0, err
		}
		return object, nil
	}

	switch p.ID {
	case 32815, 32816:
		if p.ID == 32815 {
			event.Kind = KickHumiliationStart
		} else {
			event.Kind = KickHumiliationEnd
		}
		actor, err := readObject()
		if err != nil {
			return event, err
		}
		victim, err := readObject()
		if err != nil {
			return event, err
		}
		event.Actor = actor
		event.Victim = victim
	case 32835:
		event.Kind = KickGotKicked
		victim, err := readObject()
		if err != nil {
			return event, err
		}
		event.Victim = victim
	case 32792, 32837:
		event.Kind = KickFailed
		if p.ID == 32837 {
			event.Kind = KickEnd
		}
		actor, err := readObject()
		if err != nil {
			return event, err
		}
		event.Actor = actor
	case 32791:
		event.Kind = KickStart
		actor, err := readObject()
		if err != nil {
			return event, err
		}
		victim, err := readObject()
		if err != nil {
			return event, err
		}
		style, err := r.Unsigned()
		if err != nil {
			return event, err
		}
		if style != 1 && style != 2 {
			return event, fmt.Errorf("%w: unsupported kick style", ErrInvalidPacket)
		}
		event.Actor = actor
		event.Victim = victim
		event.Style = style
	default:
		if p.ID == 32795 {
			event.Kind = KickConfirmed
			succeeded, err := r.Boolean()
			if err != nil {
				return event, err
			}
			event.Succeeded = succeeded
		} else if p.ID == 32796 {
			event.Kind = KickProtected
		} else {
			event.Kind = KickTry
		}
		victim, err := readObject()
		if err != nil {
			return event, err
		}
		actor, err := readObject()
		if err != nil {
			return event, err
		}
		event.Victim = victim
		event.Actor = actor
		if p.ID != 32794 {
			for axis := 0; axis < 3; axis++ {
				if err := readCoordinate(r, -10000000); err != nil {
					return event, err
				}
			}
		}
		if p.ID == 32795 {
			special, err := r.Boolean()
			if err != nil {
				return event, err
			}
			event.SpecialKick = special
		}
	}
	return event, r.RequireEnd()
}
