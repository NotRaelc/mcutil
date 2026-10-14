package status

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mcstatus-io/mcutil/v4/formatting"
	"github.com/mcstatus-io/mcutil/v4/options"
	"github.com/mcstatus-io/mcutil/v4/response"
)

var (
	defaultBedrockStatusOptions = options.StatusBedrock{
		Timeout:           time.Second * 5,
		ClientGUID:        0,
		ReceiveLimitBytes: 1 << 20, // 1 Megabyte
	}
	bedrockMagic = []byte{0x00, 0xFF, 0xFF, 0x00, 0xFE, 0xFE, 0xFE, 0xFE, 0xFD, 0xFD, 0xFD, 0xFD, 0x12, 0x34, 0x56, 0x78}
)

// Bedrock retrieves the status of a Bedrock Edition Minecraft server.
//
// ctx cancels the request. opts.Timeout is used as a fallback upper bound
// when ctx has no deadline of its own.
func Bedrock(ctx context.Context, hostname string, port uint16, options ...options.StatusBedrock) (*response.StatusBedrock, error) {
	return getStatusBedrock(ctx, hostname, port, options...)
}

func getStatusBedrock(ctx context.Context, hostname string, port uint16, options ...options.StatusBedrock) (*response.StatusBedrock, error) {
	opts := parseBedrockStatusOptions(options...)

	dialer := net.Dialer{Timeout: opts.Timeout}
	conn, err := dialer.DialContext(ctx, "udp", fmt.Sprintf("%s:%d", hostname, port))
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

	r := bufio.NewReader(io.LimitReader(conn, opts.ReceiveLimitBytes))

	// Unconnected ping packet
	// https://wiki.vg/Raknet_Protocol#Unconnected_Ping
	{
		buf := &bytes.Buffer{}
		if err := buf.WriteByte(0x01); err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.BigEndian, time.Now().UnixMilli()); err != nil {
			return nil, err
		}
		if _, err := buf.Write(bedrockMagic); err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.BigEndian, opts.ClientGUID); err != nil {
			return nil, err
		}
		if _, err := io.Copy(conn, buf); err != nil {
			return nil, err
		}
	}

	var (
		serverGUID   int64
		serverID     string
		receiveStart = time.Now()
		latency      time.Duration
	)

	// Unconnected pong packet
	// https://wiki.vg/Raknet_Protocol#Unconnected_Pong
	{
		var packetType byte
		if err := binary.Read(r, binary.BigEndian, &packetType); err != nil {
			return nil, err
		}
		if packetType != 0x1C {
			return nil, fmt.Errorf("statusbedrock: received unexpected packet type (expected=0x1C, received=0x%02X)", packetType)
		}

		latency = time.Since(receiveStart)

		var t int64
		if err := binary.Read(r, binary.BigEndian, &t); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.BigEndian, &serverGUID); err != nil {
			return nil, err
		}

		data := make([]byte, 16)
		if _, err := r.Read(data); err != nil {
			return nil, err
		}

		var length uint16
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		id := make([]byte, length)
		if _, err := r.Read(id); err != nil {
			return nil, err
		}
		serverID = string(id)
	}

	resp := response.StatusBedrock{
		ServerGUID: serverGUID,
		Latency:    latency,
	}

	splitID := strings.Split(serverID, ";")
	var motd string

	for k, value := range splitID {
		if len(strings.Trim(value, " ")) < 1 {
			continue
		}

		switch k {
		case 0:
			resp.Edition = pointerOf(value)
		case 1:
			motd = value
		case 2:
			protocolVersion, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			resp.ProtocolVersion = &protocolVersion
		case 3:
			resp.Version = pointerOf(value)
		case 4:
			onlinePlayers, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			resp.OnlinePlayers = &onlinePlayers
		case 5:
			maxPlayers, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			resp.MaxPlayers = &maxPlayers
		case 6:
			resp.ServerID = pointerOf(value)
		case 7:
			motd += "\n" + value
		case 8:
			resp.Gamemode = pointerOf(value)
		case 9:
			gamemodeID, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			resp.GamemodeID = &gamemodeID
		case 10:
			portIPv4, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			v := uint16(portIPv4)
			resp.PortIPv4 = &v
		case 11:
			portIPv6, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			resp.PortIPv6 = pointerOf(uint16(portIPv6))
		}
	}

	if len(motd) > 0 {
		parsed, err := formatting.Parse(motd)
		if err != nil {
			return nil, err
		}
		resp.MOTD = parsed
	}

	return &resp, nil
}

func parseBedrockStatusOptions(opts ...options.StatusBedrock) options.StatusBedrock {
	if len(opts) < 1 {
		o := options.StatusBedrock(defaultBedrockStatusOptions)
		o.ClientGUID = rand.Int63()
		return o
	}
	return opts[0]
}
