package server

import (
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/wire"
)

func TestProjectLiftsEventFields(t *testing.T) {
	at := time.Date(2026, time.August, 28, 20, 0, 0, 0, time.UTC)
	event, ok := Project("info", "land", map[string]string{
		"repo": "magicite",
		"task": "magicite-9pt.3",
		"seat": "ifrit",
		"time": at.Format(time.RFC3339Nano),
		"step": "merge",
	})
	if !ok {
		t.Fatal("Project refused land")
	}
	if event.Schema != 0 || event.Seq != 0 || !event.Time.Equal(at) || event.Kind != wire.KindLand || event.Level != "info" {
		t.Errorf("projected envelope = %#v", event)
	}
	if event.Repo != "magicite" || event.Task != "magicite-9pt.3" || event.Seat != "ifrit" {
		t.Errorf("projected columns = %#v", event)
	}
	if len(event.Fields) != 1 || event.Fields["step"] != "merge" {
		t.Errorf("projected fields = %#v", event.Fields)
	}
}

func TestProjectUnknownKind(t *testing.T) {
	if _, ok := Project("info", "other", nil); ok {
		t.Error("info unknown kind projected")
	}
	for _, level := range []string{"warn", "error"} {
		event, ok := Project(level, "other", nil)
		if !ok || event.Kind != wire.KindWarn || event.Level != level {
			t.Errorf("Project(%q, other) = %#v, %t", level, event, ok)
		}
	}
}

func TestProjectWithoutTimeLeavesZero(t *testing.T) {
	event, ok := Project("info", "pickup", map[string]string{"message": "ready"})
	if !ok || !event.Time.IsZero() {
		t.Errorf("Project without time = %#v, %t", event, ok)
	}
}
