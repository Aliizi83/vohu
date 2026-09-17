package terminal_test

import (
	"context"
	"testing"
	"time"

	"github.com/Aliizi83/vohu/internal/platform/terminal"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T) (*terminal.TicketStore, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	return terminal.NewTicketStore(client), mr
}

func TestIssue_ThenRedeem_ReturnsTheSamePair(t *testing.T) {
	store, _ := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ticket, err := store.Issue(ctx, 7, 42)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	userID, connectionID, err := store.Redeem(ctx, ticket)
	if err != nil {
		t.Fatalf("Redeem failed: %v", err)
	}
	if userID != 7 || connectionID != 42 {
		t.Fatalf("expected (7, 42), got (%d, %d)", userID, connectionID)
	}
}

func TestRedeem_SecondTimeFails(t *testing.T) {
	store, _ := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ticket, err := store.Issue(ctx, 1, 2)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}
	if _, _, err := store.Redeem(ctx, ticket); err != nil {
		t.Fatalf("first Redeem failed: %v", err)
	}

	if _, _, err := store.Redeem(ctx, ticket); err == nil {
		t.Fatal("expected the second Redeem of the same ticket to fail")
	}
}

func TestRedeem_UnknownTicketFails(t *testing.T) {
	store, _ := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, _, err := store.Redeem(ctx, "does-not-exist"); err == nil {
		t.Fatal("expected redeeming an unknown ticket to fail")
	}
}

func TestRedeem_ExpiredTicketFails(t *testing.T) {
	store, mr := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ticket, err := store.Issue(ctx, 1, 2)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}
	mr.FastForward(31 * time.Second)

	if _, _, err := store.Redeem(ctx, ticket); err == nil {
		t.Fatal("expected redeeming an expired ticket to fail")
	}
}
