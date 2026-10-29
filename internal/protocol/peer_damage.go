package protocol

import (
	"fmt"
	"math"
)

// PeerHit is C32777 (ordinary bullet hit).
type PeerHit struct {
	ProjectileID uint32
	TargetObject uint32
	TargetSlot   uint8
}

// IsPeerExplosion reports whether the id is a bullet explosion packet.
func IsPeerExplosion(id uint16) bool { return id == 32776 || id == 32923 }

// PeerExplosion is C32776 / C32923.
type PeerExplosion struct {
	ProjectileID uint32
	TargetObject uint32
	TargetSlot   uint8
	ApplyDamage  bool
}

// PeerDetonation is C32900 (grenade detonation).
type PeerDetonation struct {
	ProjectileID uint32
	SplashID     uint32
}

func readCollisionCoordinate(r *Reader) error {
	bits, err := r.Unsigned()
	if err != nil {
		return err
	}
	value := math.Float32frombits(bits)
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) ||
		math.Abs(float64(value)) > 10000000 {
		return fmt.Errorf("%w: collision coordinate", ErrInvalidPacket)
	}
	return nil
}

// ReadPeerHit parses C32777.
func ReadPeerHit(p Packet) (PeerHit, error) {
	if p.ID != 32777 {
		return PeerHit{}, fmt.Errorf("%w: unsupported peer hit", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var hit PeerHit
	var err error
	if hit.ProjectileID, err = r.Unsigned(); err != nil {
		return hit, err
	}
	if hit.TargetObject, err = r.Unsigned(); err != nil {
		return hit, err
	}
	if hit.ProjectileID >= 800000 || hit.TargetObject < 1000000 ||
		(hit.TargetObject-1000000)/100000 >= 8 {
		return hit, fmt.Errorf("%w: hit identifier partition", ErrInvalidPacket)
	}
	hit.TargetSlot = uint8((hit.TargetObject - 1000000) / 100000)
	for i := 0; i < 3; i++ {
		if err := readCollisionCoordinate(r); err != nil {
			return hit, err
		}
	}
	return hit, r.RequireEnd()
}

// ReadPeerExplosion parses C32776 / C32923.
func ReadPeerExplosion(p Packet) (PeerExplosion, error) {
	if !IsPeerExplosion(p.ID) {
		return PeerExplosion{}, fmt.Errorf("%w: unsupported explosion", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var explosion PeerExplosion
	var err error
	if explosion.ProjectileID, err = r.Unsigned(); err != nil {
		return explosion, err
	}
	target, err := r.Unsigned()
	if err != nil {
		return explosion, err
	}
	if explosion.ProjectileID >= 800000 {
		return explosion, fmt.Errorf("%w: exploded projectile partition", ErrInvalidPacket)
	}
	// The original destroys the bullet without a victim when the target is -1.
	if target != Unset {
		if target < 1000000 || (target-1000000)/100000 >= 8 {
			return explosion, fmt.Errorf("%w: explosion target partition", ErrInvalidPacket)
		}
		explosion.TargetObject = target
		explosion.TargetSlot = uint8((target - 1000000) / 100000)
	}
	for axis := 0; axis < 3; axis++ {
		if p.ID == 32923 {
			half, err := r.Short()
			if err != nil {
				return explosion, err
			}
			if half&0x7C00 == 0x7C00 {
				return explosion, fmt.Errorf("%w: nonfinite half coordinate", ErrInvalidPacket)
			}
			continue
		}
		if err := readCollisionCoordinate(r); err != nil {
			return explosion, err
		}
	}
	if explosion.ApplyDamage, err = r.Boolean(); err != nil {
		return explosion, err
	}
	return explosion, r.RequireEnd()
}

// ReadPeerDetonation parses C32900.
func ReadPeerDetonation(p Packet) (PeerDetonation, error) {
	if p.ID != 32900 {
		return PeerDetonation{}, fmt.Errorf("%w: unsupported detonation", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var detonation PeerDetonation
	var err error
	if detonation.ProjectileID, err = r.Unsigned(); err != nil {
		return detonation, err
	}
	for component := 0; component < 9; component++ {
		if err := readCollisionCoordinate(r); err != nil {
			return detonation, err
		}
	}
	if detonation.SplashID, err = r.Unsigned(); err != nil {
		return detonation, err
	}
	if _, err := r.Unsigned(); err != nil { // low-24-bit clock, upper byte unspecified
		return detonation, err
	}
	if err := r.RequireEnd(); err != nil {
		return detonation, err
	}
	if detonation.ProjectileID >= 800000 || detonation.SplashID >= 800000 ||
		detonation.ProjectileID/100000 != detonation.SplashID/100000 {
		return detonation, fmt.Errorf("%w: detonation objects", ErrInvalidPacket)
	}
	return detonation, nil
}

// ReadPeerProjectileRemoval parses C32906.
func ReadPeerProjectileRemoval(p Packet) (uint32, error) {
	if p.ID != 32906 {
		return 0, fmt.Errorf("%w: unsupported projectile removal", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	projectileID, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	if projectileID >= 800000 {
		return 0, fmt.Errorf("%w: removed projectile partition", ErrInvalidPacket)
	}
	return projectileID, r.RequireEnd()
}

// PeerNearAttack is C32773 (near weapon damage object).
type PeerNearAttack struct {
	SourceObject uint32
	WeaponDef    uint32
	DamagerID    uint32
}

// ReadPeerNearAttack parses C32773.
func ReadPeerNearAttack(p Packet, sourceSlot uint8, equippedDefinition uint32) (PeerNearAttack, error) {
	if p.ID != 32773 {
		return PeerNearAttack{}, fmt.Errorf("%w: unsupported near attack", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var attack PeerNearAttack
	var err error
	if attack.SourceObject, err = r.Unsigned(); err != nil {
		return attack, err
	}
	weapon, err := r.Short()
	if err != nil {
		return attack, err
	}
	attack.WeaponDef = uint32(weapon)
	if _, err := r.Byte(); err != nil { // Character +408 attribute
		return attack, err
	}
	if attack.DamagerID, err = r.Unsigned(); err != nil {
		return attack, err
	}
	if _, err := r.Unsigned(); err != nil { // low-24-bit clock
		return attack, err
	}
	if err := r.RequireEnd(); err != nil {
		return attack, err
	}
	if sourceSlot >= 8 || attack.SourceObject < 1000000 ||
		(attack.SourceObject-1000000)/100000 != uint32(sourceSlot) ||
		attack.DamagerID/100000 != uint32(sourceSlot) {
		return attack, fmt.Errorf("%w: near weapon partition", ErrInvalidPacket)
	}
	if attack.WeaponDef == 0 || attack.WeaponDef != equippedDefinition {
		return attack, fmt.Errorf("%w: near weapon is not the equipped one", ErrInvalidPacket)
	}
	return attack, nil
}

// PeerPing is C32840 (peer latency sample); the server echoes it unchanged.
type PeerPing struct {
	Slot        uint32
	Performance uint32
}

// ReadPeerPing parses C32840 and validates the sender slot.
func ReadPeerPing(p Packet, sourceSlot uint8) (PeerPing, error) {
	if p.ID != 32840 || sourceSlot >= 8 {
		return PeerPing{}, fmt.Errorf("%w: unsupported peer ping", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var ping PeerPing
	var err error
	if ping.Slot, err = r.Unsigned(); err != nil {
		return ping, err
	}
	if ping.Performance, err = r.Unsigned(); err != nil {
		return ping, err
	}
	if err := r.RequireEnd(); err != nil {
		return ping, err
	}
	if ping.Slot != uint32(sourceSlot) {
		return ping, fmt.Errorf("%w: peer ping slot mismatch", ErrInvalidPacket)
	}
	return ping, nil
}

// ReadRelayPeerRequest parses C8/C9 (direct-peer link update).
func ReadRelayPeerRequest(p Packet) (uint32, error) {
	if p.ID != 8 && p.ID != 9 {
		return 0, fmt.Errorf("%w: expected relay peer request", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	peer, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	return peer, r.RequireEnd()
}

// HealthTraceEntry is one packed 13-byte diagnostic record.
type HealthTraceEntry struct {
	Kind            uint8
	Source          uint32
	Amount          float32
	ResultingHealth float32
}

// ReadHealthTraceReport parses C18 (count followed by 13-byte records).
func ReadHealthTraceReport(p Packet) ([]HealthTraceEntry, error) {
	if p.ID != 18 {
		return nil, fmt.Errorf("%w: expected health trace report", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	count, err := r.Unsigned()
	if err != nil {
		return nil, err
	}
	if r.Remaining()%13 != 0 || int(count) != r.Remaining()/13 {
		return nil, fmt.Errorf("%w: health trace count", ErrInvalidPacket)
	}
	entries := make([]HealthTraceEntry, 0, count)
	for i := uint32(0); i < count; i++ {
		var entry HealthTraceEntry
		if entry.Kind, err = r.Byte(); err != nil {
			return nil, err
		}
		if entry.Source, err = r.Unsigned(); err != nil {
			return nil, err
		}
		amountBits, err := r.Unsigned()
		if err != nil {
			return nil, err
		}
		healthBits, err := r.Unsigned()
		if err != nil {
			return nil, err
		}
		entry.Amount = math.Float32frombits(amountBits)
		entry.ResultingHealth = math.Float32frombits(healthBits)
		if math.IsNaN(float64(entry.Amount)) || math.IsInf(float64(entry.Amount), 0) ||
			math.IsNaN(float64(entry.ResultingHealth)) || math.IsInf(float64(entry.ResultingHealth), 0) {
			return nil, fmt.Errorf("%w: health trace floats", ErrInvalidPacket)
		}
		entries = append(entries, entry)
	}
	return entries, r.RequireEnd()
}

// CreateHealthTraceRequest builds S8 (no payload).
func CreateHealthTraceRequest() Packet { return Packet{ID: 8} }
