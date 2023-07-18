package common

import (
	"io"
	"net"
)

// A peekedConn subverts the net.Conn.Read implementation, primarily so that
// sniffed bytes can be transparently prepended.
type PeekedConn struct {
	net.Conn
	r io.Reader
}

func NewPeekedConn(c net.Conn, r io.Reader) net.Conn {
	return &PeekedConn{
		Conn: c,
		r:    r,
	}
}

// Read allows control over the embedded net.Conn's read data. By using an
// io.MultiReader one can read from a conn, and then replace what they read, to
// be read again.
func (c *PeekedConn) Read(buf []byte) (int, error) { return c.r.Read(buf) }
