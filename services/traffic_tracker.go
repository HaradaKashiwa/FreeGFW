package services

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	N "github.com/sagernet/sing/common/network"
)

type StatisticsTracker struct {
	manager         *trafficontrol.Manager
	outboundManager adapter.OutboundManager
}

func NewStatisticsTracker(manager *trafficontrol.Manager, outboundManager adapter.OutboundManager) *StatisticsTracker {
	return &StatisticsTracker{
		manager:         manager,
		outboundManager: outboundManager,
	}
}

func (t *StatisticsTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	return trafficontrol.NewTCPTracker(conn, t.manager, metadata, t.outboundManager, matchedRule, matchOutbound)
}

func (t *StatisticsTracker) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	return trafficontrol.NewUDPTracker(conn, t.manager, metadata, t.outboundManager, matchedRule, matchOutbound)
}
