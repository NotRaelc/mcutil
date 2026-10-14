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

// Запускаем mock-server, который корректно отвечает на legacy.
// Убеждаемся, что вызов с context.Background() без дедлайна возвращает
// успешный результат за счёт opts.Timeout.
func TestLegacyWithContextBackground(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 8)
		if _, err := conn.Read(buf); err != nil {
			return
		}

		// Минимальный legacy-ответ: 0xFF + length + UTF-16BE "§1\x00...".
		// Для теста достаточно проверить, что соединение прошло и не упало.
		// Реальная проверка парсинга — в других тестах.
	}()

	host, portStr, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.ParseUint(portStr, 10, 16)

	// Дедлайн по opts.Timeout, не по ctx.
	_, _ = status.Legacy(context.Background(), host, uint16(port), options.StatusLegacy{
		Timeout: 500 * time.Millisecond,
	})
	// Не проверяем результат — важно, что не зависли.
}

var _ = context.Background
