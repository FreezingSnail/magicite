package server

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/wire"
)

func TestRegisterRead(t *testing.T) {
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
			err := RegisterRead(test.r, test.c)
			var registration *ReadRegistrationError
			if !errors.As(err, &registration) {
				t.Fatalf("RegisterRead() error = %T %v, want *ReadRegistrationError", err, err)
			}
		})
	}

	router := NewRouter(logging.Logger{})
	if err := RegisterRead(router, core); err != nil {
		t.Fatal(err)
	}
	if got, want := router.Commands(), []string{"repos", "seats", "status", "tasks"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Commands() = %v, want %v", got, want)
	}
	if err := RegisterRead(router, core); err == nil {
		t.Fatal("second RegisterRead() error = nil")
	} else {
		var registration *RegistrationError
		if !errors.As(err, &registration) || registration.Name != "status" {
			t.Fatalf("second RegisterRead() error = %T %v, want status RegistrationError", err, err)
		}
	}
}

func TestReadNoParams(t *testing.T) {
	core := &fakeCore{
		status: wire.StatusResult{Version: "test"},
		seats:  []wire.SeatResult{},
		repos:  []wire.RepoResult{},
	}
	router := readRouter(t, core)
	for _, command := range []string{"status", "seats", "repos"} {
		t.Run(command, func(t *testing.T) {
			for _, params := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`)} {
				response := handleRead(router, command, params)
				if response.Err != nil {
					t.Fatalf("%s params %s = %#v", command, params, response)
				}
			}
			for _, params := range []json.RawMessage{json.RawMessage(`[]`), json.RawMessage(`{"extra":true}`), json.RawMessage(`true`)} {
				response := handleRead(router, command, params)
				if response.Err == nil || response.Err.Code != wire.CodeBadRequest || !strings.Contains(response.Err.Message, command) {
					t.Fatalf("%s params %s = %#v, want named bad request", command, params, response)
				}
			}
		})
	}
}

func TestReadOrdering(t *testing.T) {
	core := &fakeCore{
		status: wire.StatusResult{Sessions: []wire.SessionResult{
			{Handle: "z", UptimeSeconds: 2}, {Handle: "a", UptimeSeconds: 2}, {Handle: "old", UptimeSeconds: 1},
		}},
		seats: []wire.SeatResult{{Role: "review", Name: "z"}, {Role: "implement", Name: "z"}, {Role: "implement", Name: "a"}},
		tasks: []wire.TaskResult{{Repo: "z", Priority: 1, ID: "a"}, {Repo: "a", Priority: 2, ID: "a"}, {Repo: "a", Priority: 1, ID: "z"}, {Repo: "a", Priority: 1, ID: "a"}},
		repos: []wire.RepoResult{{Name: "z"}, {Name: "a"}},
	}
	router := readRouter(t, core)

	response := handleRead(router, "status", nil)
	var status wire.StatusResult
	decodeReadResult(t, response, &status)
	if got, want := []string{status.Sessions[0].Handle, status.Sessions[1].Handle, status.Sessions[2].Handle}, []string{"a", "z", "old"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("status sessions = %v, want %v", got, want)
	}

	response = handleRead(router, "seats", nil)
	var seats []wire.SeatResult
	decodeReadResult(t, response, &seats)
	if got, want := []string{seats[0].Name, seats[1].Name, seats[2].Name}, []string{"a", "z", "z"}; !reflect.DeepEqual(got, want) || seats[2].Role != "review" {
		t.Fatalf("seats = %#v", seats)
	}

	response = handleRead(router, "tasks", json.RawMessage(`{"repo":"a","all":true}`))
	var tasks []wire.TaskResult
	decodeReadResult(t, response, &tasks)
	if got, want := []string{tasks[0].ID, tasks[1].ID, tasks[2].ID, tasks[3].ID}, []string{"a", "z", "a", "a"}; !reflect.DeepEqual(got, want) || tasks[2].Repo != "a" || tasks[3].Repo != "z" {
		t.Fatalf("tasks = %#v", tasks)
	}
	calls := core.Calls()
	if got, want := calls[len(calls)-1].Params, (wire.TasksParams{Repo: "a", All: true}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Tasks params = %#v, want %#v", got, want)
	}

	response = handleRead(router, "repos", nil)
	var repos []wire.RepoResult
	decodeReadResult(t, response, &repos)
	if got, want := []string{repos[0].Name, repos[1].Name}, []string{"a", "z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("repos = %v, want %v", got, want)
	}
}

func TestReadTasksErrorsAndEmptyArrays(t *testing.T) {
	core := &fakeCore{}
	router := readRouter(t, core)
	for _, params := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`), json.RawMessage(`{"repo":"magicite"}`), json.RawMessage(`{"all":true}`)} {
		response := handleRead(router, "tasks", params)
		if response.Err != nil {
			t.Fatalf("tasks params %s = %#v", params, response)
		}
		if string(response.Result) != "[]" {
			t.Fatalf("tasks empty result = %s, want []", response.Result)
		}
	}
	response := handleRead(router, "tasks", json.RawMessage(`{"unknown":true}`))
	if response.Err == nil || response.Err.Code != wire.CodeBadRequest || !strings.Contains(response.Err.Message, `"unknown"`) {
		t.Fatalf("tasks unknown parameter = %#v, want quoted bad request", response)
	}

	for _, command := range []string{"seats", "repos"} {
		response = handleRead(router, command, nil)
		if string(response.Result) != "[]" {
			t.Fatalf("%s empty result = %s, want []", command, response.Result)
		}
	}
	response = handleRead(router, "status", nil)
	var status wire.StatusResult
	decodeReadResult(t, response, &status)
	if status.Sessions == nil {
		t.Fatal("status empty sessions = nil, want empty slice")
	}

	core.tasksErr = ErrNotFound
	response = handleRead(router, "tasks", json.RawMessage(`{"repo":"missing"}`))
	if response.Err == nil || response.Err.Code != wire.CodeNotFound || response.Err.Message != ErrNotFound.Error() {
		t.Fatalf("unknown repo = %#v, want not found", response)
	}
	core.tasksErr = errors.New("core failed")
	response = handleRead(router, "tasks", nil)
	if response.Err == nil || response.Err.Code != wire.CodeInternal || response.Err.Message != "core failed" {
		t.Fatalf("unexpected core error = %#v, want internal", response)
	}
}

func readRouter(t *testing.T, core Core) *Router {
	t.Helper()
	router := NewRouter(logging.Logger{})
	if err := RegisterRead(router, core); err != nil {
		t.Fatal(err)
	}
	return router
}

func handleRead(router *Router, command string, params json.RawMessage) wire.Response {
	return router.Handle(tContext, wire.Request{Schema: wire.Schema, ID: command, Command: command, Params: params})
}

var tContext = context.Background()

func decodeReadResult(t *testing.T, response wire.Response, target any) {
	t.Helper()
	if response.Err != nil {
		t.Fatalf("response error = %#v", response.Err)
	}
	if err := json.Unmarshal(response.Result, target); err != nil {
		t.Fatalf("json.Unmarshal(%s): %v", response.Result, err)
	}
}
