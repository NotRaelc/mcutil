package vote

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/mcstatus-io/mcutil/v4/options"
)

var (
	ErrPublicKeyRequired = errors.New("vote: server negotiated Votifier 1, but PublicKey option is empty")
	ErrInvalidPublicKey  = errors.New("vote: invalid public key value")
	ErrTokenRequired     = errors.New("vote: server negotiated Votifier 2, but Token option is empty")
)

type voteMessage struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type votePayload struct {
	ServiceName string `json:"serviceName"`
	Username    string `json:"username"`
	Address     string `json:"address"`
	Timestamp   int64  `json:"timestamp"`
	Challenge   string `json:"challenge"`
	UUID        string `json:"uuid,omitempty"`
}

type voteResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// SendVote sends a Votifier vote to the specified Minecraft server.
//
// ctx cancels the request. opts.Timeout is used as a fallback upper bound
// when ctx has no deadline of its own.
func SendVote(ctx context.Context, host string, port uint16, opts options.Vote) error {
	return sendVote(ctx, host, port, opts)
}

func sendVote(ctx context.Context, host string, port uint16, opts options.Vote) error {
	dialer := net.Dialer{Timeout: opts.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	defer conn.Close()

	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	defer stop()

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		if err := conn.SetDeadline(time.Now().Add(opts.Timeout)); err != nil {
			return err
		}
	}

	r := bufio.NewReader(conn)

	var (
		challenge    string
		version      string
		majorVersion string
	)

	// Handshake packet
	// https://github.com/NuVotifier/NuVotifier/wiki/Technical-QA#handshake
	{
		data, err := r.ReadBytes('\n')
		if err != nil {
			return err
		}
		segments := strings.Split(string(data[:len(data)-1]), " ")
		if len(segments) < 2 {
			return fmt.Errorf("vote: server sent invalid handshake packet with value: %s", data[:len(data)-1])
		}
		version = segments[1]
		majorVersion = strings.Split(version, ".")[0]
		if len(segments) > 2 {
			challenge = segments[2]
		}
	}

	if majorVersion != "2" && majorVersion != "1" {
		return fmt.Errorf("vote: unknown Votifier version: %s", version)
	}

	if majorVersion == "2" && len(opts.Token) > 0 {
		return sendVotifier2Vote(r, conn, host, port, challenge, opts)
	}
	if len(opts.PublicKey) > 0 {
		return sendVotifier1Vote(conn, opts)
	}
	return fmt.Errorf("vote: version mismatch (server is expecting a Votifier %s packet, but options are missing to allow sending of this vote packet or any prior backwards-compatible versions)", version)
}

func sendVotifier1Vote(conn net.Conn, opts options.Vote) error {
	if len(opts.IPAddress) < 1 {
		opts.IPAddress = "127.0.0.1"
	}

	block, _ := pem.Decode([]byte(fmt.Sprintf("-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----", opts.PublicKey)))
	if block == nil {
		return ErrInvalidPublicKey
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("vote: parsed invalid key type: %T", key)
	}
	payload := fmt.Sprintf(
		"VOTE\n%s\n%s\n%s\n%s",
		opts.ServiceName,
		opts.Username,
		opts.IPAddress,
		opts.Timestamp.Format(time.RFC3339),
	)
	encryptedPayload, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey, []byte(payload))
	if err != nil {
		return err
	}
	if _, err := conn.Write(encryptedPayload); err != nil {
		return err
	}
	return nil
}

func sendVotifier2Vote(r *bufio.Reader, conn net.Conn, host string, port uint16, challenge string, opts options.Vote) error {
	{
		buf := &bytes.Buffer{}
		payload := votePayload{
			ServiceName: opts.ServiceName,
			Username:    opts.Username,
			Address:     fmt.Sprintf("%s:%d", host, port),
			Timestamp:   opts.Timestamp.UnixMilli(),
			Challenge:   challenge,
			UUID:        opts.UUID,
		}
		payloadData, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		hash := hmac.New(sha256.New, []byte(opts.Token))
		hash.Write(payloadData)
		message := voteMessage{
			Payload:   string(payloadData),
			Signature: base64.StdEncoding.EncodeToString(hash.Sum(nil)),
		}
		messageData, err := json.Marshal(message)
		if err != nil {
			return err
		}
		if err := binary.Write(buf, binary.BigEndian, uint16(0x733A)); err != nil {
			return err
		}
		if err := binary.Write(buf, binary.BigEndian, uint16(len(messageData))); err != nil {
			return err
		}
		if _, err := buf.Write(messageData); err != nil {
			return err
		}
		if _, err := io.Copy(conn, buf); err != nil {
			return err
		}
	}

	{
		data, err := r.ReadBytes('\n')
		if err != nil {
			return err
		}
		resp := voteResponse{}
		if err := json.Unmarshal(data[:len(data)-1], &resp); err != nil {
			return err
		}
		switch resp.Status {
		case "ok":
			return nil
		case "error":
			return fmt.Errorf("vote: server returned error: %s", resp.Error)
		default:
			return fmt.Errorf("vote: received unexpected server response (expected=<nil>, received=%s)", resp.Status)
		}
	}
}
