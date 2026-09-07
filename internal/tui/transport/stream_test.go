package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/wire"
)

var streamSocketSequence atomic.Uint64

func TestEventStreamDeliversOrderedEventsAndEOF(t *testing.T) {
	requests := make(chan wire.Request, 1)
	socket := serveStreamSocket(t, func(conn net.Conn) {
		defer conn.Close()
		request, err := wire.NewDecoder(conn).Request()
		if err != nil {
			t.Error(err)
			return
		}
		requests <- request
		encoder := wire.NewEncoder(conn)
		if err := encoder.Encode(wire.Event{Schema: wire.Schema, Seq: 4, Kind: wire.KindPickup}); err != nil {
			t.Error(err)
			return
		}
		if err := encoder.Encode(wire.Event{Schema: wire.Schema, Seq: 5, Kind: wire.KindComplete}); err != nil {
			t.Error(err)
		}
	})

	subscription := NewEventStream(client.New(client.Options{Socket: socket, Timeout: time.Second})).Subscribe(context.Background(), 3)
	if event := <-subscription.Events; event.Seq != 4 || event.Kind != wire.KindPickup {
		t.Fatalf("first event = %#v", event)
	}
	if event := <-subscription.Events; event.Seq != 5 || event.Kind != wire.KindComplete {
		t.Fatalf("second event = %#v", event)
	}
	if notice := <-subscription.Notices; notice.Kind != StreamNoticeEOF || notice.Cursor != 5 {
		t.Fatalf("EOF notice = %#v", notice)
	}
	if subscription.Highest() != 5 {
		t.Fatalf("Highest() = %d, want 5", subscription.Highest())
	}
	request := <-requests
	var params wire.SubscribeParams
	if request.Command != "subscribe" || !decodeStreamParams(request, &params) || params.Since != 3 {
		t.Fatalf("request = %#v", request)
	}
}

func TestEventStreamReportsInitialMissAndLaterGap(t *testing.T) {
	socket := serveStreamSocket(t, func(conn net.Conn) {
		defer conn.Close()
		if _, err := wire.NewDecoder(conn).Request(); err != nil {
			t.Error(err)
			return
		}
		encoder := wire.NewEncoder(conn)
		if err := encoder.Encode(wire.Event{Schema: wire.Schema, Seq: 7, Kind: wire.KindPickup}); err != nil {
			t.Error(err)
			return
		}
		if err := encoder.Encode(wire.Event{Schema: wire.Schema, Seq: 10, Kind: wire.KindLand}); err != nil {
			t.Error(err)
		}
	})

	subscription := NewEventStream(client.New(client.Options{Socket: socket, Timeout: time.Second})).Subscribe(context.Background(), 3)
	if notice := <-subscription.Notices; notice.Kind != StreamNoticeMiss || notice.MissingFrom != 4 || notice.MissingTo != 6 || notice.Cursor != 7 {
		t.Fatalf("miss notice = %#v", notice)
	}
	if event := <-subscription.Events; event.Seq != 7 {
		t.Fatalf("first event = %#v", event)
	}
	if notice := <-subscription.Notices; notice.Kind != StreamNoticeGap || notice.MissingFrom != 8 || notice.MissingTo != 9 || notice.Cursor != 10 {
		t.Fatalf("gap notice = %#v", notice)
	}
	if event := <-subscription.Events; event.Seq != 10 {
		t.Fatalf("second event = %#v", event)
	}
	if notice := <-subscription.Notices; notice.Kind != StreamNoticeEOF || notice.Cursor != 10 {
		t.Fatalf("EOF notice = %#v", notice)
	}
}

func TestEventStreamReportsInvalidOrdering(t *testing.T) {
	socket := serveStreamSocket(t, func(conn net.Conn) {
		defer conn.Close()
		if _, err := wire.NewDecoder(conn).Request(); err != nil {
			t.Error(err)
			return
		}
		encoder := wire.NewEncoder(conn)
		if err := encoder.Encode(wire.Event{Schema: wire.Schema, Seq: 6, Kind: wire.KindPickup}); err != nil {
			t.Error(err)
			return
		}
		if err := encoder.Encode(wire.Event{Schema: wire.Schema, Seq: 6, Kind: wire.KindClose}); err != nil {
			t.Error(err)
		}
	})

	subscription := NewEventStream(client.New(client.Options{Socket: socket, Timeout: time.Second})).Subscribe(context.Background(), 5)
	if event := <-subscription.Events; event.Seq != 6 {
		t.Fatalf("event = %#v", event)
	}
	if notice := <-subscription.Notices; notice.Kind != StreamNoticeGap || notice.Cursor != 6 || notice.Code != wire.CodeInternal {
		t.Fatalf("ordering notice = %#v", notice)
	}
}

func TestEventStreamMapsSchemaAndUnavailable(t *testing.T) {
	t.Run("schema mismatch", func(t *testing.T) {
		socket := serveStreamSocket(t, func(conn net.Conn) {
			defer conn.Close()
			if _, err := wire.NewDecoder(conn).Request(); err != nil {
				t.Error(err)
				return
			}
			_, _ = conn.Write([]byte(`{"schema":2,"seq":1}` + "\n"))
		})
		subscription := NewEventStream(client.New(client.Options{Socket: socket, Timeout: time.Second})).Subscribe(context.Background(), 0)
		if notice := <-subscription.Notices; notice.Kind != StreamNoticeSchemaMismatch || notice.Code != wire.CodeSchemaMismatch {
			t.Fatalf("schema notice = %#v", notice)
		}
	})
	t.Run("unavailable", func(t *testing.T) {
		subscription := NewEventStream(client.New(client.Options{Socket: streamSocketPath(t), Timeout: time.Second})).Subscribe(context.Background(), 0)
		if notice := <-subscription.Notices; notice.Kind != StreamNoticeUnavailable || notice.Code != wire.CodeUnavailable {
			t.Fatalf("unavailable notice = %#v", notice)
		}
	})
}

func TestEventStreamSubscriptionsOwnConnectionsAndCancellationClosesThem(t *testing.T) {
	accepted := make(chan net.Conn, 2)
	closed := make(chan struct{}, 2)
	socket := serveStreamSocket(t, func(conn net.Conn) {
		defer conn.Close()
		if _, err := wire.NewDecoder(conn).Request(); err != nil {
			t.Error(err)
			return
		}
		accepted <- conn
		if _, err := conn.Read(make([]byte, 1)); errors.Is(err, io.EOF) {
			closed <- struct{}{}
		}
	})
	stream := NewEventStream(client.New(client.Options{Socket: socket, Timeout: time.Second}))
	first := stream.Subscribe(context.Background(), 0)
	second := stream.Subscribe(context.Background(), 0)
	firstConnection := <-accepted
	secondConnection := <-accepted
	if firstConnection == secondConnection {
		t.Fatal("subscriptions reused a connection")
	}
	first.Cancel()
	second.Cancel()
	for range 2 {
		select {
		case <-closed:
		case <-time.After(time.Second):
			t.Fatal("cancellation did not close subscription connection")
		}
	}
	for _, subscription := range []*Subscription{first, second} {
		select {
		case _, ok := <-subscription.Events:
			if ok {
				t.Fatal("events remained open after cancellation")
			}
		case <-time.After(time.Second):
			t.Fatal("cancelled subscription did not finish")
		}
	}
}

func serveStreamSocket(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	socket := streamSocketPath(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()
	return socket
}

func streamSocketPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(fmt.Sprintf(".tui-stream-%d.sock", streamSocketSequence.Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func decodeStreamParams(request wire.Request, params *wire.SubscribeParams) bool {
	if request.Params == nil {
		return false
	}
	return json.Unmarshal(request.Params, params) == nil
}
