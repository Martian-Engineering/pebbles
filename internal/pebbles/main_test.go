package pebbles

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Unsetenv("PEBBLES_DIR")
	os.Exit(m.Run())
}

