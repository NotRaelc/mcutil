package status

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/NotRaelc/mcutil/v4/formatting"
	"github.com/NotRaelc/mcutil/v4/options"
	"github.com/NotRaelc/mcutil/v4/response"
	"github.com/NotRaelc/mcutil/v4/util"
)

var defaultJavaStatusLegacyOptions = options.StatusLegacy{
	EnableSRV:         true,
	Timeout:           time.Second * 5,
	ReceiveLimitBytes: 1 << 20, // 1 Megabyte
}

// Legacy retrieves the status of any Java Edition Minecraft server, but with reduced properties compared to Modern().
//
// ctx cancels the request. opts.Timeout is used as a fallback upper bound
// when ctx has no deadline of its own.
func Legacy(ctx context.Context, hostname string, port uint16, options ...options.StatusLegacy) (*response.StatusLegacy, error) {
	return getStatusLegacy(ctx, hostname, port, options...)
}

func getStatusLegacy(ctx context.Context, hostname string, port uint16, options ...options.StatusLegacy) (*response.StatusLegacy, error) {
	var (
		opts                                   = parseJavaStatusLegacyOptions(options...)
		connectionHostname                     = hostname
		connectionPort     uint16              = port
		srvRecord          *response.SRVRecord = nil
	)

	if opts.EnableSRV && port == util.DefaultJavaPort && net.ParseIP(connectionHostname) == nil {
		record, err := util.LookupSRVContext(ctx, hostname)
		if err == nil && record != nil {
			connectionHostname = record.Target
			connectionPort = record.Port
			srvRecord = &response.SRVRecord{Host: record.Target, Port: record.Port}
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

	// Client to server packet
	// https://wiki.vg/Server_List_Ping#Client_to_server
	if _, err := conn.Write([]byte{0xFE, 0x01}); err != nil {
		return nil, err
	}

	var (
		receiveStart = time.Now()
		latency      time.Duration
	)

	// Server to client packet
	// https://wiki.vg/Server_List_Ping#Server_to_client
	{
		var packetType byte
		if err := binary.Read(r, binary.BigEndian, &packetType); err != nil {
			return nil, err
		}
		if packetType != 0xFF {
			return nil, fmt.Errorf("status: received unexpected packet type (expected=0xFF, received=0x%02X)", packetType)
		}

		latency = time.Since(receiveStart)

		var packetLength uint16
		if err := binary.Read(r, binary.BigEndian, &packetLength); err != nil {
			return nil, err
		}
		if packetLength < 2 {
			return nil, fmt.Errorf("status: received status response with no data (bytes=%d)", packetLength)
		}

		data := make([]uint16, packetLength)
		if err := binary.Read(r, binary.BigEndian, &data); err != nil {
			return nil, err
		}
		result := string(utf16.Decode(data))

		if data[0] == 0x00A7 && data[1] == 0x0031 {
			// 1.4+ server
			split := strings.Split(result, "\x00")
			if len(split) < 6 {
				return nil, fmt.Errorf("status: not enough information received (expected=6, received=%d)", len(split))
			}

			protocolVersion, err := strconv.ParseInt(split[1], 10, 32)
			if err != nil {
				return nil, err
			}
			versionTree, err := formatting.Parse(split[2])
			if err != nil {
				return nil, err
			}
			motd, err := formatting.Parse(split[3])
			if err != nil {
				return nil, err
			}
			onlinePlayers, err := strconv.ParseInt(split[4], 10, 32)
			if err != nil {
				return nil, err
			}
			maxPlayers, err := strconv.ParseInt(split[5], 10, 32)
			if err != nil {
				return nil, err
			}

			return &response.StatusLegacy{
				Version: &response.Version{
					Name:     *versionTree,
					Protocol: protocolVersion,
				},
				Players: response.LegacyPlayers{
					Online: onlinePlayers,
					Max:    maxPlayers,
				},
				MOTD:      *motd,
				SRVRecord: srvRecord,
				Latency:   latency,
			}, nil
		}

		// < 1.4 server
		split := strings.Split(result, "\u00A7")
		if len(split) < 3 {
			return nil, fmt.Errorf("status: not enough information received (expected=3, received=%d)", len(split))
		}

		motd, err := formatting.Parse(split[0])
		if err != nil {
			return nil, err
		}
		onlinePlayers, err := strconv.ParseInt(split[1], 10, 32)
		if err != nil {
			return nil, err
		}
		maxPlayers, err := strconv.ParseInt(split[2], 10, 32)
		if err != nil {
			return nil, err
		}

		return &response.StatusLegacy{
			Version: nil,
			Players: response.LegacyPlayers{
				Online: onlinePlayers,
				Max:    maxPlayers,
			},
			MOTD:      *motd,
			SRVRecord: srvRecord,
		}, nil
	}
}

func parseJavaStatusLegacyOptions(opts ...options.StatusLegacy) options.StatusLegacy {
	if len(opts) < 1 {
		return defaultJavaStatusLegacyOptions
	}
	return opts[0]
}
