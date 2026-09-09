package command

import (
	"context"
	"testing"
)

// TestTestDial_UnparseablePrivateKeyFailsBeforeAnyDial confirms the key is
// parsed before any network I/O is attempted — same short-circuit
// reasoning TestSSHExecutor_Execute_DeniedCommandNeverDials uses, avoiding
// a real network dependency in this test.
func TestTestDial_UnparseablePrivateKeyFailsBeforeAnyDial(t *testing.T) {
	err := TestDial(context.Background(), "203.0.113.1", 22, "user", "not a real private key")
	if err == nil {
		t.Fatal("expected an error for an unparseable private key")
	}
}
