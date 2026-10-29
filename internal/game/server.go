// Package game implements rooms, match coordination and packet dispatch.
package game

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/features"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/store"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/world"
)

// RelayRegistry reserves peer relay keys and groups connections by room.
type RelayRegistry interface {
	Register(key protocol.RelayKey, roomID uint16, slot uint8, peerAddress string)
	ReleaseRoom(roomID uint16)
	// RevokeSlot closes one member's relay connection. The reference server does
	// this when a player leaves the room, so a stale transport cannot outlive him.
	RevokeSlot(roomID uint16, slot uint8)
	// SendToSlot pushes a packet to one member's relay connection. Defense seed
	// tables travel this way, exactly like the reference implementation.
	SendToSlot(roomID uint16, slot uint8, packet protocol.Packet) bool
}

// Sender delivers one encoded packet to a client.
type Sender interface {
	Send(protocol.Packet) error
}

// toucher is implemented by the network connection so relay traffic can keep
// the game connection's idle deadline alive.
type toucher interface {
	Touch()
}

// Touch marks the session as active.
func (s *Session) Touch() {
	if t, ok := s.send.(toucher); ok {
		t.Touch()
	}
}

// Session is one authenticated game connection.
// Profile returns a consistent snapshot of the character and inventory.
func (s *Session) Profile() (store.Character, []store.Item) {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()
	items := make([]store.Item, len(s.Items))
	copy(items, s.Items)
	return s.Character, items
}

// SetProfile replaces the cached character and inventory under the lock.
func (s *Session) SetProfile(character store.Character, items []store.Item) {
	s.profileMu.Lock()
	defer s.profileMu.Unlock()
	s.Character = character
	if items != nil {
		s.Items = append([]store.Item(nil), items...)
	}
}

// CharacterID is safe to call from any goroutine.
func (s *Session) CharacterID() uint32 {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()
	return s.Character.ID
}

// Nickname is safe to call from any goroutine.
func (s *Session) Nickname() string {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()
	return s.Character.Nickname
}

type Session struct {
	ID          uint64
	Account     store.Account
	AccountName string
	// profileMu guards Character and Items: they are replaced by the owning
	// connection goroutine but read by others (plaza dedupe, kick by nickname).
	profileMu  sync.RWMutex
	Character  store.Character
	Items      []store.Item
	Challenges [2]uint32

	channelType  uint32
	channelIndex uint32
	peerAddress  string

	// room and slot are set once when the session joins and only read by the
	// owning connection goroutine afterwards.
	slot int
	room *Room

	// relayKey is the per-member key handed out in S601.
	relayKey    protocol.RelayKey
	relayActive bool
	inPlaza     bool

	// EnforceHandshake mirrors the original ClientConnection state machine; the
	// net layer enables it so unit tests can drive handlers directly.
	EnforceHandshake bool
	handshake        bool
	lastChatNotice   time.Time

	send Sender
}

// Send writes a packet to the connection.
func (s *Session) Send(p protocol.Packet) error {
	if s.send == nil {
		return nil
	}
	return s.send.Send(p)
}

// Server owns every room and dispatches client packets.
type Server struct {
	cfg        config.Config
	db         *store.DB
	svc        *features.Service
	accounts   *features.Accounts
	maps       world.Maps
	weapons    world.Weapons
	characters world.Characters
	defense    world.Defense
	respawn    world.Respawn
	prog       world.Progression
	log        *slog.Logger

	mu       sync.Mutex
	chatMu   sync.Mutex
	chats    map[uint32]*chatThrottle
	rooms    map[uint16]*Room
	nextRoom uint16
	channels *ChannelDirectory
	plazas   *PlazaDirectory
	relay    RelayRegistry
}

// New builds a server from the configured catalogs.
func New(cfg config.Config, db *store.DB, log *slog.Logger) (*Server, error) {
	prog, err := world.LoadProgression(cfg.ProgressionFile)
	if err != nil {
		return nil, err
	}
	maps, err := world.LoadMaps(cfg.GameMapsFile)
	if err != nil {
		return nil, err
	}
	weapons, err := world.LoadWeapons(cfg.GameWeaponsFile)
	if err != nil {
		return nil, err
	}
	characters, err := world.LoadCharacters(cfg.GameCharactersFile)
	if err != nil {
		return nil, err
	}
	defense, err := world.LoadDefense(cfg.GameDefenseFile)
	if err != nil {
		return nil, err
	}
	respawn, err := world.LoadRespawn(cfg.GameRespawnFile)
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:        cfg,
		db:         db,
		svc:        features.NewService(db, prog),
		accounts:   features.NewAccounts(db),
		maps:       maps,
		weapons:    weapons,
		characters: characters,
		defense:    defense,
		respawn:    respawn,
		prog:       prog,
		log:        log,
		rooms:      map[uint16]*Room{},
		nextRoom:   1,
		channels:   NewChannelDirectory(nil),
		plazas:     NewPlazaDirectory(),
	}, nil
}

// Log exposes the logger.
func (s *Server) Log() *slog.Logger { return s.log }

// SetRelay connects the relay registry used to hand out relay endpoints.
func (s *Server) SetRelay(registry RelayRegistry) { s.relay = registry }

// RelayEndpoint returns the advertised relay address/port when enabled.
func (s *Server) RelayEndpoint() (protocol.RelayKey, bool) {
	if !s.cfg.Relay.Enabled || s.cfg.Relay.ListenPort == 0 {
		return protocol.RelayKey{}, false
	}
	return protocol.RelayKey{}, true
}

// NewSession creates a session with fresh login challenges.
func (s *Server) NewSession(id uint64, send Sender) *Session {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return &Session{
		ID:   id,
		send: send,
		Challenges: [2]uint32{
			binary.LittleEndian.Uint32(raw[0:4]),
			binary.LittleEndian.Uint32(raw[4:8]),
		},
		slot: -1,
	}
}

// Disconnect releases a connection that went away: it leaves its room (which
// may delete it), cancels pending channel transfers and drops timers.
func (s *Server) Disconnect(session *Session) {
	if session == nil {
		return
	}
	if session.inPlaza {
		s.leavePlaza(session)
	}
	if session.room != nil {
		if err := s.leaveRoom(session); err != nil {
			s.log.Debug("leave on disconnect failed", "session", session.ID, "error", err)
		}
	}
	s.channels.Cancel(session.ID, session.Account.ID)
}

// SetPeerAddress records the client endpoint for channel transfers.
func (s *Session) SetPeerAddress(address string) { s.peerAddress = address }

// handleFirstConnect answers C1 with the exact sequence the reference server
// sends: three content flags, the patch reply, then the center list that lets
// the client leave its startup loading screen.
func (s *Server) handleFirstConnect(session *Session, packet protocol.Packet) error {
	if _, err := protocol.ReadFirstConnect(packet); err != nil {
		return err
	}
	for _, flag := range []protocol.Packet{
		protocol.CreateContentFlag(1004, 0), // disable the official login stage
		protocol.CreateContentFlag(1503, 1), // launcher login enabled
		protocol.CreateContentFlag(483, 0),  // starter selection handled by us
		protocol.CreatePatchReply(),
		protocol.CreateCenterList(),
	} {
		if err := session.Send(flag); err != nil {
			return err
		}
	}
	return nil
}

// handleInitialInfo answers C102 with the same burst the reference server
// sends: stats, inventory, equipment, the solo party and the main-menu stage.
// Without it the client stays on a dead screen and never reaches the plaza.
func (s *Server) handleInitialInfo(session *Session, packet protocol.Packet) error {
	if err := protocol.ValidateInitialInfo(packet, session.Challenges[0], session.Challenges[1]); err != nil {
		return err
	}
	character, items := session.Profile()
	if character.ID == 0 {
		return nil
	}
	if err := s.sendUserStatFor(session, character, items); err != nil {
		return err
	}
	wireItems := s.svc.WireItems(items)
	if reply, err := protocol.CreateInventoryReply(wireItems); err == nil {
		_ = session.Send(reply)
	}
	wire := s.svc.WireCharacter(character)
	if reply, err := protocol.CreateEquipmentReply(wire.AccountID, wire.CharacterType, wireItems); err == nil {
		_ = session.Send(reply)
	}
	if reply, err := protocol.CreateSoloPartyReply(wire, wireItems); err == nil {
		_ = session.Send(reply)
	}
	mainMenu, err := protocol.CreateMainMenuReply(wire, wireItems)
	if err != nil {
		return err
	}
	return session.Send(mainMenu)
}

// Handle dispatches one client packet.
func (s *Server) Handle(session *Session, packet protocol.Packet) error {
	if session.EnforceHandshake {
		if !session.handshake {
			// Response is {u16 count}{count bytes}. Only a description short
			// enough to fit in 255 bytes may be sent.
			if packet.ID != 1 && packet.ID != 14 {
				return fmt.Errorf("%w: expected FirstConnect", protocol.ErrInvalidPacket)
			}
			session.handshake = true
		} else if packet.ID == 1 {
			return fmt.Errorf("%w: duplicate FirstConnect", protocol.ErrInvalidPacket)
		}
	}
	s.log.Debug("inbound", "session", session.ID, "id", packet.ID, "len", len(packet.Payload))
	switch packet.ID {
	case 1: // FirstConnect: content flags, patch reply, center list
		return s.handleFirstConnect(session, packet)
	case 3: // ReqLogin
		return s.handleLogin(session, packet)
	case 102: // InitialInfo
		return s.handleInitialInfo(session, packet)
	case 15: // ReqCheckCreateUserName
		return s.handleNicknameCheck(session, packet)
	case 16: // ReqCreateUserInfo, or a recovery report during a match
		if session.room != nil && session.room.StartedReport() {
			return s.handleHealthReport(session, packet)
		}
		return s.handleCreateCharacter(session, packet)
	case 17: // character ready -> login reply
		return s.handleCharacterReady(session, packet)
	case 105, 106: // scene entry: main menu / lobby
		return s.handleSceneEntry(session, packet)
	case 12, 13: // channel move
		return s.handleChannelMove(session, packet)
	case 14: // ReqMoveInto (outside a match)
		if session.room != nil && session.room.StartedReport() {
			return s.handleHealthReport(session, packet)
		}
		return s.handleMoveInto(session, packet)
	case 401: // ReqListSession
		return s.handleSessionList(session, packet)
	case 402: // ReqCreateSession
		return s.handleCreateRoom(session, packet)
	case 403: // ReqJoinSession
		return s.handleJoinRoom(session, packet)
	case 404: // ReqLeaveSession
		return s.handleLeaveRoom(session, packet)
	case 505: // ready status
		return s.handleReady(session, packet)
	case 510: // ReqKickFromSession
		return s.handleKick(session, packet)
	case 515: // RoomChangeTeam
		return s.handleChangeTeam(session, packet)
	case 513: // RoomChangeGameSetting
		return s.handleGameSetting(session, packet)
	case 501: // room chat
		return s.handleSessionChat(session, packet)
	case 502: // module chat
		return s.handleModuleChat(session, packet)
	case 304: // party chat
		return s.handlePartyChat(session, packet)
	case 503: // RoomReqEnterGame
		return s.handleEnterGame(session, packet)
	case 701: // map loaded
		return s.handleMapLoaded(session, packet)
	case 601: // relay connected report
		return s.handleRelayConnected(session, packet)
	case 607: // ReportTableSetHolePunchingEnd must be empty
		if len(packet.Payload) != 0 {
			return fmt.Errorf("%w: expected empty peer table report", protocol.ErrInvalidPacket)
		}
		return nil
	case 801: // EnterShop
		return s.handleShopEntry(session, packet)
	case 802: // EnterInventory
		return s.handleInventoryOpen(session, packet)
	case 825: // ReqCheckCash
		return s.handleCashCheck(session, packet)
	case 808: // ReqShopBuy
		return s.handlePurchase(session, packet)
	case 803, 804: // equip / unequip
		return s.handleEquipment(session, packet)
	case 830: // preset select
		return s.handlePresetSelect(session, packet)
	case 517: // ReqPingUser
		return s.handlePingRequest(session, packet)
	case 518: // RepPingUser
		return s.handlePingResponse(session, packet)
	case 511: // plaza position
		return s.handlePlazaPosition(session, packet)
	case 521: // SendSessionInit
		if session.inPlaza {
			return s.handlePlazaInitialize(session)
		}
		return s.handleSessionInit(session, packet)
	case 32920: // own life point report
		return s.handleLifePoint(session, packet, false)
	case 704, 713, 32904, 32925: // respawn
		return s.handleRespawn(session, packet)
	}
	if packet.ID == 32777 || packet.ID == 32776 || packet.ID == 32923 ||
		packet.ID == 32900 || packet.ID == 32906 {
		return s.handlePeerDamage(session, packet)
	}
	if packet.ID == 32773 {
		return s.handlePeerNear(session, packet)
	}
	if packet.ID == 32840 {
		return s.handlePeerPing(session, packet)
	}
	if packet.ID == 14 && session.room != nil && session.room.StartedReport() {
		// During a match C14 is the health reset report, not ReqMoveInto.
		return s.handleHealthReport(session, packet)
	}
	if packet.ID == 8 || packet.ID == 9 {
		return s.handleRelayPeerUpdate(session, packet)
	}
	if packet.ID == 20 || packet.ID == 21 {
		return s.handleDefenseSeedReport(session, packet)
	}
	if packet.ID == 18 {
		return s.handleHealthReport(session, packet)
	}
	if packet.ID == 16 && session.room != nil && session.room.StartedReport() {
		return s.handleHealthReport(session, packet)
	}
	if protocol.IsPeerLife(packet.ID) {
		return s.handlePeerLife(session, packet, false)
	}
	if protocol.IsPeerKick(packet.ID) {
		return s.handlePeerKick(session, packet, false)
	}
	if packet.ID == 32813 { // return to normal after stun recovery
		return s.handleReturnToNormal(session, packet, false)
	}
	if packet.ID == 2015 || packet.ID == 2017 {
		return s.handleBattleState(session, packet)
	}
	if protocol.IsPeerAction(packet.ID) {
		return s.handlePeerAction(session, packet)
	}
	preview := packet.Payload
	if len(preview) > 48 {
		preview = preview[:48]
	}
	s.log.Debug("unhandled packet", "session", session.ID, "id", packet.ID,
		"len", len(packet.Payload), "hex", fmt.Sprintf("%x", preview))
	return nil
}

func (s *Server) handleLogin(session *Session, packet protocol.Packet) error {
	token, err := protocol.ReadLoginTicket(packet)
	if err != nil {
		return err
	}
	accountID, err := s.accounts.ConsumeTicket(token)
	if err != nil {
		return session.Send(protocol.CreateLoginReply(1, 0, nil, session.Challenges[0], session.Challenges[1], nil))
	}
	account, err := s.db.AccountByID(accountID)
	if err != nil {
		return err
	}
	session.Account = account
	session.AccountName = account.Name
	character, err := s.svc.Profile(accountID)
	if errors.Is(err, features.ErrCharacterMissing) {
		// No character yet: the reference server sends S12 so the client opens
		// the creation flow, then C15/C16/C17 follow.
		return session.Send(protocol.Packet{ID: 12})
	}
	if err != nil {
		return err
	}
	session.Character = character
	items, err := s.svc.Items(character.ID)
	if err != nil {
		// Login continues with an empty inventory rather than failing outright,
		// but the reason must not be lost silently.
		s.log.Error("inventory load failed at login", "character", character.ID, "error", err)
	}
	session.Items = items
	return s.sendLoginReply(session)
}

// sendLoginReply emits S3 with fresh challenges, matching SendLoginReply.
func (s *Server) sendLoginReply(session *Session) error {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return err
	}
	session.Challenges = [2]uint32{
		binary.LittleEndian.Uint32(raw[0:4]),
		binary.LittleEndian.Uint32(raw[4:8]),
	}
	character := s.svc.WireCharacter(session.Character)
	return session.Send(protocol.CreateLoginReply(0, session.Account.ID,
		protocol.U16String(session.AccountName), session.Challenges[0],
		session.Challenges[1], character.Nickname))
}

func (s *Server) handleNicknameCheck(session *Session, packet protocol.Packet) error {
	nickname, err := protocol.ReadNicknameRequest(packet)
	if err != nil {
		return err
	}
	result := protocol.CheckNickname(nickname)
	if result == protocol.NicknameOK {
		taken, err := s.svc.NicknameTaken(nickname)
		if err != nil {
			return err
		}
		if taken {
			result = protocol.NicknameTaken
		}
	}
	return session.Send(protocol.CreateNicknameReply(result, nickname))
}

func (s *Server) handleCreateCharacter(session *Session, packet protocol.Packet) error {
	request, err := protocol.ReadCreateCharacterRequest(packet)
	if err != nil {
		return err
	}
	if session.Account.ID == 0 || session.Character.ID != 0 {
		return nil
	}
	character, err := s.svc.CreateCharacter(session.Account.ID, request.Nickname,
		request.CharacterType, request.StarterSelection)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return session.Send(protocol.CreateCharacterReply(protocol.NicknameTaken))
		}
		return session.Send(protocol.CreateCharacterReply(protocol.NicknameInvalid))
	}
	session.Character = character
	items, err := s.svc.Items(character.ID)
	if err != nil {
		s.log.Error("inventory load failed after character creation",
			"character", character.ID, "error", err)
	}
	session.Items = items
	// The login reply is deferred until C17, exactly like the original.
	return session.Send(protocol.CreateCharacterReply(protocol.NicknameOK))
}

// handleCharacterReady answers C17 with the login reply after creation.
func (s *Server) handleCharacterReady(session *Session, packet protocol.Packet) error {
	if len(packet.Payload) != 0 || session.Character.ID == 0 {
		return fmt.Errorf("%w: unexpected C17", protocol.ErrInvalidPacket)
	}
	return s.sendLoginReply(session)
}

// handleSceneEntry answers C105 (main menu) and C106 (lobby entry).
func (s *Server) handleSceneEntry(session *Session, packet protocol.Packet) error {
	if len(packet.Payload) != 0 || session.Character.ID == 0 {
		return fmt.Errorf("%w: scene entry requires an empty payload", protocol.ErrInvalidPacket)
	}
	if packet.ID == 105 {
		// Main menu: the plaza roster no longer applies.
		if session.inPlaza {
			s.leavePlaza(session)
		}
		character, items := session.Profile()
		reply, err := protocol.CreateMainMenuReply(s.svc.WireCharacter(character), s.svc.WireItems(items))
		if err != nil {
			return err
		}
		return session.Send(reply)
	}
	// C106 is the plaza scene: join the real plaza instance so other players,
	// positions and ping probes work, then S110 carries the live roster.
	return s.enterPlaza(session)
}

func (s *Server) sendUserStat(session *Session) error {
	return s.sendUserStatFor(session, session.Character, session.Items)
}

// sendUserStatFor emits S105 for an explicit profile, so goroutines other than
// the owning connection never read session fields.
func (s *Server) sendUserStatFor(session *Session, character store.Character, items []store.Item) error {
	wire := s.svc.WireCharacter(character)
	stats := s.svc.Stats(character)
	reply, err := protocol.CreateUserStatReply(wire, stats)
	if err != nil {
		return err
	}
	return session.Send(reply)
}

func (s *Server) sendMainMenu(session *Session) error {
	if err := s.sendUserStat(session); err != nil {
		return err
	}
	wire := s.svc.WireCharacter(session.Character)
	items := s.svc.WireItems(session.Items)
	reply, err := protocol.CreateMainMenuReply(wire, items)
	if err != nil {
		return err
	}
	if err := session.Send(reply); err != nil {
		return err
	}
	party, err := protocol.CreateSoloPartyReply(wire, items)
	if err != nil {
		return err
	}
	return session.Send(party)
}

// handleChannelMove answers C12/C13. This server runs one lobby channel: the
// client's toolbar still offers two, so a request for any channel is answered
// with the failure result and no endpoint. The client then shows its own
// "cannot enter" prompt and keeps its current connection, instead of
// reconnecting into a second channel that would split players, plaza and room
// lists.
func (s *Server) handleChannelMove(session *Session, packet protocol.Packet) error {
	request, err := protocol.ReadChannelRequest(packet)
	if err != nil {
		return err
	}
	if session.room != nil {
		s.log.Debug("[Channel] move refused inside a room", "session", session.ID,
			"type", request.Type, "index", request.Index)
		return session.Send(protocol.CreateChannelReply(1, [4]byte{}, 0))
	}
	if request.Type == DefaultChannelType && request.Index == DefaultChannelIndex {
		// The client re-selects the channel it is already in; keep the reply
		// shape it expects but point it back at this listener so nothing moves.
		address, port := s.endpoint()
		return session.Send(protocol.CreateChannelReply(0, address, port))
	}
	s.log.Info("[Channel] move refused: this server runs a single channel",
		"session", session.ID, "type", request.Type, "index", request.Index)
	return session.Send(protocol.CreateChannelReply(1, [4]byte{}, 0))
}

// endpoint returns the advertised game listener address.
func (s *Server) endpoint() ([4]byte, uint16) {
	var address [4]byte
	if parsed := net.ParseIP(s.cfg.Network.ListenAddress); parsed != nil {
		if v4 := parsed.To4(); v4 != nil {
			copy(address[:], v4)
		}
	}
	if address == ([4]byte{}) {
		address = [4]byte{127, 0, 0, 1}
	}
	return address, s.cfg.Network.ListenPort
}
