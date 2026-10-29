package net

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// relayReservation maps a handed-out key to its room slot. The reference server
// also pins the reservation to the member's peer id and source address so a
// leaked key cannot be used from somewhere else, and allows exactly one
// connection per reservation.
type relayReservation struct {
	roomID    uint16
	slot      uint8
	peerAddr  string // host part of the game connection's remote address
	peerID    uint32 // 0 until the first registration reports it
	bound     bool
	expiresAt time.Time
}

// relayRegistrationTimeout bounds how long a relay connection may stay open
// before it identifies itself, mirroring the reference server's five seconds.
const relayRegistrationTimeout = 5 * time.Second

// relayConn is one client connected to the relay port.
type relayConn struct {
	conn   net.Conn
	codec  *protocol.LambdaCodec
	mu     sync.Mutex
	roomID uint16
	slot   uint8
	joined bool
	// released marks connections this server closed on purpose (round over,
	// member left), so their teardown does not look like a peer disappearing.
	released bool
}

func (r *relayConn) send(p protocol.Packet) {
	frame, err := r.codec.Encode(p)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = r.conn.Write(frame)
}

// RelayDisconnectHandler reports that a member's relay transport went away
// without the server closing it. The reference treats that exactly like the
// member vanishing mid-match.
type RelayDisconnectHandler func(roomID uint16, slot uint8)

// RelayHandler receives every in-match packet that arrives over a relay
// connection, so the game logic can attribute it (character reports, defense
// seeds, life events) before it is forwarded to the other peers.
type RelayHandler func(roomID uint16, slot uint8, packet protocol.Packet)

// Relay forwards peer traffic between the members of one match. Keys are
// reserved by the game server when a match starts, mirroring the original
// reservation model.
type Relay struct {
	cfg config.Relay
	log *slog.Logger

	onPacket     RelayHandler
	onDisconnect RelayDisconnectHandler

	// registrationTimeout is how long an unregistered connection may stay open.
	registrationTimeout time.Duration

	mu           sync.Mutex
	rooms        map[uint16][]*relayConn
	reservations map[protocol.RelayKey]relayReservation
	active       int
}

// NewRelay builds the relay listener.
// SetPacketHandler installs the game-side observer for relay traffic.
func (r *Relay) SetPacketHandler(handler RelayHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onPacket = handler
}

// SetRegistrationTimeout changes how long a relay connection may stay open
// without registering. Call it before connections are accepted.
func (r *Relay) SetRegistrationTimeout(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registrationTimeout = timeout
}

// SetDisconnectHandler installs the game-side observer for unexpected relay
// teardown. It is not called for connections the server closed itself.
func (r *Relay) SetDisconnectHandler(handler RelayDisconnectHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onDisconnect = handler
}

// SendToSlot delivers a packet to a room member's registered relay connection.
func (r *Relay) SendToSlot(roomID uint16, slot uint8, packet protocol.Packet) bool {
	r.mu.Lock()
	connections := append([]*relayConn(nil), r.rooms[roomID]...)
	r.mu.Unlock()
	sent := false
	for _, conn := range connections {
		if conn.slot != slot {
			continue
		}
		conn.send(packet)
		sent = true
	}
	return sent
}

func NewRelay(cfg config.Relay, log *slog.Logger) *Relay {
	return &Relay{
		cfg:                 cfg,
		log:                 log,
		registrationTimeout: relayRegistrationTimeout,
		rooms:               map[uint16][]*relayConn{},
		reservations:        map[protocol.RelayKey]relayReservation{},
	}
}

// Register reserves a relay key for a room slot. peerAddress is the game
// connection's remote address; its host part pins the relay connection to the
// same client.
func (r *Relay) Register(key protocol.RelayKey, roomID uint16, slot uint8, peerAddress string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reservations[key] = relayReservation{
		roomID:    roomID,
		slot:      slot,
		peerAddr:  hostOnly(peerAddress),
		expiresAt: time.Now().Add(2 * time.Minute),
	}
}

// ReleaseRoom drops every reservation and connection of a finished room.
func (r *Relay) ReleaseRoom(roomID uint16) {
	r.mu.Lock()
	for key, reservation := range r.reservations {
		if reservation.roomID == roomID {
			delete(r.reservations, key)
		}
	}
	connections := r.closeRoomLocked(roomID)
	r.mu.Unlock()
	for _, conn := range connections {
		conn.conn.Close()
	}
}

// RevokeSlot drops one member's reservations and relay connection, mirroring the
// reference server's per-member Revoke when a player leaves the room.
func (r *Relay) RevokeSlot(roomID uint16, slot uint8) {
	r.mu.Lock()
	for key, reservation := range r.reservations {
		if reservation.roomID == roomID && reservation.slot == slot {
			delete(r.reservations, key)
		}
	}
	var closing []*relayConn
	remaining := r.rooms[roomID][:0]
	for _, conn := range r.rooms[roomID] {
		if conn.slot == slot {
			conn.released = true
			closing = append(closing, conn)
			continue
		}
		remaining = append(remaining, conn)
	}
	if len(remaining) == 0 {
		delete(r.rooms, roomID)
	} else {
		r.rooms[roomID] = remaining
	}
	r.mu.Unlock()
	for _, conn := range closing {
		conn.conn.Close()
	}
}

// closeRoomLocked detaches every connection of a room and marks them released.
// The caller holds r.mu and closes the returned connections after unlocking.
func (r *Relay) closeRoomLocked(roomID uint16) []*relayConn {
	connections := r.rooms[roomID]
	delete(r.rooms, roomID)
	for _, conn := range connections {
		conn.released = true
	}
	return connections
}

func (r *Relay) expire(now time.Time) {
	for key, reservation := range r.reservations {
		if !reservation.expiresAt.After(now) {
			delete(r.reservations, key)
		}
	}
}

// Serve accepts relay connections until the context is cancelled.
func (r *Relay) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", net.JoinHostPort(r.cfg.ListenAddress, itoa(int(r.cfg.ListenPort))))
	if err != nil {
		return err
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	r.log.Info("[Relay] listening", "port", r.cfg.ListenPort)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		r.mu.Lock()
		if r.cfg.MaxConnections > 0 && r.active >= r.cfg.MaxConnections {
			r.mu.Unlock()
			r.log.Warn("[Relay] connection limit reached")
			conn.Close()
			continue
		}
		r.active++
		r.mu.Unlock()
		go r.handle(conn)
	}
}

func (r *Relay) handle(conn net.Conn) {
	client := &relayConn{conn: conn, codec: protocol.NewLambdaCodec(protocol.EndpointServer)}
	defer func() {
		if recovered := recover(); recovered != nil {
			r.log.Error("[Relay] connection panic", "panic", recovered)
		}
		r.leave(client)
		conn.Close()
		r.mu.Lock()
		r.active--
		r.mu.Unlock()
	}()
	// The client's transport consumes the same 258-byte preamble the game
	// connection uses before it will send ReqJoinRelaySession.
	if _, err := conn.Write(protocol.Greeting()); err != nil {
		return
	}
	// A connection that never identifies itself would sit in the accept budget
	// until the idle timeout; the reference server drops it after five seconds.
	timeout := r.registrationTimeout
	if timeout <= 0 {
		timeout = relayRegistrationTimeout
	}
	registrationTimer := time.AfterFunc(timeout, func() {
		r.mu.Lock()
		joined := client.joined
		r.mu.Unlock()
		if !joined {
			r.log.Info("[Relay] registration timeout", "remote", conn.RemoteAddr().String())
			conn.Close()
		}
	})
	defer registrationTimer.Stop()

	idle := time.Duration(r.cfg.IdleTimeoutSeconds) * time.Second
	buffer := make([]byte, 8192)
	for {
		if idle > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(idle))
		}
		n, err := conn.Read(buffer)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				r.log.Debug("relay connection closed", "error", err)
			}
			return
		}
		packets, err := client.codec.Feed(buffer[:n])
		if err != nil {
			return
		}
		for _, packet := range packets {
			switch packet.ID {
			case 1: // ReqJoinRelaySession
				registration, err := protocol.ReadRelayRegistration(packet)
				if err != nil {
					return
				}
				if !r.join(client, registration) {
					return
				}
				registrationTimer.Stop()
				client.send(protocol.CreateRelayRegistrationReply())
			case 20006: // ReqLagDisPing
				if reply, err := protocol.CreateRelayLagReply(packet); err == nil {
					client.send(reply)
				}
			case 6: // RepPing
			case 15:
				// rcpChangeChrType is a peer-only signal. The reference server
				// does not forward it, and a peer that receives it runs its
				// rspHackDetectedNotice handler, which pops up a 公告 dialog
				// carrying a number from this body.
				r.log.Debug("[Relay] dropped", "room", client.roomID, "slot", client.slot,
					"id", packet.ID, "len", len(packet.Payload))
			default:
				r.mu.Lock()
				handler := r.onPacket
				roomID, slot := client.roomID, client.slot
				r.mu.Unlock()
				r.log.Debug("[Relay] inbound", "room", roomID, "slot", slot, "id", packet.ID, "len", len(packet.Payload))
				if handler != nil && client.joined {
					handler(roomID, slot, packet)
				}
				r.forward(client, packet)
			}
		}
	}
}

// join binds a connection to its reserved room slot.
func (r *Relay) join(client *relayConn, registration protocol.RelayRegistration) bool {
	remote := ""
	if address := client.conn.RemoteAddr(); address != nil {
		remote = hostOnly(address.String())
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(time.Now())
	reservation, ok := r.reservations[registration.Key]
	if !ok || reservation.slot != registration.Slot {
		r.log.Debug("[Relay] unknown relay key", "slot", registration.Slot)
		return false
	}
	// One reservation, one connection: a client that re-registers with the same
	// key must not end up with two transports, or every SendToSlot doubles.
	if reservation.bound {
		r.log.Warn("[Relay] duplicate registration rejected",
			"room", reservation.roomID, "slot", reservation.slot)
		return false
	}
	// The relay connection has to come from the same client that owns the game
	// connection, otherwise a leaked key is enough to take the slot over.
	// Loopback is only reachable from the same machine, so local probes that
	// dial the game port on 127.0.0.1 and the relay on the public address are
	// still the same client.
	if reservation.peerAddr != "" && remote != "" &&
		!isLoopbackHost(reservation.peerAddr) && !sameHost(reservation.peerAddr, remote) {
		r.log.Warn("[Relay] registration from unexpected address",
			"room", reservation.roomID, "slot", reservation.slot,
			"want", reservation.peerAddr, "got", remote)
		return false
	}
	if registration.PeerID != 0 {
		if reservation.peerID == 0 {
			reservation.peerID = registration.PeerID
		} else if reservation.peerID != registration.PeerID {
			r.log.Warn("[Relay] registration with a different peer id",
				"room", reservation.roomID, "slot", reservation.slot)
			return false
		}
	}
	reservation.bound = true
	r.reservations[registration.Key] = reservation

	client.roomID = reservation.roomID
	client.slot = reservation.slot
	client.joined = true
	r.rooms[reservation.roomID] = append(r.rooms[reservation.roomID], client)
	r.log.Debug("[Relay] registered", "room", reservation.roomID, "slot", reservation.slot)
	return true
}

// hostOnly strips the port from a net.Addr style address.
func hostOnly(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return address
}

// isLoopbackHost reports whether the host is a loopback address.
func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sameHost compares two hosts, treating IPv4 and its IPv6-mapped form as equal.
func sameHost(a, b string) bool {
	if a == b {
		return true
	}
	ipA, ipB := net.ParseIP(a), net.ParseIP(b)
	if ipA == nil || ipB == nil {
		return false
	}
	if v4 := ipA.To4(); v4 != nil {
		ipA = v4
	}
	if v4 := ipB.To4(); v4 != nil {
		ipB = v4
	}
	return ipA.Equal(ipB)
}

func (r *Relay) leave(client *relayConn) {
	r.mu.Lock()
	if !client.joined {
		r.mu.Unlock()
		return
	}
	roomID, slot := client.roomID, client.slot
	released := client.released
	members := r.rooms[roomID]
	for i, member := range members {
		if member == client {
			members[i] = members[len(members)-1]
			r.rooms[roomID] = members[:len(members)-1]
			break
		}
	}
	if len(r.rooms[roomID]) == 0 {
		delete(r.rooms, roomID)
	}
	client.joined = false
	handler := r.onDisconnect
	r.mu.Unlock()

	// A connection the server closed itself (round over, member left) is booked
	// as a normal teardown; anything else is a member disappearing mid-match.
	if !released && handler != nil {
		handler(roomID, slot)
	}
}

// relayRoomCapacity is the number of relay connections one room can hold: one
// per slot, eight slots. forwardTargets keeps its scratch list on the stack up
// to that size so relaying a packet costs no allocation.
const relayRoomCapacity = 8

// forwardTargetsLocked copies the room's connections except the sender into dst
// and reports how many were written. The caller holds r.mu. Rooms larger than the
// fixed list (which the slot rules make impossible) fall back to a heap copy.
func (r *Relay) forwardTargetsLocked(from *relayConn, dst *[relayRoomCapacity]*relayConn) (int, []*relayConn) {
	members := r.rooms[from.roomID]
	if len(members) > relayRoomCapacity {
		overflow := make([]*relayConn, 0, len(members))
		for _, member := range members {
			if member != from {
				overflow = append(overflow, member)
			}
		}
		return 0, overflow
	}
	written := 0
	for _, member := range members {
		if member == from {
			continue
		}
		dst[written] = member
		written++
	}
	return written, nil
}

func (r *Relay) forward(from *relayConn, packet protocol.Packet) {
	if !from.joined {
		return
	}
	var targets [relayRoomCapacity]*relayConn
	r.mu.Lock()
	written, overflow := r.forwardTargetsLocked(from, &targets)
	r.mu.Unlock()
	if overflow != nil {
		for _, member := range overflow {
			member.send(packet)
		}
		return
	}
	for i := 0; i < written; i++ {
		targets[i].send(packet)
	}
}
