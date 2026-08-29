package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestRouterRegister(t *testing.T) {
	router := NewRouter(logging.Logger{})
	handler := func(context.Context, json.RawMessage) (any, error) { return nil, nil }

	for _, test := range []struct {
		name    string
		nameArg string
		h       Handler
	}{
		{name: "empty name", h: handler},
		{name: "nil handler", nameArg: "status"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := router.Register(test.nameArg, test.h)
			var registration *RegistrationError
			if !errors.As(err, &registration) {
				t.Fatalf("Register() error = %T %v, want *RegistrationError", err, err)
			}
		})
	}

	if err := router.Register("status", handler); err != nil {
		t.Fatal(err)
	}
	if err := router.Register("status", handler); err == nil {
		t.Fatal("duplicate Register() error = nil")
	}
	if got, want := router.Commands(), []string{"status"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("Commands() = %v, want %v", got, want)
	}
}

func TestRouterCommandsSorted(t *testing.T) {
	router := NewRouter(logging.Logger{})
	for _, name := range []string{"tasks", "dispatch", "status"} {
		if err := router.Register(name, func(context.Context, json.RawMessage) (any, error) { return nil, nil }); err != nil {
			t.Fatal(err)
		}
	}
	got := router.Commands()
	want := []string{"dispatch", "status", "tasks"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("Commands() = %v, want %v", got, want)
	}
}

func TestRouterHandle(t *testing.T) {
	var log bytes.Buffer
	router := NewRouter(*logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: &log}))
	if err := router.Register("status", func(_ context.Context, params json.RawMessage) (any, error) {
		if string(params) != `{"full":true}` {
			return nil, fmt.Errorf("params = %s", params)
		}
		return wire.StatusResult{Version: "test", Schema: wire.Schema}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := router.Register("missing", func(context.Context, json.RawMessage) (any, error) {
		return nil, fmt.Errorf("task: %w", ErrNotFound)
	}); err != nil {
		t.Fatal(err)
	}
	if err := router.Register("panic", func(context.Context, json.RawMessage) (any, error) {
		panic("bad input")
	}); err != nil {
		t.Fatal(err)
	}
	if err := router.Register("unencodable", func(context.Context, json.RawMessage) (any, error) {
		return make(chan int), nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		req  wire.Request
		code wire.Code
	}{
		{name: "schema", req: wire.Request{Schema: wire.Schema + 1, ID: "schema"}, code: wire.CodeSchemaMismatch},
		{name: "command", req: wire.Request{Schema: wire.Schema, ID: "empty"}, code: wire.CodeBadRequest},
		{name: "unknown", req: wire.Request{Schema: wire.Schema, ID: "unknown", Command: "nope"}, code: wire.CodeUnknownCommand},
		{name: "handler error", req: wire.Request{Schema: wire.Schema, ID: "missing", Command: "missing"}, code: wire.CodeNotFound},
		{name: "handler panic", req: wire.Request{Schema: wire.Schema, ID: "panic", Command: "panic"}, code: wire.CodeInternal},
		{name: "marshal", req: wire.Request{Schema: wire.Schema, ID: "marshal", Command: "unencodable"}, code: wire.CodeInternal},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := router.Handle(context.Background(), test.req)
			if response.Schema != wire.Schema || response.ID != test.req.ID || response.Err == nil || response.Err.Code != test.code {
				t.Fatalf("Handle() = %#v, want id %q code %q", response, test.req.ID, test.code)
			}
		})
	}

	response := router.Handle(context.Background(), wire.Request{Schema: wire.Schema, ID: "ok", Command: "status", Params: json.RawMessage(`{"full":true}`)})
	if response.Err != nil || response.ID != "ok" || response.Schema != wire.Schema || string(response.Result) != `{"version":"test","schema":1,"running":false,"draining":false,"repos":0,"implementer_cap":0,"sessions":null}` {
		t.Fatalf("successful Handle() = %#v", response)
	}
	if lines := bytes.Count(log.Bytes(), []byte{'\n'}); lines != 1 {
		t.Fatalf("panic logs = %d, want 1: %s", lines, log.String())
	}
}

func TestClassify(t *testing.T) {
	for _, test := range []struct {
		err  error
		code wire.Code
	}{
		{ErrBadRequest, wire.CodeBadRequest},
		{fmt.Errorf("wrapped: %w", ErrNotFound), wire.CodeNotFound},
		{ErrConflict, wire.CodeConflict},
		{ErrUnavailable, wire.CodeUnavailable},
		{context.Canceled, wire.CodeUnavailable},
		{context.DeadlineExceeded, wire.CodeUnavailable},
		{errors.New("unexpected"), wire.CodeInternal},
	} {
		got := Classify(test.err)
		if got.Code != test.code || got.Message != test.err.Error() {
			t.Fatalf("Classify(%v) = %#v, want code %q and error text", test.err, got, test.code)
		}
	}
	if got := Classify(nil); got != nil {
		t.Fatalf("Classify(nil) = %#v, want nil", got)
	}
}

func TestRouterHandleConcurrent(t *testing.T) {
	router := NewRouter(logging.Logger{})
	entered := make(chan struct{})
	release := make(chan struct{})
	if err := router.Register("wait", func(context.Context, json.RawMessage) (any, error) {
		close(entered)
		<-release
		return map[string]bool{"ok": true}, nil
	}); err != nil {
		t.Fatal(err)
	}

	done := make(chan wire.Response, 1)
	go func() {
		done <- router.Handle(context.Background(), wire.Request{Schema: wire.Schema, Command: "wait"})
	}()
	<-entered
	if err := router.Register("other", func(context.Context, json.RawMessage) (any, error) { return nil, nil }); err != nil {
		t.Fatalf("Register() blocked on handler: %v", err)
	}
	close(release)
	if response := <-done; response.Err != nil {
		t.Fatalf("Handle() = %#v", response)
	}

	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			if response := router.Handle(context.Background(), wire.Request{Schema: wire.Schema, Command: "other"}); response.Err != nil {
				t.Errorf("Handle() = %#v", response)
			}
		}()
	}
	group.Wait()
}
