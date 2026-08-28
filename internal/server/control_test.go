package server

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestRegisterControl(t *testing.T) {
	core := &fakeCore{}
	for _, test := range []struct {
		name string
		r    *Router
		c    Core
	}{
		{name: "nil router", c: core},
		{name: "nil core", r: NewRouter(logging.Logger{})},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := RegisterControl(test.r, test.c)
			var registration *ControlRegistrationError
			if !errors.As(err, &registration) {
				t.Fatalf("RegisterControl() error = %T %v, want *ControlRegistrationError", err, err)
			}
		})
	}

	router := NewRouter(logging.Logger{})
	if err := RegisterControl(router, core); err != nil {
		t.Fatal(err)
	}
	if got, want := router.Commands(), []string{"dispatch", "review", "start", "stop"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Commands() = %v, want %v", got, want)
	}
	if err := RegisterControl(router, core); err == nil {
		t.Fatal("second RegisterControl() error = nil")
	} else {
		var registration *RegistrationError
		if !errors.As(err, &registration) || registration.Name != "start" {
			t.Fatalf("second RegisterControl() error = %T %v, want start RegistrationError", err, err)
		}
	}
}

func TestControlStart(t *testing.T) {
	core := &fakeCore{start: wire.StatusResult{Version: "test", Schema: wire.Schema, Running: true}}
	router := controlRouter(t, core)

	for range 2 {
		response := handleControl(router, "start", nil)
		if response.Err != nil {
			t.Fatalf("start response error = %#v", response.Err)
		}
		var result wire.StatusResult
		decodeControlResult(t, response, &result)
		if !result.Running {
			t.Fatalf("start result = %#v, want running", result)
		}
	}
	if calls := core.Calls(); len(calls) != 2 || calls[0].Method != "Start" || calls[1].Method != "Start" {
		t.Fatalf("Start calls = %#v, want two Start calls", calls)
	}

	response := handleControl(router, "start", json.RawMessage(`{"extra":true}`))
	if response.Err == nil || response.Err.Code != wire.CodeBadRequest {
		t.Fatalf("start unknown params = %#v, want bad request", response)
	}
}

func TestControlStop(t *testing.T) {
	core := &fakeCore{stop: wire.StopResult{Mode: "drain", Sessions: 3, Draining: true}}
	router := controlRouter(t, core)

	response := handleControl(router, "stop", json.RawMessage(`{}`))
	if response.Err != nil {
		t.Fatalf("soft stop response error = %#v", response.Err)
	}
	var soft wire.StopResult
	decodeControlResult(t, response, &soft)
	if soft.Mode != "drain" || soft.Sessions != 3 || !soft.Draining {
		t.Fatalf("soft stop = %#v", soft)
	}

	core.stop = wire.StopResult{Mode: "hard", Sessions: 2}
	response = handleControl(router, "stop", json.RawMessage(`{"hard":true}`))
	if response.Err != nil {
		t.Fatalf("hard stop response error = %#v", response.Err)
	}
	var hard wire.StopResult
	decodeControlResult(t, response, &hard)
	if hard.Mode != "hard" || hard.Sessions != 2 || hard.Draining {
		t.Fatalf("hard stop = %#v", hard)
	}

	calls := core.Calls()
	if got, want := calls[0].Params, (wire.StopParams{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("soft stop params = %#v, want %#v", got, want)
	}
	if got, want := calls[1].Params, (wire.StopParams{Hard: true}); !reflect.DeepEqual(got, want) {
		t.Fatalf("hard stop params = %#v, want %#v", got, want)
	}
	response = handleControl(router, "stop", json.RawMessage(`{"unknown":true}`))
	if response.Err == nil || response.Err.Code != wire.CodeBadRequest {
		t.Fatalf("stop unknown params = %#v, want bad request", response)
	}
}

func TestControlDispatch(t *testing.T) {
	core := &fakeCore{dispatch: wire.DispatchResult{Handle: "h1", Task: "magicite-1", Role: "implement"}}
	router := controlRouter(t, core)

	for _, params := range []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`{"task":"magicite-1","role":"unknown"}`),
		json.RawMessage(`{"task":"magicite-1","role":"implement","unknown":true}`),
	} {
		response := handleControl(router, "dispatch", params)
		if response.Err == nil || response.Err.Code != wire.CodeBadRequest {
			t.Fatalf("dispatch params %s = %#v, want bad request", params, response)
		}
	}
	response := handleControl(router, "dispatch", json.RawMessage(`{"task":"magicite-1","repo":"magicite","role":"implement"}`))
	if response.Err != nil {
		t.Fatalf("dispatch response error = %#v", response.Err)
	}
	var result wire.DispatchResult
	decodeControlResult(t, response, &result)
	if result.Handle != "h1" {
		t.Fatalf("dispatch result = %#v", result)
	}

	core.dispatchErr = ErrConflict
	response = handleControl(router, "dispatch", json.RawMessage(`{"task":"magicite-1","role":"implement"}`))
	if response.Err == nil || response.Err.Code != wire.CodeConflict || response.Err.Message != ErrConflict.Error() {
		t.Fatalf("claimed dispatch = %#v, want conflict", response)
	}
	core.dispatchErr = ErrNotFound
	response = handleControl(router, "dispatch", json.RawMessage(`{"task":"missing","role":"implement"}`))
	if response.Err == nil || response.Err.Code != wire.CodeNotFound || response.Err.Message != ErrNotFound.Error() {
		t.Fatalf("unknown dispatch = %#v, want not found", response)
	}
}

func TestControlReview(t *testing.T) {
	core := &fakeCore{review: wire.ReviewResult{Epic: "magicite-epic", Handle: "h2"}}
	router := controlRouter(t, core)

	for _, params := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{"epic":"magicite-epic","unknown":true}`)} {
		response := handleControl(router, "review", params)
		if response.Err == nil || response.Err.Code != wire.CodeBadRequest {
			t.Fatalf("review params %s = %#v, want bad request", params, response)
		}
	}
	response := handleControl(router, "review", json.RawMessage(`{"epic":"magicite-epic","repo":"magicite"}`))
	if response.Err != nil {
		t.Fatalf("review response error = %#v", response.Err)
	}
	var result wire.ReviewResult
	decodeControlResult(t, response, &result)
	if result.Handle != "h2" {
		t.Fatalf("review result = %#v", result)
	}

	core.reviewErr = ErrConflict
	response = handleControl(router, "review", json.RawMessage(`{"epic":"magicite-epic"}`))
	if response.Err == nil || response.Err.Code != wire.CodeConflict || response.Err.Message != ErrConflict.Error() {
		t.Fatalf("disabled review = %#v, want conflict", response)
	}
	core.reviewErr = ErrNotFound
	response = handleControl(router, "review", json.RawMessage(`{"epic":"missing"}`))
	if response.Err == nil || response.Err.Code != wire.CodeNotFound || response.Err.Message != ErrNotFound.Error() {
		t.Fatalf("unknown review = %#v, want not found", response)
	}
}

func controlRouter(t *testing.T, core Core) *Router {
	t.Helper()
	router := NewRouter(logging.Logger{})
	if err := RegisterControl(router, core); err != nil {
		t.Fatal(err)
	}
	return router
}

func handleControl(router *Router, command string, params json.RawMessage) wire.Response {
	return router.Handle(context.Background(), wire.Request{Schema: wire.Schema, ID: command, Command: command, Params: params})
}

func decodeControlResult(t *testing.T, response wire.Response, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Result, target); err != nil {
		t.Fatalf("json.Unmarshal(%s): %v", response.Result, err)
	}
}

func TestControlBadRoleListsAcceptedRoles(t *testing.T) {
	router := controlRouter(t, &fakeCore{})
	response := handleControl(router, "dispatch", json.RawMessage(`{"task":"magicite-1","role":"writer"}`))
	if response.Err == nil || !strings.Contains(response.Err.Message, "implement, design, repair, review") {
		t.Fatalf("bad role response = %#v, want accepted roles", response)
	}
}
