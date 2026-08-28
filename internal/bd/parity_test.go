package bd_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/parity"
	"github.com/connorfranc/magicite/internal/testenv"
)

func TestMaduinBDParity(t *testing.T) {
	for _, name := range bdParityNames() {
		t.Run(name, func(t *testing.T) {
			env := testenv.New(t)
			fake := testenv.NewBD(t, env)
			fake.Seed()
			if err := bd.Classify("show", nil, bd.Result{}); err != nil {
				t.Fatalf("exit-zero result classified: %v", err)
			}
			if _, err := bd.DecodeBeads([]byte(`[{"id":"t1","title":"native JSON"}]`)); err != nil {
				t.Fatalf("DecodeBeads() = %v", err)
			}
			if _, err := bd.DecodeBeads([]byte(`{"error":"x"}`)); err == nil {
				t.Fatal("DecodeBeads accepted error object")
			}
			if !bd.IsNotFound(bd.Classify("show", nil, bd.Result{ExitCode: 1, Stdout: []byte(`{"error":"missing"}`)})) {
				t.Fatal("semantic negative was not classified")
			}
		})
	}
}

func bdParityNames() []string {
	counterparts := parity.SubstrateCounterparts()
	names := make([]string, 0, 14)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinBDParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
