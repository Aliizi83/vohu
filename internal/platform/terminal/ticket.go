package terminal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ticketTTL bounds how long a minted ticket is redeemable — long enough
// for the browser to open the WebSocket right after minting it, short
// enough that a leaked ticket (browser history, a proxy log) is useless
// within seconds.
const ticketTTL = 30 * time.Second

// TicketStore hands out one-time, short-lived tickets binding a
// (userID, connectionID) pair — the WebSocket upgrade request can't carry
// the caller's real Bearer token (browsers don't let a WebSocket handshake
// set custom headers), so a ticket minted over a normal authenticated
// request stands in for it instead. Redeeming a ticket deletes it, so a
// captured URL is worthless after the first (and only) use.
type TicketStore struct {
	redis *redis.Client
}

func NewTicketStore(client *redis.Client) *TicketStore {
	return &TicketStore{redis: client}
}

func ticketKey(ticket string) string { return "terminal:ticket:" + ticket }

func (s *TicketStore) Issue(ctx context.Context, userID, connectionID uint) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(b)

	value := fmt.Sprintf("%d:%d", userID, connectionID)
	if err := s.redis.Set(ctx, ticketKey(ticket), value, ticketTTL).Err(); err != nil {
		return "", err
	}
	return ticket, nil
}

// Redeem consumes a ticket — a second call with the same ticket, or one
// past its TTL, always fails.
func (s *TicketStore) Redeem(ctx context.Context, ticket string) (userID, connectionID uint, err error) {
	value, err := s.redis.Get(ctx, ticketKey(ticket)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, 0, errors.New("terminal: ticket not found or expired")
	}
	if err != nil {
		return 0, 0, err
	}
	s.redis.Del(ctx, ticketKey(ticket))

	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return 0, 0, errors.New("terminal: malformed ticket value")
	}
	uid, err1 := strconv.ParseUint(parts[0], 10, 64)
	cid, err2 := strconv.ParseUint(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, errors.New("terminal: malformed ticket value")
	}
	return uint(uid), uint(cid), nil
}
