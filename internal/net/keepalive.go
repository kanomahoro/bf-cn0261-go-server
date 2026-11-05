package net

import (
	"crypto/tls"
	"net"
	"time"
)

// TCP keepalive settings for every accepted connection. They are what separates
// a client that is merely quiet from one that has gone away: the kernel probes a
// silent peer and reports the failure, so the idle timeout can be turned off
// without leaking half-open connections.
const (
	keepAliveIdle     = 15 * time.Second
	keepAliveInterval = 15 * time.Second
	keepAliveCount    = 4
)

// enableKeepAlive turns on TCP keepalive for a connection whose transport
// supports it. A TLS listener hands out *tls.Conn, so the underlying TCP
// connection is unwrapped first; anything else is left alone and keeps whatever
// it had.
func enableKeepAlive(conn net.Conn) {
	if secure, ok := conn.(*tls.Conn); ok {
		conn = secure.NetConn()
	}
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcp.SetKeepAliveConfig(net.KeepAliveConfig{
		Enable:   true,
		Idle:     keepAliveIdle,
		Interval: keepAliveInterval,
		Count:    keepAliveCount,
	})
}
