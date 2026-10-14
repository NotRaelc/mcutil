package rcon_test

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/mcstatus-io/mcutil/v4/options"
	"github.com/mcstatus-io/mcutil/v4/rcon"
)

func TestDialContextCancel(t *testing.T) {
	// 10.255.255.1 — обычно недостижимый адрес, заставит Dial висеть.
	// Используем его только для проверки отмены ctx.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := rcon.DialContext(ctx, "10.255.255.1", 25575, options.RCON{
		Timeout: 10 * time.Second,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("DialContext ignored ctx cancellation: %s", elapsed)
	}
}

func TestDialContextSuccess(t *testing.T) {
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
			_ = conn.Close()
		}
	}()

	host, portStr, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.ParseUint(portStr, 10, 16)

	client, err := rcon.DialContext(context.Background(), host, uint16(port), options.RCON{
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("DialContext error = %v", err)
	}
	defer client.Close()
}

func TestDialCompatibility(t *testing.T) {
	// Deprecated Dial должен работать через context.Background().
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
			_ = conn.Close()
		}
	}()

	host, portStr, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.ParseUint(portStr, 10, 16)

	client, err := rcon.Dial(host, uint16(port), options.RCON{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("Dial error = %v", err)
	}
	defer client.Close()
}
