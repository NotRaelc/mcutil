package status_test

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/mcstatus-io/mcutil/v4/options"
	"github.com/mcstatus-io/mcutil/v4/status"
)

func deafTCPListener(t *testing.T) net.Listener {
	t.Helper()
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
				buf := make([]byte, 4096)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return l
}

func deafUDPConn(t *testing.T) net.PacketConn {
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

// --- Modern ---

func TestModernCancel(t *testing.T) {
	l := deafTCPListener(t)
	host, port := splitAddr(t, l.Addr())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := status.Modern(ctx, host, port, options.StatusModern{
		Timeout: 10 * time.Second, // заведомо больше реального ожидания
		Ping:    true,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Modern ignored ctx cancellation: %s", elapsed)
	}
}

func TestModernDeadline(t *testing.T) {
	l := deafTCPListener(t)
	host, port := splitAddr(t, l.Addr())

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := status.Modern(ctx, host, port, options.StatusModern{
		Timeout: 10 * time.Second,
		Ping:    true,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Modern ignored ctx deadline: %s", elapsed)
	}
}

func TestModernTimeoutFallback(t *testing.T) {
	l := deafTCPListener(t)
	host, port := splitAddr(t, l.Addr())

	// ctx без дедлайна: opts.Timeout должен сработать.
	start := time.Now()
	_, err := status.Modern(context.Background(), host, port, options.StatusModern{
		Timeout: 200 * time.Millisecond,
		Ping:    true,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > time.Second {
		t.Fatalf("Modern ignored opts.Timeout fallback: %s", elapsed)
	}
}

// --- Legacy ---

func TestLegacyCancel(t *testing.T) {
	l := deafTCPListener(t)
	host, port := splitAddr(t, l.Addr())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := status.Legacy(ctx, host, port, options.StatusLegacy{
		Timeout: 10 * time.Second,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Legacy ignored ctx cancellation: %s", elapsed)
	}
}

// --- Bedrock ---

func TestBedrockCancel(t *testing.T) {
	c := deafUDPConn(t)
	host, port := splitAddr(t, c.LocalAddr())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := status.Bedrock(ctx, host, port, options.StatusBedrock{
		Timeout: 10 * time.Second,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Bedrock ignored ctx cancellation: %s", elapsed)
	}
}

// --- ModernRaw ---

func TestModernRawCancel(t *testing.T) {
	l := deafTCPListener(t)
	host, port := splitAddr(t, l.Addr())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := status.ModernRaw(ctx, host, port, options.StatusModern{
		Timeout: 10 * time.Second,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("ModernRaw ignored ctx cancellation: %s", elapsed)
	}
}
