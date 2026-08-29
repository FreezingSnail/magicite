// Command fake-agent is a hermetic agent process double for parity tests.
package main

import "github.com/FreezingSnail/magicite/internal/testenv"

func main() { testenv.RunFakeAgent() }
