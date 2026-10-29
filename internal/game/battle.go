package game

import (
	"errors"
	"fmt"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
)

func (s *Server) handleRelayConnected(session *Session, packet protocol.Packet) error {
	r := protocol.NewReader(packet.Payload)
	if _, err := r.Boolean(); err != nil {
		return err
	}
	return r.RequireEnd()
}

func (s *Server) handleShopEntry(session *Session, packet protocol.Packet) error {
	category, err := protocol.ReadShopEntry(packet)
	if err != nil {
		return err
	}
	catalog, err := s.svc.ShopWire()
	if err != nil {
		return err
	}
	reply, err := protocol.CreateShopReply(catalog, category)
	if err != nil {
		return err
	}
	return session.Send(reply)
}

func (s *Server) handlePurchase(session *Session, packet protocol.Packet) error {
	request, err := protocol.ReadShopPurchase(packet)
	if err != nil {
		return err
	}
	if session.Character.ID == 0 {
		return session.Send(protocol.CreateShopPurchaseReply(request.StockID, protocol.PurchaseFailed))
	}
	result, err := s.svc.Purchase(session.Character.ID, request.StockID)
	if err != nil {
		s.log.Error("purchase failed", "character", session.Character.ID, "error", err)
		result = protocol.PurchaseFailed
	}
	if result == protocol.PurchaseOK {
		character, _ := session.Profile()
		items, err := s.svc.Items(character.ID)
		if err != nil {
			s.log.Error("inventory load failed after purchase", "character", character.ID, "error", err)
		}
		session.SetProfile(character, items)
		if refreshed, err := s.svc.Profile(session.Account.ID); err == nil {
			session.SetProfile(refreshed, nil)
			character = refreshed
		}
		cachedCharacter, cachedItems := session.Profile()
		if session.room != nil {
			session.room.SetProfile(session.slot, cachedCharacter, cachedItems)
		}
		if session.inPlaza {
			s.updatePlazaPresence(cachedCharacter.ID, cachedItems, cachedCharacter)
		}
		// The reference server refreshes stats, inventory and appearance before
		// acknowledging the purchase, so the dialog shows the new balance.
		if err := s.sendUserStatFor(session, cachedCharacter, cachedItems); err != nil {
			return err
		}
		wireItems := s.svc.WireItems(cachedItems)
		if reply, err := protocol.CreateInventoryReply(wireItems); err == nil {
			_ = session.Send(reply)
		}
		wire := s.svc.WireCharacter(cachedCharacter)
		if reply, err := protocol.CreateEquipmentReply(wire.AccountID, wire.CharacterType, wireItems); err == nil {
			_ = session.Send(reply)
		}
	}
	return session.Send(protocol.CreateShopPurchaseReply(request.StockID, result))
}

func (s *Server) handleEquipment(session *Session, packet protocol.Packet) error {
	itemID, err := protocol.ReadEquipmentRequest(packet)
	if err != nil {
		return err
	}
	equipped := packet.ID == 803
	if session.Character.ID == 0 {
		return session.Send(protocol.CreateEquipmentResult(equipped, false))
	}
	if err := s.svc.Equip(session.Character.ID, itemID, equipped); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.log.Debug("equipment failed", "character", session.Character.ID, "error", err)
		}
		return session.Send(protocol.CreateEquipmentResult(equipped, false))
	}

	// The reference server reloads the inventory after an equip request and
	// republishes it before acknowledging: S801 (the bag) then S805 (the
	// equipped list). Both the client's own appearance and its equipment panel
	// are built from S805, which resolves the worn character from the slot-63
	// item, so skipping them leaves the old character on screen even though the
	// row changed.
	cached, _ := session.Profile()
	cachedItems, err := s.svc.Items(cached.ID)
	if err != nil {
		return session.Send(protocol.CreateEquipmentResult(equipped, false))
	}
	session.SetProfile(cached, cachedItems)

	wire := s.svc.WireCharacter(cached)
	wireItems := s.svc.WireItems(cachedItems)
	if reply, err := protocol.CreateInventoryReply(wireItems); err == nil {
		if err := session.Send(reply); err != nil {
			return err
		}
	}
	if reply, err := protocol.CreateEquipmentReply(wire.AccountID, wire.CharacterType, wireItems); err == nil {
		if err := session.Send(reply); err != nil {
			return err
		}
	}

	// Refresh every published copy of the appearance, then the room seats, so
	// teammates see the new character too.
	if session.room != nil {
		if refreshed, err := s.svc.Profile(session.Account.ID); err == nil {
			session.SetProfile(refreshed, cachedItems)
			session.room.SetProfile(session.slot, refreshed, cachedItems)
		}
	}
	if session.inPlaza {
		if refreshed, err := s.svc.Profile(session.Account.ID); err == nil {
			session.SetProfile(refreshed, cachedItems)
			s.updatePlazaPresence(session.CharacterID(), cachedItems, refreshed)
		}
	}
	if session.room != nil {
		if reply, err := protocol.CreateRoomSlotReply(uint32(session.slot), s.members(session.room)[session.slot]); err == nil {
			session.room.Broadcast(reply)
		}
	}
	return session.Send(protocol.CreateEquipmentResult(equipped, true))
}

func (s *Server) handlePresetSelect(session *Session, packet protocol.Packet) error {
	index, err := protocol.ReadWeaponPresetRequest(packet)
	if err != nil {
		return err
	}
	if session.Character.ID == 0 {
		return session.Send(protocol.CreateWeaponPresetResult(index, false))
	}
	// Selecting a preset also equips the weapon it holds; an unavailable slot or
	// a weapon the worn character cannot use is refused instead of stored.
	presets, accepted, err := s.svc.SetPreset(session.Character.ID, index)
	if err != nil || !accepted {
		return session.Send(protocol.CreateWeaponPresetResult(index, false))
	}
	if err := session.Send(protocol.CreateWeaponPresetResult(index, true)); err != nil {
		return err
	}
	reply, err := protocol.CreateWeaponPresetsReply(session.Character.ID, presets)
	if err != nil {
		return err
	}
	return session.Send(reply)
}

func (s *Server) handlePing(session *Session, packet protocol.Packet) error {
	requester, err := protocol.ReadSessionPingRequest(packet)
	if err != nil {
		return err
	}
	reply, err := protocol.CreateSessionPingReply(protocol.SessionPingResponse{
		ResponderSlot:     uint32(session.slot),
		RequesterSlot:     requester,
		PerformanceSample: 0,
		StageSample:       0,
	})
	if err != nil {
		return err
	}
	return session.Send(reply)
}

func (s *Server) handlePlazaPosition(session *Session, packet protocol.Packet) error {
	position, err := protocol.ReadPlazaPosition(packet)
	if err != nil {
		return err
	}
	if session.Account.ID == 0 {
		return nil
	}
	// In the lobby the position must reach every other scene-ready member of
	// the same plaza instance, exactly like the original server's
	// HandlePlazaRequest (C511 -> S514 broadcast, sender excluded).
	if session.room == nil {
		if session.inPlaza {
			return s.handlePlazaMove(session, position)
		}
		reply, err := protocol.CreatePlazaPositionReply(session.Account.ID, position)
		if err != nil {
			return err
		}
		return session.Send(reply)
	}
	reply, err := protocol.CreatePlazaPositionReply(session.Account.ID, position)
	if err != nil {
		return err
	}
	room := session.room
	room.Each(func(slot int, other *Session) {
		if slot != session.slot {
			_ = other.Send(reply)
		}
	})
	return nil
}

func (s *Server) handleSessionInit(session *Session, packet protocol.Packet) error {
	if packet.ID != 521 || len(packet.Payload) != 0 {
		return fmt.Errorf("%w: expected empty SendSessionInit", protocol.ErrInvalidPacket)
	}
	reply, err := protocol.CreateSessionInitializationReply(session.Account.ID)
	if err != nil {
		return err
	}
	return session.Send(reply)
}

// handleBattleState intentionally ignores C2015/C2017: they belong to intrusion
// synchronization, and the original explicitly refuses to relay unsolicited
// state snapshots and never waits for these reports.
func (s *Server) handleBattleState(session *Session, packet protocol.Packet) error {
	return nil
}

func (s *Server) handleRespawn(session *Session, packet protocol.Packet) error {
	request, err := protocol.ReadRespawnRequest(packet)
	if err != nil {
		return err
	}
	room := session.room
	if room == nil {
		return nil
	}
	life := room.Life()
	if life == nil {
		return nil
	}
	objectID := objectFor(session.slot)
	if request.ObjectID != objectID {
		return fmt.Errorf("%w: respawn object mismatch", protocol.ErrInvalidPacket)
	}
	switch packet.ID {
	case 713, 32904:
		if err := life.RequestRespawn(uint8(session.slot), objectID); err != nil {
			return err
		}
		s.tryRespawn(room, session.slot)
	case 704:
		confirmed, err := life.ConfirmRespawn(uint8(session.slot), objectID, request.Value)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
		if _, err := life.CompleteRespawn(uint8(session.slot), objectID, request.Value); err != nil {
			return err
		}
		room.Broadcast(protocol.CreateRespawnReply(objectID, request.Value))
	}
	return nil
}

func (s *Server) handlePeerAction(session *Session, packet protocol.Packet) error {
	room := session.room
	if room == nil {
		return nil
	}
	if _, err := protocol.ReadPeerActionObject(packet, uint8(session.slot), s.weapons.DefinitionIDs()); err != nil {
		return err
	}
	room.Each(func(slot int, other *Session) {
		if slot == session.slot {
			return
		}
		_ = other.Send(packet)
	})
	return nil
}

// AddScore is kept for callers that already computed a score; life events use
// broadcastScores instead.
func (s *Server) AddScore(session *Session, score protocol.BattleScore, personal uint32) {
	room := session.room
	if room == nil || session.slot < 0 {
		return
	}
	reply, err := protocol.CreatePlayerScoreReply(uint8(session.slot), score, personal)
	if err == nil {
		room.Broadcast(reply)
	}
}

// handleInventoryOpen answers C802 with the inventory and equipped-item lists.
func (s *Server) handleInventoryOpen(session *Session, packet protocol.Packet) error {
	if len(packet.Payload) != 0 {
		return fmt.Errorf("%w: EnterInventory must have an empty payload", protocol.ErrInvalidPacket)
	}
	if session.Character.ID == 0 {
		return nil
	}
	cached, _ := session.Profile()
	cachedItems, err := s.svc.Items(cached.ID)
	if err != nil {
		s.log.Error("inventory load failed", "character", cached.ID, "error", err)
	}
	session.SetProfile(cached, cachedItems)
	items := s.svc.WireItems(session.Items)
	inventory, err := protocol.CreateInventoryReply(items)
	if err != nil {
		return err
	}
	if err := session.Send(inventory); err != nil {
		return err
	}
	character := s.svc.WireCharacter(session.Character)
	equipment, err := protocol.CreateEquipmentReply(character.AccountID, character.CharacterType, items)
	if err != nil {
		return err
	}
	if err := session.Send(equipment); err != nil {
		return err
	}
	presets, err := s.svc.Presets(session.Character.ID)
	if err != nil {
		return err
	}
	presetReply, err := protocol.CreateWeaponPresetsReply(session.Character.ID, presets)
	if err != nil {
		return err
	}
	return session.Send(presetReply)
}

// handleCashCheck answers C825 (ReqCheckCash) with the user statistics.
func (s *Server) handleCashCheck(session *Session, packet protocol.Packet) error {
	if len(packet.Payload) != 0 {
		return fmt.Errorf("%w: ReqCheckCash must have an empty payload", protocol.ErrInvalidPacket)
	}
	if session.Character.ID == 0 {
		return nil
	}
	session.Character, _ = s.svc.Profile(session.Account.ID)
	return s.sendUserStat(session)
}

// equippedWeaponDefinition returns the item id in a non-character equipment
// slot, which is the definition the client reports for near attacks.
func equippedWeaponDefinition(session *Session) uint32 {
	for _, item := range session.Items {
		if item.EquippedSlot != 0 && item.EquippedSlot != protocol.CharacterSlot {
			return item.ItemID
		}
	}
	return 0
}

// relayToRoom forwards a validated peer packet to the other room members. When
// destination is set only that slot receives it (bullet explosions address one
// victim).
func (s *Server) relayToRoom(session *Session, packet protocol.Packet, destination int) {
	room := session.room
	if room == nil {
		return
	}
	for _, member := range room.View() {
		if !member.Occupied || member.Slot == session.slot {
			continue
		}
		if destination >= 0 && member.Slot != destination {
			continue
		}
		_ = member.Session.Send(packet)
	}
}

// handlePeerDamage validates and relays the projectile reports.
func (s *Server) handlePeerDamage(session *Session, packet protocol.Packet) error {
	if session.room == nil {
		return nil
	}
	switch packet.ID {
	case 32777:
		hit, err := protocol.ReadPeerHit(packet)
		if err != nil {
			return err
		}
		s.relayToRoom(session, packet, int(hit.TargetSlot))
	case 32776, 32923:
		explosion, err := protocol.ReadPeerExplosion(packet)
		if err != nil {
			return err
		}
		destination := -1
		if explosion.TargetObject != 0 {
			destination = int(explosion.TargetSlot)
		}
		s.relayToRoom(session, packet, destination)
	case 32900:
		if _, err := protocol.ReadPeerDetonation(packet); err != nil {
			return err
		}
		s.relayToRoom(session, packet, -1)
	case 32906:
		if _, err := protocol.ReadPeerProjectileRemoval(packet); err != nil {
			return err
		}
		s.relayToRoom(session, packet, -1)
	}
	return nil
}

// handlePeerNear validates C32773 against the frozen equipped weapon.
func (s *Server) handlePeerNear(session *Session, packet protocol.Packet) error {
	if session.room == nil {
		return nil
	}
	if _, err := protocol.ReadPeerNearAttack(packet, uint8(session.slot), equippedWeaponDefinition(session)); err != nil {
		return err
	}
	s.relayToRoom(session, packet, -1)
	return nil
}

// handlePeerPing echoes C32840 back to the sender, matching the original relay of
// the untouched packet.
func (s *Server) handlePeerPing(session *Session, packet protocol.Packet) error {
	if _, err := protocol.ReadPeerPing(packet, uint8(session.slot)); err != nil {
		return err
	}
	return session.Send(packet)
}

// handleHealthReport accepts the client's diagnostic health reports. The
// original never derives HP or life state from them, so they are validated and
// dropped.
func (s *Server) handleHealthReport(session *Session, packet protocol.Packet) error {
	r := protocol.NewReader(packet.Payload)
	switch packet.ID {
	case 18: // bounded diagnostics trace
		entries, err := protocol.ReadHealthTraceReport(packet)
		if err != nil {
			return err
		}
		s.log.Debug("health trace report", "session", session.ID, "entries", len(entries))
		return nil
	case 16: // recovery report during a match
		objectID, err := r.Unsigned()
		if err != nil {
			return err
		}
		kind, err := r.Byte()
		if err != nil {
			return err
		}
		if _, err := r.Unsigned(); err != nil {
			return err
		}
		if err := r.RequireEnd(); err != nil {
			return err
		}
		if objectID < 1000000 || objectID >= 1800000 ||
			(objectID-1000000)/100000 != uint32(session.slot) || kind > 13 {
			return fmt.Errorf("%w: health recovery partition", protocol.ErrInvalidPacket)
		}
		return nil
	case 14: // health reset report during a match
		objectID, err := r.Unsigned()
		if err != nil {
			return err
		}
		if _, err := r.Unsigned(); err != nil { // low-24-bit clock
			return err
		}
		if err := r.RequireEnd(); err != nil {
			return err
		}
		if objectID < 1000000 || objectID >= 1800000 ||
			(objectID-1000000)/100000 != uint32(session.slot) {
			return fmt.Errorf("%w: health reset partition", protocol.ErrInvalidPacket)
		}
	}
	return nil
}

// handleRelayPeerUpdate accepts C8/C9. The deployment is relay-only, so the
// direct-peer bookkeeping is validated and discarded.
func (s *Server) handleRelayPeerUpdate(session *Session, packet protocol.Packet) error {
	peer, err := protocol.ReadRelayPeerRequest(packet)
	if err != nil {
		return err
	}
	room := session.room
	if room == nil {
		return nil
	}
	for _, member := range room.View() {
		if member.Occupied && member.Slot != session.slot && objectFor(member.Slot) == peer {
			return nil
		}
	}
	return fmt.Errorf("%w: relay peer is outside the current game", protocol.ErrInvalidPacket)
}
