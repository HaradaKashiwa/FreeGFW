package services

import (
	"context"
	"fmt"
	"net"
	"sync"

	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"golang.org/x/time/rate"
)

type StatisticsTracker struct {
	manager         *trafficontrol.Manager
	outboundManager adapter.OutboundManager
	userLimits      map[string]uint64
	limiters        map[string]*rate.Limiter
	mu              sync.RWMutex
}

func NewStatisticsTracker(manager *trafficontrol.Manager, outboundManager adapter.OutboundManager, limits map[string]uint64) *StatisticsTracker {
	t := &StatisticsTracker{
		manager:         manager,
		outboundManager: outboundManager,
		userLimits:      limits,
		limiters:        make(map[string]*rate.Limiter),
	}
	t.UpdateLimits(limits)
	return t
}

func (t *StatisticsTracker) UpdateLimits(limits map[string]uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.userLimits = limits
	// Rebuild limiters
	t.limiters = make(map[string]*rate.Limiter)
	for user, limit := range limits {
		if limit > 0 {
			burst := int(limit)
			if burst > 512*1024 {
				burst = 512 * 1024
			}
			if burst < 64*1024 {
				burst = 64 * 1024
			}
			t.limiters[user] = rate.NewLimiter(rate.Limit(limit), burst)
		}
	}
}

func (t *StatisticsTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	limiter := t.getLimiter(metadata)
	if limiter != nil {
		fmt.Printf("DEBUG: Applying TCP limit for user %s\n", metadata.User)
		conn = NewRateLimitedConn(conn, limiter)
	} else {
		fmt.Printf("DEBUG: No TCP limit for user %s\n", metadata.User)
	}
	return trafficontrol.NewTCPTracker(conn, t.manager, metadata, t.outboundManager, matchedRule, matchOutbound)
}

func (t *StatisticsTracker) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	limiter := t.getLimiter(metadata)
	if limiter != nil {
		conn = NewRateLimitedPacketConn(conn, limiter)
	}
	return trafficontrol.NewUDPTracker(conn, t.manager, metadata, t.outboundManager, matchedRule, matchOutbound)

}

func (t *StatisticsTracker) GetLimiterForUser(user string) *rate.Limiter {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.limiters == nil {
		return nil
	}

	limiter, ok := t.limiters[user]
	if !ok {
		limiter = t.limiters["__DEFAULT__"]
	}
	return limiter
}

func (t *StatisticsTracker) getLimiter(metadata adapter.InboundContext) *rate.Limiter {
	return t.GetLimiterForUser(metadata.User)
}

// remove unused getLimit

// RateLimitedConn wraps net.Conn to enforce rate limiting.
// We avoid embedding net.Conn directly in the struct to prevent
// method promotion of optional interfaces (like io.ReaderFrom)
// that would bypass our Read/Write methods.
type RateLimitedConn struct {
	conn    net.Conn
	limiter *rate.Limiter
}

func NewRateLimitedConn(conn net.Conn, limiter *rate.Limiter) net.Conn {
	// Note: We return net.Conn interface.
	return &RateLimitedConn{
		conn:    conn,
		limiter: limiter,
	}
}

func (c *RateLimitedConn) Read(b []byte) (n int, err error) {
	n, err = c.conn.Read(b)
	if n > 0 {
		c.limiter.WaitN(context.Background(), n)
	}
	return
}

func (c *RateLimitedConn) Write(b []byte) (n int, err error) {
	c.limiter.WaitN(context.Background(), len(b))
	return c.conn.Write(b)
}

func (c *RateLimitedConn) Close() error {
	return c.conn.Close()
}

func (c *RateLimitedConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *RateLimitedConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *RateLimitedConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *RateLimitedConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *RateLimitedConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

type RateLimitedPacketConn struct {
	conn    N.PacketConn
	limiter *rate.Limiter
}

func NewRateLimitedPacketConn(conn N.PacketConn, limiter *rate.Limiter) *RateLimitedPacketConn {
	return &RateLimitedPacketConn{
		conn:    conn,
		limiter: limiter,
	}
}

func (c *RateLimitedPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.conn.ReadPacket(buffer)
	if err == nil {
		c.limiter.WaitN(context.Background(), buffer.Len())
	}
	return
}

func (c *RateLimitedPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	c.limiter.WaitN(context.Background(), buffer.Len())
	return c.conn.WritePacket(buffer, destination)
}

func (c *RateLimitedPacketConn) Close() error {
	return c.conn.Close()
}

func (c *RateLimitedPacketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *RateLimitedPacketConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *RateLimitedPacketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *RateLimitedPacketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
