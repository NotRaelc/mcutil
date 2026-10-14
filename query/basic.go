package query

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/NotRaelc/mcutil/v4/formatting"
	"github.com/NotRaelc/mcutil/v4/options"
	"github.com/NotRaelc/mcutil/v4/response"
)

// Basic runs a query on the server and returns basic information.
//
// ctx cancels the request. opts.Timeout is used as a fallback upper bound
// when ctx has no deadline of its own.
func Basic(ctx context.Context, hostname string, port uint16, options ...options.Query) (*response.QueryBasic, error) {
	return performBasicQuery(ctx, hostname, port, options...)
}

func performBasicQuery(ctx context.Context, hostname string, port uint16, options ...options.Query) (*response.QueryBasic, error) {
	opts := parseQueryOptions(options...)

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

	if err := writeHandshakeRequest(conn, opts.SessionID); err != nil {
		return nil, err
	}
	challengeToken, err := readHandshakeResponse(r, opts.SessionID)
	if err != nil {
		return nil, err
	}
	if err := writeBasicStatRequest(conn, opts.SessionID, challengeToken); err != nil {
		return nil, err
	}
	return readBasicStatResponse(r, opts.SessionID)
}

func writeBasicStatRequest(w io.Writer, sessionID int32, challengeToken int32) error {
	buf := &bytes.Buffer{}

	if _, err := buf.Write(magic); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, byte(0x00)); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, sessionID&0x0F0F0F0F); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, challengeToken); err != nil {
		return err
	}
	if _, err := io.Copy(w, buf); err != nil {
		return err
	}
	return nil
}

func readBasicStatResponse(r io.Reader, sessionID int32) (*response.QueryBasic, error) {
	{
		var packetType byte
		if err := binary.Read(r, binary.BigEndian, &packetType); err != nil {
			return nil, err
		}
		if packetType != 0x00 {
			return nil, fmt.Errorf("query: received unexpected packet type (expected=0x00, received=0x%02X)", packetType)
		}
	}
	{
		var serverSessionID int32
		if err := binary.Read(r, binary.BigEndian, &serverSessionID); err != nil {
			return nil, err
		}
		if serverSessionID != sessionID {
			return nil, fmt.Errorf("query: session ID mismatch (expected=%d, received=%d)", sessionID, serverSessionID)
		}
	}

	var resp response.QueryBasic

	{
		value, err := readNTString(r)
		if err != nil {
			return nil, err
		}
		motd, err := formatting.Parse(value)
		if err != nil {
			return nil, err
		}
		resp.MOTD = *motd
	}
	{
		value, err := readNTString(r)
		if err != nil {
			return nil, err
		}
		resp.GameType = value
	}
	{
		value, err := readNTString(r)
		if err != nil {
			return nil, err
		}
		resp.Map = value
	}
	{
		value, err := readNTString(r)
		if err != nil {
			return nil, err
		}
		onlinePlayers, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, err
		}
		resp.OnlinePlayers = onlinePlayers
	}
	{
		value, err := readNTString(r)
		if err != nil {
			return nil, err
		}
		maxPlayers, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, err
		}
		resp.MaxPlayers = maxPlayers
	}
	{
		var value uint16
		if err := binary.Read(r, binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		resp.HostPort = value
	}
	{
		value, err := readNTString(r)
		if err != nil {
			return nil, err
		}
		resp.HostIP = value
	}

	return &resp, nil
}
