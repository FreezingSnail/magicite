package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/connorfranc/magicite/internal/cli"
)

func TestMainUsesCLI(t *testing.T) {
	var out, err bytes.Buffer
	if code := cli.Run(context.Background(), []string{"--help"}, &out, &err); code != 0 {
		t.Fatalf("Run() = %d, want 0; stderr = %q", code, err.String())
	}
}
