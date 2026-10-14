package vote_test

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/NotRaelc/mcutil/v4/options"
	"github.com/NotRaelc/mcutil/v4/vote"
)

func TestSendVoteCancel(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// Не отвечаем на handshake.
				buf := make([]byte, 4096)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	host, portStr, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.ParseUint(portStr, 10, 16)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err = vote.SendVote(ctx, host, uint16(port), options.Vote{
		ServiceName: "test",
		Username:    "user",
		Token:       "token",
		Timeout:     10 * time.Second,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("SendVote ignored ctx cancellation: %s", elapsed)
	}
}
