// Package net hosts the game, launcher-authentication and relay listeners.
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
	"github.com/kanomahoro/bf-cn0261-go-server/internal/game"
	"github.com/kanomahoro/bf-cn0261-go-server/internal/protocol"
)

// Game serves the CN0261 lobby/game protocol.
type Game struct {
	cfg    config.Config
	server *game.Server
	log    *slog.Logger
	mu     sync.Mutex
	nextID uint64
	count  int
}

// NewGame builds the game listener.
func NewGame(cfg config.Config, server *game.Server, log *slog.Logger) *Game {
	return &Game{cfg: cfg, server: server, log: log}
}

// Serve accepts connections until the context is cancelled.
func (g *Game) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", g.cfg.Address())
	if err != nil {
		return err
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	g.log.Info("[Application] ready port=" + itoa(int(g.cfg.Network.ListenPort)) + " protocol=cn0261-ticket-login")
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return err
		}
		g.mu.Lock()
		if g.count >= g.cfg.Network.MaxConnections {
			g.mu.Unlock()
			conn.Close()
			continue
		}
		g.count++
		g.nextID++
		id := g.nextID
		g.mu.Unlock()
		go g.handle(ctx, id, conn)
	}
}

func (g *Game) release() {
	g.mu.Lock()
	g.count--
	g.mu.Unlock()
}

type connection struct {
	conn   net.Conn
	idle   time.Duration
	log    *slog.Logger
	codec  *protocol.LambdaCodec
	mu     sync.Mutex
	closed bool
}

// Touch refreshes the idle deadline. Relay traffic counts as activity for the
// game connection too, otherwise a client that only talks over the relay is
// dropped after one idle window.
func (c *connection) Touch() {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(c.idle))
}

// framePool recycles the scratch buffers used to encode one outbound frame.
// net.Conn.Write copies into the kernel buffer before returning, so the slice is
// free to be reused immediately afterwards.
const pooledFrameLimit = 8192

var framePool = sync.Pool{New: func() any { buf := make([]byte, 0, 256); return &buf }}

func (c *connection) Send(p protocol.Packet) error {
	// Encoding is stateful (the cipher advances per frame), and broadcasts reach
	// this connection from other players' goroutines, so both steps share a lock.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	if c.log != nil {
		c.log.Debug("outbound", "id", p.ID, "len", len(p.Payload))
	}
	// Frames are written and dropped, so one scratch buffer per send can come
	// from a pool instead of allocating a fresh slice for every packet.
	buffer := framePool.Get().(*[]byte)
	frame, err := c.codec.EncodeInto((*buffer)[:0], p)
	if err != nil {
		framePool.Put(buffer)
		return err
	}
	_, err = c.conn.Write(frame)
	if cap(frame) <= pooledFrameLimit {
		*buffer = frame
		framePool.Put(buffer)
	}
	return err
}

func (g *Game) handle(ctx context.Context, id uint64, conn net.Conn) {
	defer g.release()
	defer conn.Close()
	var session *game.Session
	defer func() {
		if recovered := recover(); recovered != nil {
			g.log.Error("connection panic", "session", id, "panic", recovered)
		}
		// Releasing the room here is what keeps ghost rooms and stale slots from
		// accumulating when a client disappears without ReqLeaveSession.
		if session != nil {
			g.server.Disconnect(session)
		}
	}()

	if _, err := conn.Write(protocol.Greeting()); err != nil {
		return
	}
	client := &connection{conn: conn, idle: time.Duration(g.cfg.Network.IdleTimeoutSeconds) * time.Second, log: g.log, codec: protocol.NewLambdaCodec(protocol.EndpointServer)}
	session = g.server.NewSession(id, client)
	// Real clients send C1 before anything else; enforce it only on live
	// connections so unit tests can drive handlers directly.
	session.EnforceHandshake = true
	if remote := conn.RemoteAddr(); remote != nil {
		session.SetPeerAddress(remote.String())
	}
	idle := time.Duration(g.cfg.Network.IdleTimeoutSeconds) * time.Second
	buffer := make([]byte, 8192)
	for {
		if idle > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(idle))
		}
		n, err := conn.Read(buffer)
		if err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				g.log.Debug("connection closed", "session", id, "error", err)
			}
			return
		}
		packets, err := client.codec.Feed(buffer[:n])
		if err != nil {
			g.log.Debug("codec failure", "session", id, "error", err)
			return
		}
		for _, packet := range packets {
			if err := g.server.Handle(session, packet); err != nil {
				// A rejected packet is either a client bug or a protocol mismatch;
				// warn so operators can see it without enabling debug logging.
				g.log.Warn("handle failure", "session", id, "id", packet.ID,
					"len", len(packet.Payload), "error", err)
				if errors.Is(err, protocol.ErrInvalidPacket) || errors.Is(err, protocol.ErrTruncated) || errors.Is(err, protocol.ErrTrailing) {
					return
				}
			}
		}
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [8]byte{}
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
