package state

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/parity"
)

func TestMaduinStateParity(t *testing.T) {
	for _, name := range stateParityNames() {
		t.Run(name, func(t *testing.T) {
			now := time.Date(2026, time.August, 28, 0, 0, 0, 0, time.UTC)
			store := New(time.Second, func() time.Time { return now })
			store.Put("pipeline", "snapshot")
			if value, fresh := store.Get("pipeline"); !fresh || value != "snapshot" {
				t.Fatalf("Get before expiry = %#v, %t", value, fresh)
			}
			now = now.Add(time.Second)
			if value, fresh := store.Get("pipeline"); fresh || value != nil {
				t.Fatalf("Get at TTL boundary = %#v, %t", value, fresh)
			}
			store.Put("titles", "cache")
			store.Invalidate("pipeline")
			store.Reset()
			if _, fresh := store.Get("titles"); fresh {
				t.Fatal("Reset retained snapshot")
			}
		})
	}
}

func stateParityNames() []string {
	counterparts := parity.SubstrateCounterparts()
	names := make([]string, 0, 5)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinStateParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
