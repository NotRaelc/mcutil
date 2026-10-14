package query

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	"github.com/NotRaelc/mcutil/v4/options"
	"github.com/NotRaelc/mcutil/v4/response"
)

// Full runs a query on the server and returns the full information.
//
// ctx cancels the request. opts.Timeout is used as a fallback upper bound
// when ctx has no deadline of its own.
func Full(ctx context.Context, hostname string, port uint16, options ...options.Query) (*response.QueryFull, error) {
	return performFullQuery(ctx, hostname, port, options...)
}

func performFullQuery(ctx context.Context, hostname string, port uint16, options ...options.Query) (*response.QueryFull, error) {
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
	if err := writeFullStatRequest(conn, opts.SessionID, challengeToken); err != nil {
		return nil, err
	}
	return readFullStatResponse(r, opts.SessionID)
}

func writeFullStatRequest(w io.Writer, sessionID int32, challengeToken int32) error {
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
	if _, err := buf.Write([]byte{0x00, 0x00, 0x00, 0x00}); err != nil {
		return err
	}
	if _, err := io.Copy(w, buf); err != nil {
		return err
	}
	return nil
}

func readFullStatResponse(r io.Reader, sessionID int32) (*response.QueryFull, error) {
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
	{
		data := make([]byte, 11)
		if _, err := r.Read(data); err != nil {
			return nil, err
		}
	}

	resp := response.QueryFull{
		Data:    make(map[string]string),
		Players: make([]string, 0),
	}

	{
		for {
			key, err := readNTString(r)
			if err != nil {
				return nil, err
			}
			if len(key) < 1 {
				break
			}
			value, err := readNTString(r)
			if err != nil {
				return nil, err
			}
			resp.Data[key] = value
		}
	}
	{
		data := make([]byte, 10)
		if _, err := r.Read(data); err != nil {
			return nil, err
		}
	}
	{
		for {
			username, err := readNTString(r)
			if err != nil {
				return nil, err
			}
			if len(username) < 1 {
				break
			}
			resp.Players = append(resp.Players, username)
		}
	}

	return &resp, nil
}

func parseQueryOptions(opts ...options.Query) options.Query {
	if len(opts) < 1 {
		o := options.Query(defaultQueryOptions)
		o.SessionID = rand.Int31() & 0x0F0F0F0F
		return o
	}

	result := opts[0]
	result.SessionID &= 0x0F0F0F0F
	return result
}
