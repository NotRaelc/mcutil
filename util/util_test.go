package util

import (
	"context"
	"testing"
	"time"
)

func TestLookupSRVContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// _minecraft._tcp.<domain> отсутствует у .invalid по RFC 6761.
	// Resolver вернёт ошибку или nil, но не должен паниковать.
	_, _ = LookupSRVContext(ctx, "nonexistent.invalid")
}

func TestLookupSRVContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := LookupSRVContext(ctx, "example.com")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for canceled context")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("LookupSRVContext ignored cancellation: %s", elapsed)
	}
}

func TestLookupSRVDeprecated(t *testing.T) {
	// Deprecated wrapper должен работать без паники даже если SRV нет.
	_, _ = LookupSRV("nonexistent.invalid")
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		host    string
		port    *uint16
		wantErr bool
	}{
		{"host with port", "example.com:25565", "example.com", ptr(uint16(25565)), false},
		{"host without port", "example.com", "example.com", nil, false},
		{"bracketed ipv6", "[::1]:25565", "::1", ptr(uint16(25565)), false},
		{"invalid port", "example.com:abc", "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := ParseAddress(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if host != tt.host {
				t.Fatalf("host = %q, want %q", host, tt.host)
			}
			if (port == nil) != (tt.port == nil) {
				t.Fatalf("port = %v, want %v", port, tt.port)
			}
			if port != nil && *port != *tt.port {
				t.Fatalf("port = %d, want %d", *port, *tt.port)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
