package status

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	"github.com/NotRaelc/mcutil/v4/options"
	"github.com/NotRaelc/mcutil/v4/util"
)

// ModernRaw returns the raw status data of any 1.7+ Java Edition Minecraft server.
//
// ctx cancels the request. opts.Timeout is used as a fallback upper bound
// when ctx has no deadline of its own.
func ModernRaw(ctx context.Context, hostname string, port uint16, options ...options.StatusModern) (map[string]any, error) {
	return getStatusRaw(ctx, hostname, port, options...)
}

func getStatusRaw(ctx context.Context, hostname string, port uint16, options ...options.StatusModern) (map[string]any, error) {
	var (
		opts                              = parseJavaStatusOptions(options...)
		connectionHostname                = hostname
		connectionPort     uint16         = port
		result             map[string]any = make(map[string]any)
		payload            int64          = rand.Int63()
	)

	if opts.EnableSRV && port == util.DefaultJavaPort && net.ParseIP(connectionHostname) == nil {
		record, err := util.LookupSRVContext(ctx, hostname)
		if err == nil && record != nil {
			connectionHostname = record.Target
			connectionPort = record.Port
		}
	}

	dialer := net.Dialer{Timeout: opts.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", connectionHostname, connectionPort))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	defer stop()

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		if err := conn.SetDeadline(time.Now().Add(opts.Timeout)); err != nil {
			return nil, err
		}
	}

	r := io.LimitReader(conn, opts.ReceiveLimitBytes)

	if err := writeJavaStatusHandshakePacket(conn, int32(opts.ProtocolVersion), connectionHostname, connectionPort); err != nil {
		return nil, err
	}
	if err := writeJavaStatusStatusRequestPacket(conn); err != nil {
		return nil, err
	}
	if err := readJavaStatusStatusResponsePacket(r, &result); err != nil {
		return nil, err
	}
	if err := writeJavaStatusPingPacket(conn, payload); err != nil {
		return nil, err
	}
	if err := readJavaStatusPongPacket(r, payload); err != nil {
		return nil, err
	}
	return result, nil
}
