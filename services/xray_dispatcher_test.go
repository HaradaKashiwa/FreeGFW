package services

import (
	"context"
	"testing"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/transport"
)

// Mock Dispatcher
type mockDispatcher struct{}

func (m *mockDispatcher) Dispatch(ctx context.Context, dest net.Destination) (*transport.Link, error) {
	// Create a pipe to simulate a link
	return &transport.Link{
		Reader: &mockReader{},
		Writer: &mockWriter{},
	}, nil
}

func (m *mockDispatcher) DispatchLink(ctx context.Context, dest net.Destination, link *transport.Link) error {
	return nil
}

func (m *mockDispatcher) Start() error {
	return nil
}

func (m *mockDispatcher) Close() error {
	return nil
}

func (m *mockDispatcher) Type() interface{} {
	return routing.DispatcherType()
}

// Mock Reader/Writer to satisfy buf.Reader/Writer interfaces
type mockReader struct{}

func (r *mockReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	return buf.MultiBuffer{}, nil
}

type mockWriter struct{}

func (w *mockWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	for _, b := range mb {
		b.Release()
	}
	return nil
}

func TestXrayDispatcher_Dispatch_RateLimiting(t *testing.T) {
	// 1. Setup Tracker
	limits := map[string]uint64{
		"test_user@example.com": 1024 * 1024, // 1MB/s
	}
	tracker := NewStatisticsTracker(nil, nil, limits)

	// 2. Setup Dispatcher
	mock := &mockDispatcher{}
	xd := NewXrayDispatcher(mock, tracker)

	// 3. Setup Context with User
	// session.Inbound uses *protocol.MemoryUser
	user := &protocol.MemoryUser{
		Email: "test_user@example.com",
	}
	inbound := &session.Inbound{
		User: user,
	}
	ctx := session.ContextWithInbound(context.Background(), inbound)

	// 4. Test Dispatch
	dest := net.Destination{}
	link, err := xd.Dispatch(ctx, dest)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	// 5. Check if link is wrapped
	if link == nil {
		t.Fatal("Link is nil")
	}

	if _, ok := link.Reader.(*RateLimitedReader); !ok {
		t.Error("Reader is not wrapped with RateLimitedReader")
	} else {
		t.Log("Reader successfully wrapped")
	}

	if _, ok := link.Writer.(*RateLimitedWriter); !ok {
		t.Error("Writer is not wrapped with RateLimitedWriter")
	} else {
		t.Log("Writer successfully wrapped")
	}

	// 6. Test Traffic Stats Update
	// Reset stats for the user just in case
	XrayUserTrafficMutex.Lock()
	if stats, ok := XrayUserTraffic["test_user@example.com"]; ok {
		stats.Up = 0
		stats.Down = 0
	}
	XrayUserTrafficMutex.Unlock()

	stats := GetXrayUserStats("test_user@example.com")
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	// Create some data
	data := make([]byte, 100)
	b := buf.New()
	b.Write(data)
	mb := make(buf.MultiBuffer, 0, 1)
	mb = append(mb, b)

	// Write data (Uplink)
	err = link.Writer.WriteMultiBuffer(mb)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if stats.Up != 100 {
		t.Errorf("Expected Up stats to be 100, got %d", stats.Up)
	} else {
		t.Logf("Up stats updated correctly: %d", stats.Up)
	}

	// Read data (Downlink) - Mock Reader returns empty so we can't easily test Read stats without complex mock,
	// but the wrapping logic is identical to Writer.
}

func TestXrayDispatcher_Dispatch_NoUser(t *testing.T) {
	tracker := NewStatisticsTracker(nil, nil, nil)
	mock := &mockDispatcher{}
	xd := NewXrayDispatcher(mock, tracker)

	// Context without user
	ctx := context.Background()

	link, err := xd.Dispatch(ctx, net.Destination{})
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if _, ok := link.Reader.(*RateLimitedReader); ok {
		t.Error("Reader should NOT be wrapped when no user is present")
	}

	if _, ok := link.Writer.(*RateLimitedWriter); ok {
		t.Error("Writer should NOT be wrapped when no user is present")
	}
}
