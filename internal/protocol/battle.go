package protocol

import (
	"fmt"
	"math"
)

// CreateRoomResult builds S102/S103.
func CreateRoomResult(creating bool, result uint32) Packet {
	w := NewWriter()
	w.Unsigned(result)
	id := uint16(103)
	if creating {
		id = 102
	}
	return Packet{ID: id, Payload: w.Payload()}
}

// CreateRoomEntryReply builds S110.
func CreateRoomEntryReply(setting SessionSetting, masterSlot, ownSlot uint32, members []SessionMember) (Packet, error) {
	if len(members) == 0 || len(members) > 32 || int(masterSlot) >= len(members) ||
		int(ownSlot) >= len(members) || members[masterSlot].Character == nil ||
		members[ownSlot].Character == nil {
		return Packet{}, fmt.Errorf("%w: room entry slots", ErrInvalidPacket)
	}
	w := NewWriter()
	if err := WriteSessionSetting(w, setting); err != nil {
		return Packet{}, err
	}
	w.Unsigned(masterSlot)
	w.Unsigned(ownSlot)
	w.Unsigned(uint32(len(members)))
	for slot, member := range members {
		if err := WriteSessionMember(w, member, uint32(slot)); err != nil {
			return Packet{}, err
		}
	}
	occupied := uint32(0)
	for _, member := range members {
		if member.Character != nil {
			occupied++
		}
	}
	w.Unsigned(occupied)
	for _, member := range members {
		if member.Character == nil {
			continue
		}
		reply, err := CreateEquipmentReply(member.Character.AccountID,
			member.Character.CharacterType, member.Items)
		if err != nil {
			return Packet{}, err
		}
		w.Bytes(reply.Payload)
	}
	w.Unsigned(0)
	w.Unsigned(0)
	return Packet{ID: 110, Payload: w.Payload()}, nil
}

// CreateMainMenuReply builds the S110 main-menu stage.
func CreateMainMenuReply(character Character, items []InvItem) (Packet, error) {
	setting := DefaultSessionSetting()
	setting.Kind = 1
	members := []SessionMember{{Character: &character, Items: items, Team: 1}}
	return CreateRoomEntryReply(setting, 0, 0, members)
}

// CreateSoloPartyReply builds S201.
func CreateSoloPartyReply(character Character, items []InvItem) (Packet, error) {
	equipment, err := CreateEquipmentReply(character.AccountID, character.CharacterType, items)
	if err != nil {
		return Packet{}, err
	}
	w := NewWriter()
	w.Unsigned(2)
	w.Unsigned(1)
	w.Unsigned(character.AccountID)
	w.String(character.Nickname)
	w.Unsigned(Unset)
	w.Unsigned(Unset)
	w.Unsigned(0)
	w.Unsigned(1)
	w.Bytes(equipment.Payload)
	return Packet{ID: 201, Payload: w.Payload()}, nil
}

// CreateRoomSettingReply builds S501.
func CreateRoomSettingReply(setting SessionSetting) (Packet, error) {
	w := NewWriter()
	if err := WriteSessionSetting(w, setting); err != nil {
		return Packet{}, err
	}
	return Packet{ID: 501, Payload: w.Payload()}, nil
}

// CreateRoomGameSettingReply builds S505.
func CreateRoomGameSettingReply(setting GameSetting) (Packet, error) {
	w := NewWriter()
	if err := WriteGameSetting(w, setting); err != nil {
		return Packet{}, err
	}
	return Packet{ID: 505, Payload: w.Payload()}, nil
}

// CreateRoomMasterReply builds S502.
func CreateRoomMasterReply(slot uint32) (Packet, error) {
	if slot >= 32 {
		return Packet{}, fmt.Errorf("%w: master slot", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(slot)
	return Packet{ID: 502, Payload: w.Payload()}, nil
}

// CreateRoomSlotReply builds S503.
func CreateRoomSlotReply(slot uint32, member SessionMember) (Packet, error) {
	if slot >= 32 || member.Character == nil {
		return Packet{}, fmt.Errorf("%w: room set slot", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(slot)
	if err := WriteSessionMember(w, member, slot); err != nil {
		return Packet{}, err
	}
	equipment, err := CreateEquipmentReply(member.Character.AccountID,
		member.Character.CharacterType, member.Items)
	if err != nil {
		return Packet{}, err
	}
	w.Bytes(equipment.Payload)
	return Packet{ID: 503, Payload: w.Payload()}, nil
}

// CreateRoomDepartureReply builds S511.
func CreateRoomDepartureReply(slot uint32) (Packet, error) {
	if slot >= 32 {
		return Packet{}, fmt.Errorf("%w: departure slot", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(slot)
	if err := WriteSessionMember(w, SessionMember{}, slot); err != nil {
		return Packet{}, err
	}
	return Packet{ID: 511, Payload: w.Payload()}, nil
}

// CreateRoomTeamReply builds S504.
func CreateRoomTeamReply(slot uint32, team uint8) (Packet, error) {
	if slot >= 32 || (team != 1 && team != 2) {
		return Packet{}, fmt.Errorf("%w: room team", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(slot)
	w.Unsigned(uint32(team))
	return Packet{ID: 504, Payload: w.Payload()}, nil
}

// RoomKickResult is the S513 result code.
type RoomKickResult uint32

// Kick results.
const (
	KickSuccess     RoomKickResult = 0
	KickSelf        RoomKickResult = 2
	KickNotInRoom   RoomKickResult = 3
	KickUnavailable RoomKickResult = 5
)

// CreateRoomKickReply builds S513.
func CreateRoomKickReply(requester, target []uint16, result RoomKickResult) Packet {
	w := NewWriter()
	w.String(requester)
	w.String(target)
	w.Unsigned(uint32(result))
	return Packet{ID: 513, Payload: w.Payload()}
}

// ReadyStatus is C505.
type ReadyStatus struct {
	Ready         bool
	ActivityFlags [2]bool
}

// ReadReadyStatus parses C505.
func ReadReadyStatus(p Packet) (ReadyStatus, error) {
	if p.ID != 505 {
		return ReadyStatus{}, fmt.Errorf("%w: expected RoomReqReadyStatus", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var status ReadyStatus
	var err error
	if status.Ready, err = r.Boolean(); err != nil {
		return status, err
	}
	if status.ActivityFlags[0], err = r.Boolean(); err != nil {
		return status, err
	}
	if status.ActivityFlags[1], err = r.Boolean(); err != nil {
		return status, err
	}
	if err := r.RequireEnd(); err != nil {
		return status, err
	}
	active := 0
	if status.Ready {
		active++
	}
	if status.ActivityFlags[0] {
		active++
	}
	if status.ActivityFlags[1] {
		active++
	}
	if active > 1 {
		return status, fmt.Errorf("%w: activity indicators are exclusive", ErrInvalidPacket)
	}
	return status, nil
}

// CreateJoinRoomRequest parses C403.
type JoinRoomRequest struct {
	RoomID   uint32
	Value8   uint32
	Value12  uint32
	Password []uint16
	Value44  uint32
}

// ReadJoinRoomRequest parses C403.
func ReadJoinRoomRequest(p Packet) (JoinRoomRequest, error) {
	if p.ID != 403 {
		return JoinRoomRequest{}, fmt.Errorf("%w: expected ReqJoinSession", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var req JoinRoomRequest
	var err error
	if req.RoomID, err = r.Unsigned(); err != nil {
		return req, err
	}
	if req.Value8, err = r.Unsigned(); err != nil {
		return req, err
	}
	if req.Value12, err = r.Unsigned(); err != nil {
		return req, err
	}
	if req.Password, err = r.String(SessionStringLimit); err != nil {
		return req, err
	}
	if req.Value44, err = r.Unsigned(); err != nil {
		return req, err
	}
	return req, r.RequireEnd()
}

// CreateRoomRequest is C402.
type CreateRoomRequest struct {
	Setting   SessionSetting
	String600 []uint16
	Flag628   bool
}

// ReadCreateRoomRequest parses C402.
func ReadCreateRoomRequest(p Packet) (CreateRoomRequest, error) {
	if p.ID != 402 {
		return CreateRoomRequest{}, fmt.Errorf("%w: expected ReqCreateSession", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var req CreateRoomRequest
	var err error
	if req.Setting, err = ReadSessionSetting(r); err != nil {
		return req, err
	}
	if req.String600, err = r.String(SessionStringLimit); err != nil {
		return req, err
	}
	if req.Flag628, err = r.Boolean(); err != nil {
		return req, err
	}
	return req, r.RequireEnd()
}

// CreateGameEntryReply builds S510.
func CreateGameEntryReply(result uint32, gameMap []uint16, state uint32) Packet {
	w := NewWriter()
	w.Unsigned(result)
	w.String(gameMap)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(state)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	return Packet{ID: 510, Payload: w.Payload()}
}

// Game start result codes.
const (
	StartSuccess     uint32 = 0
	StartNotReady    uint32 = 1
	StartUnequal     uint32 = 2
	StartUnavailable uint32 = 3
	StartAlone       uint32 = 4
)

// CreateSessionSignatureReply builds S522.
func CreateSessionSignatureReply(mode uint32) (Packet, error) {
	if mode == 0 || mode > math.MaxUint16 {
		return Packet{}, fmt.Errorf("%w: session signature mode", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Short(uint16(mode))
	w.Byte(0)
	w.Unsigned(0)
	return Packet{ID: 522, Payload: w.Payload()}, nil
}

// CreatePeerIdentityReply builds S607.
func CreatePeerIdentityReply(peerID uint32) Packet {
	w := NewWriter()
	w.Short(0)
	w.Unsigned(peerID)
	w.Unsigned(0)
	return Packet{ID: 607, Payload: w.Payload()}
}

// CreatePeerTableReply builds S610.
func CreatePeerTableReply(slot, peerID uint32) (Packet, error) {
	if slot >= 8 || peerID == 0 || peerID == Unset {
		return Packet{}, fmt.Errorf("%w: peer table identity", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(slot)
	w.Unsigned(peerID)
	w.Unsigned(0)
	w.Short(0)
	w.Unsigned(0)
	w.Short(0)
	return Packet{ID: 610, Payload: w.Payload()}, nil
}

// CreateMapLoadedReply builds S701.
func CreateMapLoadedReply(slot uint32) (Packet, error) {
	if slot >= 8 {
		return Packet{}, fmt.Errorf("%w: map loading slot", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(slot)
	return Packet{ID: 701, Payload: w.Payload()}, nil
}

// CreateEverybodyLoadedReply builds S702.
func CreateEverybodyLoadedReply() Packet {
	w := NewWriter()
	w.Unsigned(1)
	w.Unsigned(0)
	w.Unsigned(0)
	return Packet{ID: 702, Payload: w.Payload()}
}

// CreateGameCountdownReply builds S703.
func CreateGameCountdownReply(milliseconds uint32) Packet {
	w := NewWriter()
	w.Unsigned(milliseconds)
	return Packet{ID: 703, Payload: w.Payload()}
}

// CreateGameStartReply builds S704.
func CreateGameStartReply(durationMilliseconds, trackingID uint32) Packet {
	w := NewWriter()
	w.Unsigned(durationMilliseconds)
	w.Unsigned(0)
	w.Unsigned(trackingID)
	return Packet{ID: 704, Payload: w.Payload()}
}

// CreateGameEndReply builds S705.
func CreateGameEndReply(timeExpired bool) Packet {
	w := NewWriter()
	w.Unsigned(0)
	if timeExpired {
		w.Unsigned(2)
	} else {
		w.Unsigned(1)
	}
	return Packet{ID: 705, Payload: w.Payload()}
}

// CreateGameWinnerReply builds S732.
func CreateGameWinnerReply(firstTeam, secondTeam uint16) (Packet, error) {
	if firstTeam > 200 || secondTeam > 200 {
		return Packet{}, fmt.Errorf("%w: final team score", ErrInvalidPacket)
	}
	return Packet{ID: 732, Payload: []byte{byte(firstTeam), byte(secondTeam)}}, nil
}

// RelayKey is the opaque 20-byte relay session key echoed by the client.
type RelayKey [20]byte

// RelayRegistration is C1.
type RelayRegistration struct {
	Key    RelayKey
	Slot   uint8
	PeerID uint32
}

// ReadRelayRegistration parses C1.
func ReadRelayRegistration(p Packet) (RelayRegistration, error) {
	if p.ID != 1 {
		return RelayRegistration{}, fmt.Errorf("%w: expected ReqJoinRelaySession", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var reg RelayRegistration
	for i := range reg.Key {
		b, err := r.Byte()
		if err != nil {
			return reg, err
		}
		reg.Key[i] = b
	}
	var err error
	if reg.Slot, err = r.Byte(); err != nil {
		return reg, err
	}
	if reg.PeerID, err = r.Unsigned(); err != nil {
		return reg, err
	}
	return reg, r.RequireEnd()
}

// CreateRelayEndpointReply builds S601.
func CreateRelayEndpointReply(address [4]byte, port uint16, key RelayKey) (Packet, error) {
	if port == 0 {
		return Packet{}, fmt.Errorf("%w: relay port", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Bytes(address[:])
	w.Short(port)
	w.Bytes(key[:])
	return Packet{ID: 601, Payload: w.Payload()}, nil
}

// CreateRelayRegistrationReply builds C601.
func CreateRelayRegistrationReply() Packet {
	return Packet{ID: 13}
}

// CreateRelayLagReply builds S21.
func CreateRelayLagReply(p Packet) (Packet, error) {
	if p.ID != 20006 {
		return Packet{}, fmt.Errorf("%w: expected ReqLagDisPing", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	value, err := r.Unsigned()
	if err != nil {
		return Packet{}, err
	}
	if err := r.RequireEnd(); err != nil {
		return Packet{}, err
	}
	w := NewWriter()
	w.Unsigned(value)
	return Packet{ID: 21, Payload: w.Payload()}, nil
}

// PlazaPosition is C511.
type PlazaPosition struct {
	Sequence    uint32
	Coordinates [3]float32
	InputFlags  [2]byte
	Heading     float32
}

// ReadPlazaPosition parses C511.
func ReadPlazaPosition(p Packet) (PlazaPosition, error) {
	if p.ID != 511 {
		return PlazaPosition{}, fmt.Errorf("%w: expected SendSessionChrPos", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var pos PlazaPosition
	var err error
	if pos.Sequence, err = r.Unsigned(); err != nil {
		return pos, err
	}
	for i := range pos.Coordinates {
		bits, err := r.Unsigned()
		if err != nil {
			return pos, err
		}
		value := math.Float32frombits(bits)
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) ||
			math.Abs(float64(value)) > 10000000 {
			return pos, fmt.Errorf("%w: plaza coordinate", ErrInvalidPacket)
		}
		pos.Coordinates[i] = value
	}
	for i := range pos.InputFlags {
		if pos.InputFlags[i], err = r.Byte(); err != nil {
			return pos, err
		}
	}
	bits, err := r.Unsigned()
	if err != nil {
		return pos, err
	}
	pos.Heading = math.Float32frombits(bits)
	return pos, r.RequireEnd()
}

// CreatePlazaPositionReply builds S514.
func CreatePlazaPositionReply(accountID uint32, pos PlazaPosition) (Packet, error) {
	if accountID == 0 {
		return Packet{}, fmt.Errorf("%w: plaza identity", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(accountID)
	w.Unsigned(pos.Sequence)
	for _, coordinate := range pos.Coordinates {
		w.Unsigned(math.Float32bits(coordinate))
	}
	for _, flag := range pos.InputFlags {
		w.Byte(flag)
	}
	w.Unsigned(math.Float32bits(pos.Heading))
	return Packet{ID: 514, Payload: w.Payload()}, nil
}

// CreateSessionInitializationReply builds S524.
func CreateSessionInitializationReply(accountID uint32) (Packet, error) {
	if accountID == 0 {
		return Packet{}, fmt.Errorf("%w: session init identity", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(accountID)
	return Packet{ID: 524, Payload: w.Payload()}, nil
}

// BattleScore is one player's score row.
type BattleScore struct {
	BubbleCount uint16
	PangCount   uint16
	RescueCount uint16
}

// CreatePlayerScoreReply builds S986 for a player row.
func CreatePlayerScoreReply(slot uint8, score BattleScore, personal uint32) (Packet, error) {
	if slot >= 8 {
		return Packet{}, fmt.Errorf("%w: score slot", ErrInvalidPacket)
	}
	if personal > 65535 {
		personal = 65535
	}
	w := NewWriter()
	w.Byte(0)
	w.Short(0xFFFF)
	w.Byte(slot)
	w.Byte(0xFF)
	w.Unsigned(4)
	w.Byte(0)
	w.Short(score.BubbleCount)
	w.Byte(24)
	w.Short(score.PangCount)
	w.Byte(10)
	w.Short(score.RescueCount)
	w.Byte(29)
	w.Short(uint16(personal))
	w.Unsigned(0)
	return Packet{ID: 986, Payload: w.Payload()}, nil
}

// CreateTeamScoreReply builds S986 for a team row.
func CreateTeamScoreReply(team uint8, score uint16) (Packet, error) {
	if team < 1 || team > 2 || score > 200 {
		return Packet{}, fmt.Errorf("%w: team score", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Byte(team)
	w.Short(score)
	w.Byte(0xFF)
	w.Byte(0xFF)
	w.Unsigned(0)
	w.Unsigned(0)
	return Packet{ID: 986, Payload: w.Payload()}, nil
}

// RespawnRequest is C713/C704/C32904/C32925.
type RespawnRequest struct {
	ObjectID uint32
	Value    uint32
}

// ReadRespawnRequest parses the respawn requests.
func ReadRespawnRequest(p Packet) (RespawnRequest, error) {
	switch p.ID {
	case 713, 704, 32904, 32925:
	default:
		return RespawnRequest{}, fmt.Errorf("%w: unsupported respawn request", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var req RespawnRequest
	var err error
	if req.ObjectID, err = r.Unsigned(); err != nil {
		return req, err
	}
	if req.Value, err = r.Unsigned(); err != nil {
		return req, err
	}
	if req.ObjectID < 1000000 || (req.ObjectID-1000000)/100000 >= 8 {
		return req, fmt.Errorf("%w: respawn object", ErrInvalidPacket)
	}
	if (p.ID == 713 || p.ID == 32904) && req.Value > 60000 {
		return req, fmt.Errorf("%w: respawn delay", ErrInvalidPacket)
	}
	if p.ID == 704 {
		if _, err := r.Boolean(); err != nil {
			return req, err
		}
	}
	return req, r.RequireEnd()
}

// CreateRespawnOffer builds S708.
func CreateRespawnOffer(objectID, spawnIndex uint32) Packet {
	w := NewWriter()
	w.Unsigned(objectID)
	w.Unsigned(spawnIndex)
	w.Unsigned(1)
	return Packet{ID: 708, Payload: w.Payload()}
}

// CreateRespawnReply builds S707.
func CreateRespawnReply(objectID, spawnIndex uint32) Packet {
	w := NewWriter()
	w.Unsigned(objectID)
	w.Unsigned(spawnIndex)
	w.Unsigned(1)
	w.Unsigned(0)
	return Packet{ID: 707, Payload: w.Payload()}
}

// SessionPingResponse is C518.
type SessionPingResponse struct {
	ResponderSlot     uint32
	RequesterSlot     uint32
	PerformanceSample uint32
	StageSample       float32
}

// ReadSessionPingRequest parses C517.
func ReadSessionPingRequest(p Packet) (uint32, error) {
	if p.ID != 517 {
		return 0, fmt.Errorf("%w: expected ReqPingUser", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	slot, err := r.Unsigned()
	if err != nil {
		return 0, err
	}
	if slot >= 32 {
		return 0, fmt.Errorf("%w: ping slot", ErrInvalidPacket)
	}
	return slot, r.RequireEnd()
}

// ReadSessionPingResponse parses C518.
func ReadSessionPingResponse(p Packet) (SessionPingResponse, error) {
	if p.ID != 518 {
		return SessionPingResponse{}, fmt.Errorf("%w: expected RepPingUser", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var resp SessionPingResponse
	var err error
	if resp.ResponderSlot, err = r.Unsigned(); err != nil {
		return resp, err
	}
	if resp.RequesterSlot, err = r.Unsigned(); err != nil {
		return resp, err
	}
	if resp.PerformanceSample, err = r.Unsigned(); err != nil {
		return resp, err
	}
	bits, err := r.Unsigned()
	if err != nil {
		return resp, err
	}
	resp.StageSample = math.Float32frombits(bits)
	if resp.ResponderSlot >= 32 || resp.RequesterSlot >= 32 {
		return resp, fmt.Errorf("%w: ping slots", ErrInvalidPacket)
	}
	return resp, r.RequireEnd()
}

// CreateSessionPingRequest builds S517.
func CreateSessionPingRequest(requesterSlot uint32) (Packet, error) {
	if requesterSlot >= 32 {
		return Packet{}, fmt.Errorf("%w: ping slot", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(requesterSlot)
	return Packet{ID: 517, Payload: w.Payload()}, nil
}

// CreateSessionPingReply builds S518.
func CreateSessionPingReply(resp SessionPingResponse) (Packet, error) {
	if resp.ResponderSlot >= 32 || resp.RequesterSlot >= 32 {
		return Packet{}, fmt.Errorf("%w: ping slots", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(resp.ResponderSlot)
	w.Unsigned(resp.PerformanceSample)
	w.Unsigned(math.Float32bits(resp.StageSample))
	return Packet{ID: 518, Payload: w.Payload()}, nil
}

// GameStartRequest is C503 (RoomReqEnterGame).
type GameStartRequest struct {
	Flag4      bool
	Strings8   [3][]uint16
	Difficulty uint32
	Values100  [13]uint32
	EventFlags [5]bool
	Value156   uint32
}

// ReadGameStartRequest parses C503.
func ReadGameStartRequest(p Packet) (GameStartRequest, error) {
	if p.ID != 503 {
		return GameStartRequest{}, fmt.Errorf("%w: expected RoomReqEnterGame", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	var req GameStartRequest
	var err error
	if req.Flag4, err = r.Boolean(); err != nil {
		return req, err
	}
	for i := range req.Strings8 {
		if req.Strings8[i], err = r.String(SessionStringLimit); err != nil {
			return req, err
		}
	}
	if req.Difficulty, err = r.Unsigned(); err != nil {
		return req, err
	}
	for i := range req.Values100 {
		if req.Values100[i], err = r.Unsigned(); err != nil {
			return req, err
		}
	}
	for i := 0; i < 3; i++ {
		if req.EventFlags[i], err = r.Boolean(); err != nil {
			return req, err
		}
	}
	if req.Value156, err = r.Unsigned(); err != nil {
		return req, err
	}
	if req.Value156 > 0xFF {
		return req, fmt.Errorf("%w: legacy field out of range", ErrInvalidPacket)
	}
	for i := 3; i < len(req.EventFlags); i++ {
		if req.EventFlags[i], err = r.Boolean(); err != nil {
			return req, err
		}
	}
	return req, r.RequireEnd()
}
