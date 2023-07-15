package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var (
	DebugMode  = false
	LogFactory log.Factory
)

func NewLogger(name string) log.ContextLogger {
	if LogFactory == nil {
		f, err := log.New(log.Options{
			Options: option.LogOptions{
				Timestamp: true,
			},
		})
		if err != nil {
			panic(err)
		}
		LogFactory = f
	}
	return LogFactory.NewLogger(name)
}

func SpawnPprofServer(port int) {
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
	if err != nil {
		panic(err)
	}
}

func IsIgnoredError(err error) bool {
	return errors.Is(err, context.Canceled)
}

func IsNetCancelError(err error) bool {
	return strings.Contains(err.Error(), "operation was canceled")
}

// A peekedConn subverts the net.Conn.Read implementation, primarily so that
// sniffed bytes can be transparently prepended.
type peekedConn struct {
	net.Conn
	r io.Reader
}

// Read allows control over the embedded net.Conn's read data. By using an
// io.MultiReader one can read from a conn, and then replace what they read, to
// be read again.
func (c *peekedConn) Read(buf []byte) (int, error) { return c.r.Read(buf) }
