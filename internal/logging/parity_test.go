package logging_test

import (
	"bytes"
	"sort"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/parity"
)

type parityPanicJSON struct{}

func (parityPanicJSON) MarshalJSON() ([]byte, error) { panic("malformed") }

func TestMaduinLoggingParity(t *testing.T) {
	for _, name := range loggingParityNames() {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			logger := logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: &output})
			logger.Event(logging.Level(255), "land\ncomplete", map[string]any{"bad": make(chan int), "panic": parityPanicJSON{}, "text": "100% wrong"})
			line := output.String()
			if strings.Count(line, "\n") != 1 || !strings.Contains(line, "100% wrong") || !strings.Contains(line, "land\\ncomplete") {
				t.Fatalf("Event() = %q", line)
			}
		})
	}
}

func loggingParityNames() []string {
	counterparts := parity.SubstrateCounterparts()
	names := make([]string, 0, 12)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinLoggingParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
