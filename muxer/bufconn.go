package muxer

import (
	"bufio"
	"net"
	"time"
)

type bufconn struct {
	conn           net.Conn
	bufferedWriter *bufio.Writer
}

func newBufconn(conn net.Conn, bufSize int) *bufconn {
	return &bufconn{
		conn:           conn,
		bufferedWriter: bufio.NewWriterSize(conn, bufSize),
	}
}

func (c *bufconn) Close() error {
	return c.conn.Close()
}
func (c *bufconn) LocalAddr() net.Addr  { return c.LocalAddr() }
func (c *bufconn) RemoteAddr() net.Addr { return c.RemoteAddr() }

func (c *bufconn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *bufconn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *bufconn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }

func (c *bufconn) Read(b []byte) (n int, err error) { return c.conn.Read(b) }

func (c *bufconn) Write(b []byte) (n int, err error) {
	return c.bufferedWriter.Write(b)
}
