package query_test

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/mcstatus-io/mcutil/v4/query"
)

func deafUDP(t *testing.T) net.PacketConn {
	t.Helper()
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, _, err := c.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
	return c
}

func splitAddr(t *testing.T, addr net.Addr) (string, uint16) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	return host, uint16(port)
}

func TestBasicCancel(t *testing.T) {
	c := deafUDP(t)
	host, port := splitAddr(t, c.LocalAddr())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := query.Basic(ctx, host, port)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Basic ignored ctx cancellation: %s", elapsed)
	}
}

func TestFullCancel(t *testing.T) {
	c := deafUDP(t)
	host, port := splitAddr(t, c.LocalAddr())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := query.Full(ctx, host, port)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Full ignored ctx cancellation: %s", elapsed)
	}
}
