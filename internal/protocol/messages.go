package protocol

import (
	"fmt"
	"math"
	"time"
)

// OLE date base: 25569 days between 1899-12-30 and the Unix epoch.
const oleEpochDays = 25569.0

func oleNow() float64 {
	return oleEpochDays + float64(time.Now().UnixNano())/1e9/86400.0
}

// ReadLoginTicket parses C3 (ReqLogin) and returns the one-time ticket.
func ReadLoginTicket(p Packet) (string, error) {
	if p.ID != 3 {
		return "", fmt.Errorf("%w: expected ReqLogin", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	identity, err := r.String(64)
	if err != nil {
		return "", err
	}
	if _, err = r.String(4096); err != nil {
		return "", err
	}
	if _, err = r.String(4096); err != nil {
		return "", err
	}
	count, err := r.Unsigned()
	if err != nil {
		return "", err
	}
	if count > 4096 || int(count) > r.Remaining()/4 {
		return "", fmt.Errorf("%w: login vector out of bounds", ErrInvalidPacket)
	}
	for i := uint32(0); i < count; i++ {
		if _, err := r.Unsigned(); err != nil {
			return "", err
		}
	}
	if _, err := r.Unsigned(); err != nil {
		return "", err
	}
	if _, err := r.Unsigned(); err != nil {
		return "", err
	}
	if err := r.RequireEnd(); err != nil {
		return "", err
	}
	token := make([]byte, len(identity))
	for i, unit := range identity {
		if unit > 127 {
			return "", fmt.Errorf("%w: login identity encoding", ErrInvalidPacket)
		}
		token[i] = byte(unit)
	}
	return string(token), nil
}

// CreateLoginReply builds S3.
func CreateLoginReply(result, accountID uint32, accountName []uint16,
	firstChallenge, secondChallenge uint32, nickname []uint16) Packet {
	w := NewWriter()
	w.Unsigned(result)
	w.String(accountName)
	w.Unsigned(accountID)
	w.String(nickname)
	w.Unsigned(firstChallenge)
	w.Unsigned(secondChallenge)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Uint64(math.Float64bits(oleNow()))
	w.String(nil)
	w.Unsigned(0)
	return Packet{ID: 3, Payload: w.Payload()}
}

// ValidateInitialInfo parses C102 and checks the echoed challenges.
func ValidateInitialInfo(p Packet, firstChallenge, secondChallenge uint32) error {
	if p.ID != 102 {
		return fmt.Errorf("%w: expected InitialInfo", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	first, err := r.Unsigned()
	if err != nil {
		return err
	}
	second, err := r.Unsigned()
	if err != nil {
		return err
	}
	if first != firstChallenge || second != secondChallenge {
		return fmt.Errorf("%w: login challenge mismatch", ErrInvalidPacket)
	}
	count, err := r.Unsigned()
	if err != nil {
		return err
	}
	if count != 5 {
		return fmt.Errorf("%w: InitialInfo requires five settings", ErrInvalidPacket)
	}
	for i := uint32(0); i < count; i++ {
		if _, err := r.Unsigned(); err != nil {
			return err
		}
	}
	return r.RequireEnd()
}

// ChannelRequest is C12/C13.
type ChannelRequest struct {
	Type  uint32
	Index uint32
}

// ReadChannelRequest parses C12/C13.
func ReadChannelRequest(p Packet) (ChannelRequest, error) {
	if p.ID != 12 && p.ID != 13 {
		return ChannelRequest{}, fmt.Errorf("%w: expected channel move", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	typ, err := r.Unsigned()
	if err != nil {
		return ChannelRequest{}, err
	}
	req := ChannelRequest{Type: typ}
	if p.ID == 12 {
		if req.Index, err = r.Unsigned(); err != nil {
			return req, err
		}
	}
	return req, r.RequireEnd()
}

// MoveIntoRequest is C14.
type MoveIntoRequest struct {
	AccountID  uint32
	EnterLobby bool
}

// ReadMoveIntoRequest parses C14.
func ReadMoveIntoRequest(p Packet) (MoveIntoRequest, error) {
	if p.ID != 14 {
		return MoveIntoRequest{}, fmt.Errorf("%w: expected ReqMoveInto", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	id, err := r.Unsigned()
	if err != nil {
		return MoveIntoRequest{}, err
	}
	enter, err := r.Unsigned()
	if err != nil {
		return MoveIntoRequest{}, err
	}
	if err := r.RequireEnd(); err != nil {
		return MoveIntoRequest{}, err
	}
	if id == 0 || enter > 1 {
		return MoveIntoRequest{}, fmt.Errorf("%w: ReqMoveInto identity", ErrInvalidPacket)
	}
	return MoveIntoRequest{AccountID: id, EnterLobby: enter != 0}, nil
}

// SessionListRequest is C401.
type SessionListRequest struct {
	Page  uint16
	Count uint16
}

// ReadSessionListRequest parses C401.
func ReadSessionListRequest(p Packet) (SessionListRequest, error) {
	if p.ID != 401 {
		return SessionListRequest{}, fmt.Errorf("%w: expected ReqListSession", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	page, err := r.Short()
	if err != nil {
		return SessionListRequest{}, err
	}
	count, err := r.Short()
	if err != nil {
		return SessionListRequest{}, err
	}
	if err := r.RequireEnd(); err != nil {
		return SessionListRequest{}, err
	}
	if count == 0 || count > 6 {
		return SessionListRequest{}, fmt.Errorf("%w: session list count", ErrInvalidPacket)
	}
	return SessionListRequest{Page: page, Count: count}, nil
}

// CreateChannelReply builds S9.
func CreateChannelReply(result uint32, address [4]byte, port uint16) Packet {
	w := NewWriter()
	w.Unsigned(result)
	w.Bytes(address[:])
	w.Short(port)
	return Packet{ID: 9, Payload: w.Payload()}
}

// CreateMoveIntoReply builds S11.
func CreateMoveIntoReply(success bool, channelType, channelIndex uint32) Packet {
	w := NewWriter()
	if success {
		w.Unsigned(1)
	} else {
		w.Unsigned(0)
	}
	w.Unsigned(channelType)
	w.Unsigned(channelIndex)
	return Packet{ID: 11, Payload: w.Payload()}
}

// CreateGotoLobbyReply builds S109.
func CreateGotoLobbyReply() Packet {
	w := NewWriter()
	w.Unsigned(0)
	return Packet{ID: 109, Payload: w.Payload()}
}

// CreateSessionListReply builds S101.
func CreateSessionListReply(page uint32, hasMore bool, settings [][]byte) (Packet, error) {
	if len(settings) > 6 {
		return Packet{}, fmt.Errorf("%w: session list exceeds six cards", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(page)
	w.Boolean(hasMore)
	w.Unsigned(uint32(len(settings)))
	for _, s := range settings {
		w.Bytes(s)
	}
	return Packet{ID: 101, Payload: w.Payload()}, nil
}

// NicknameResult is the character creation result code.
type NicknameResult uint32

// Character creation result codes.
const (
	NicknameOK            NicknameResult = 0
	NicknameTaken         NicknameResult = 1
	NicknameInvalid       NicknameResult = 2
	NicknameAccountExists NicknameResult = 3
)

// CheckNickname validates a nickname the way the client does.
func CheckNickname(nickname []uint16) NicknameResult {
	if len(nickname) < 2 || len(nickname) > 12 {
		return NicknameInvalid
	}
	visible := false
	for _, unit := range nickname {
		switch {
		case unit < 0x20 || (unit >= 0x7F && unit <= 0x9F):
			return NicknameInvalid
		case unit == 0x3000 || unit == ' ':
			continue
		case unit == 0x2028 || unit == 0x2029:
			return NicknameInvalid
		}
		visible = true
	}
	if !visible {
		return NicknameInvalid
	}
	return NicknameOK
}

// ReadNicknameRequest parses C15.
func ReadNicknameRequest(p Packet) ([]uint16, error) {
	if p.ID != 15 {
		return nil, fmt.Errorf("%w: expected ReqCheckCreateUserName", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	nickname, err := r.String(64)
	if err != nil {
		return nil, err
	}
	return nickname, r.RequireEnd()
}

// CreateCharacterRequest is C16.
type CreateCharacterRequest struct {
	Nickname         []uint16
	CharacterType    uint32
	StarterSelection uint32
}

// ReadCreateCharacterRequest parses C16.
func ReadCreateCharacterRequest(p Packet) (CreateCharacterRequest, error) {
	if p.ID != 16 {
		return CreateCharacterRequest{}, fmt.Errorf("%w: expected ReqCreateUserInfo", ErrInvalidPacket)
	}
	r := NewReader(p.Payload)
	nickname, err := r.String(64)
	if err != nil {
		return CreateCharacterRequest{}, err
	}
	characterType, err := r.Unsigned()
	if err != nil {
		return CreateCharacterRequest{}, err
	}
	starter, err := r.Unsigned()
	if err != nil {
		return CreateCharacterRequest{}, err
	}
	if _, err := r.String(4096); err != nil {
		return CreateCharacterRequest{}, err
	}
	if _, err := r.Unsigned(); err != nil {
		return CreateCharacterRequest{}, err
	}
	return CreateCharacterRequest{Nickname: nickname, CharacterType: characterType,
		StarterSelection: starter}, r.RequireEnd()
}

// CreateNicknameReply builds S13.
func CreateNicknameReply(result NicknameResult, nickname []uint16) Packet {
	w := NewWriter()
	w.Unsigned(uint32(result))
	w.String(nickname)
	return Packet{ID: 13, Payload: w.Payload()}
}

// CreateCharacterReply builds S14.
func CreateCharacterReply(result NicknameResult) Packet {
	w := NewWriter()
	w.Unsigned(uint32(result))
	return Packet{ID: 14, Payload: w.Payload()}
}

// Stats is the wire view of a character's persisted progress.
type Stats struct {
	Lucci      uint32
	Cash       uint32
	Level      uint32
	LevelStart uint32
	Experience uint32
	NextLevel  uint32
}

// CreateUserStatReply builds S105 (UserInfo).
func CreateUserStatReply(character Character, stats Stats) (Packet, error) {
	const maximum = uint32(math.MaxInt32)
	if character.AccountID == 0 || CheckNickname(character.Nickname) != NicknameOK ||
		stats.Lucci > maximum || stats.Cash > maximum ||
		stats.Level < 1 || stats.Level > 200 ||
		stats.LevelStart > stats.Experience ||
		stats.Experience > stats.NextLevel ||
		stats.NextLevel > maximum {
		return Packet{}, fmt.Errorf("%w: user stat identity or value", ErrInvalidPacket)
	}
	w := NewWriter()
	w.Unsigned(0) // UserInfo +100 default color
	w.Unsigned(stats.Experience)
	w.Unsigned(stats.LevelStart)
	w.Unsigned(stats.NextLevel)
	w.Unsigned(stats.Lucci)
	w.Unsigned(stats.Cash)
	w.Unsigned(0)
	w.Unsigned(0)
	w.String(nil)
	w.Unsigned(0)
	writeFighterInfo(w, character, stats.Level)
	return Packet{ID: 105, Payload: w.Payload()}, nil
}

func writeFighterInfo(w *Writer, character Character, level uint32) {
	w.Unsigned(character.AccountID)
	w.String(character.Nickname)
	w.Unsigned(0)
	w.Unsigned(level)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Double(0)
	w.Unsigned(0)
	w.Unsigned(0)
	for i := 0; i < 49; i++ {
		w.Unsigned(0)
	}
	w.Unsigned(0)
	w.Unsigned(1000)
	w.Unsigned(1)
	w.Unsigned(0)
	w.Unsigned(0)
	w.Unsigned(0)
	for i := 0; i < 8; i++ {
		w.Unsigned(0)
	}
	for i := 0; i < 8; i++ {
		w.Unsigned(0)
	}
	w.String(nil)
	w.Unsigned(0)
}

// ChatChannel identifies the chat scope.
type ChatChannel uint32

// Chat channels.
const (
	ChatNearby ChatChannel = 0
	ChatTeam   ChatChannel = 1
	ChatParty  ChatChannel = 2
	ChatSystem ChatChannel = 4
)

// ValidateChatText enforces the client's chat constraints.
func ValidateChatText(text []uint16) error {
	if len(text) == 0 || len(text) > 256 {
		return fmt.Errorf("%w: chat length", ErrInvalidPacket)
	}
	visible := false
	for i := 0; i < len(text); i++ {
		unit := text[i]
		if unit < 0x20 || (unit >= 0x7F && unit <= 0x9F) ||
			(unit >= 0xDC00 && unit <= 0xDFFF) || unit == 0x2028 || unit == 0x2029 {
			return fmt.Errorf("%w: chat control character", ErrInvalidPacket)
		}
		if unit >= 0xD800 && unit <= 0xDBFF {
			i++
			if i >= len(text) || text[i] < 0xDC00 || text[i] > 0xDFFF {
				return fmt.Errorf("%w: chat surrogate pair", ErrInvalidPacket)
			}
		}
		if unit != ' ' && unit != 0x3000 {
			visible = true
		}
	}
	if !visible {
		return fmt.Errorf("%w: chat must contain visible text", ErrInvalidPacket)
	}
	return nil
}

// CreateModuleChatReply builds S509.
func CreateModuleChatReply(slot uint16, nickname []uint16, channel uint32, text []uint16) (Packet, error) {
	if (slot >= 32 && slot != 0xFFFD) || len(nickname) == 0 || len(nickname) > 16 {
		return Packet{}, fmt.Errorf("%w: chat identity", ErrInvalidPacket)
	}
	if err := ValidateChatText(text); err != nil {
		return Packet{}, err
	}
	w := NewWriter()
	w.Short(slot)
	w.String(nickname)
	w.Unsigned(channel)
	w.String(nil)
	w.String(text)
	w.Unsigned(Unset)
	w.Unsigned(0)
	w.Unsigned(0)
	return Packet{ID: 509, Payload: w.Payload()}, nil
}

// CreateChatNotice builds the system chat line.
func CreateChatNotice(text []uint16) (Packet, error) {
	return CreateModuleChatReply(0xFFFD, U16String("系统"), uint32(ChatSystem), text)
}

// CreateSessionChatReply builds S508.
func CreateSessionChatReply(slot uint16, teamOnly bool, text []uint16) (Packet, error) {
	if slot >= 32 {
		return Packet{}, fmt.Errorf("%w: session chat slot", ErrInvalidPacket)
	}
	if err := ValidateChatText(text); err != nil {
		return Packet{}, err
	}
	w := NewWriter()
	w.Short(slot)
	w.Boolean(teamOnly)
	w.String(nil)
	w.String(text)
	w.Unsigned(Unset)
	w.Unsigned(0)
	return Packet{ID: 508, Payload: w.Payload()}, nil
}

// CreatePartyChatReply builds S305.
func CreatePartyChatReply(accountID uint32, text []uint16) (Packet, error) {
	if accountID == 0 {
		return Packet{}, fmt.Errorf("%w: party chat identity", ErrInvalidPacket)
	}
	if err := ValidateChatText(text); err != nil {
		return Packet{}, err
	}
	w := NewWriter()
	w.Unsigned(accountID)
	w.String(nil)
	w.String(text)
	w.Unsigned(Unset)
	w.Unsigned(0)
	return Packet{ID: 305, Payload: w.Payload()}, nil
}
