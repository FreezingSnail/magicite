package stamp

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/parity"
)

func TestMaduinStampParity(t *testing.T) {
	for _, name := range stampParityNames() {
		t.Run(name, func(t *testing.T) {
			message := Apply("subject\n\nMagicite-Task: old\n", Stamp{Model: "model", Repo: "repo", Task: "task"}.Trailers())
			if again := Apply(message, Stamp{Model: "model", Repo: "repo", Task: "task"}.Trailers()); again != message {
				t.Fatal("Apply is not idempotent")
			}
			want := []Trailer{{Key: KeyModel, Value: "model"}, {Key: KeyRepo, Value: "repo"}, {Key: KeyTask, Value: "task"}}
			if got := Parse(message); !reflect.DeepEqual(got, want) {
				t.Fatalf("Parse(Apply()) = %#v, want %#v", got, want)
			}
		})
	}
}

func stampParityNames() []string {
	counterparts := parity.OrchestrationCounterparts()
	names := make([]string, 0)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinStampParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
