package services

import (
	"context"
	"log"
	"sync/atomic"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/transport"
	"golang.org/x/time/rate"
)

type XrayDispatcher struct {
	routing.Dispatcher
	tracker *StatisticsTracker
}

func NewXrayDispatcher(d routing.Dispatcher, tracker *StatisticsTracker) *XrayDispatcher {
	return &XrayDispatcher{
		Dispatcher: d,
		tracker:    tracker,
	}
}

func (d *XrayDispatcher) Dispatch(ctx context.Context, dest net.Destination) (*transport.Link, error) {
	log.Printf("[XrayDispatcher] Dispatch called for dest: %s", dest.String())
	link, err := d.Dispatcher.Dispatch(ctx, dest)
	if err != nil {
		return nil, err
	}

	var email string
	// InboundSourceObject is *protocol.User in some contexts?
	// According to Xray code:
	// if u, ok := content.Attributes["InboundUser"]; ok { ... }
	// Or content.InboundUser but that failed.
	// Let's use what search said: InboundFromContext

	inbound := session.InboundFromContext(ctx)
	if inbound == nil {
		if content := session.ContentFromContext(ctx); content == nil {
			log.Printf("[XrayDispatcher] ContentFromContext is nil")
		} else {
			keys := []string{}
			for k := range content.Attributes {
				keys = append(keys, k)
			}
			log.Printf("[XrayDispatcher] InboundFromContext is nil. Content Attributes: %v", keys)
		}
	} else if inbound.User == nil {
		log.Printf("[XrayDispatcher] Inbound.User is nil. Source: %v, Tag: %s", inbound.Source, inbound.Tag)
	} else {
		email = inbound.User.Email
		log.Printf("[XrayDispatcher] Found user: %s", email)
	}

	// FALLBACK DEBUGGING: Always wrap, just label as unknown if missing
	if email == "" {
		log.Printf("[XrayDispatcher] User not found in context. Defaulting to 'unknown'.")
		email = "unknown"
		// Check fallback attributes
		if content := session.ContentFromContext(ctx); content != nil {
			if uVal, ok := content.Attributes["InboundUser"]; ok {
				log.Printf("[XrayDispatcher] Found InboundUser in attributes: %T", uVal)
			}
		}
	}

	limiter := d.tracker.GetLimiterForUser(email)
	stats := GetXrayUserStats(email)

	if limiter != nil {
		log.Printf("[XrayDispatcher] Rate limiting user: %s, limit: %.2f", email, limiter.Limit())
	}

	if limiter == nil && stats == nil {
		return link, nil
	}

	log.Printf("[XrayDispatcher] Wrapping connection for user: %s", email)

	// We need to construct a new Link that wraps the Reader/Writer
	newLink := &transport.Link{
		Reader: link.Reader,
		Writer: link.Writer,
	}

	// Wrap Reader (Downlink? Upstream to client?)
	// Link.Reader is what we read from upstream (remote). Writing to client.
	// Actually, Dispatch returns a Link to write to outbound and read from outbound.
	// So Link.Writer is writing to outbound (Uplink). Link.Reader is reading from outbound (Downlink).

	if link.Reader != nil {
		newLink.Reader = &RateLimitedReader{
			Reader:  link.Reader,
			limiter: limiter,
			stats:   stats,
		}
	}

	if link.Writer != nil {
		newLink.Writer = &RateLimitedWriter{
			Writer:  link.Writer,
			limiter: limiter,
			stats:   stats,
		}
	}

	return newLink, nil
}

func (d *XrayDispatcher) DispatchLink(ctx context.Context, dest net.Destination, link *transport.Link) error {
	log.Printf("[XrayDispatcher] DispatchLink called for dest: %s", dest.String())
	inbound := session.InboundFromContext(ctx)
	if inbound != nil && inbound.User != nil {
		log.Printf("[XrayDispatcher] DispatchLink Found user: %s", inbound.User.Email)
	}
	return d.Dispatcher.DispatchLink(ctx, dest, link)
}

// Xray 1.8+ uses Type(), older used something else.
func (d *XrayDispatcher) Type() interface{} {
	return routing.DispatcherType()
}

func (d *XrayDispatcher) Start() error {
	log.Printf("[XrayDispatcher] Start called")
	return d.Dispatcher.Start()
}

func (d *XrayDispatcher) Close() error {
	log.Printf("[XrayDispatcher] Close called")
	return d.Dispatcher.Close()
}

type RateLimitedWriter struct {
	buf.Writer
	limiter *rate.Limiter
	stats   *XrayTrafficStats
}

func (w *RateLimitedWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	len := int64(mb.Len())
	if len > 0 {
		if w.limiter != nil {
			// Split wait if len > burst to avoid error
			burst := w.limiter.Burst()
			remaining := int(len)
			for remaining > 0 {
				waitN := remaining
				if waitN > burst {
					waitN = burst
				}
				w.limiter.WaitN(context.Background(), waitN)
				remaining -= waitN
			}
		}
		if w.stats != nil {
			atomic.AddInt64(&w.stats.Up, len)
		}
	}
	return w.Writer.WriteMultiBuffer(mb)
}

type RateLimitedReader struct {
	buf.Reader
	limiter *rate.Limiter
	stats   *XrayTrafficStats
}

func (r *RateLimitedReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	mb, err := r.Reader.ReadMultiBuffer()
	if !mb.IsEmpty() {
		len := int64(mb.Len())
		if r.limiter != nil {
			// Split wait if len > burst to avoid error
			burst := r.limiter.Burst()
			remaining := int(len)
			for remaining > 0 {
				waitN := remaining
				if waitN > burst {
					waitN = burst
				}
				r.limiter.WaitN(context.Background(), waitN)
				remaining -= waitN
			}
		}
		if r.stats != nil {
			atomic.AddInt64(&r.stats.Down, len)
		}
	}
	return mb, err
}
