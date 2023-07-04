package http2to1

import (
	"fmt"
	"net"
	"time"
)

type http2OverHttp1Conn struct {
	http1Conn net.Conn
}

func newHttp2OverHttp1Conn(http1Conn net.Conn) net.Conn {
	return &http2OverHttp1Conn{
		http1Conn: http1Conn,
	}
}

func (c *http2OverHttp1Conn) Close() error         { return nil }
func (c *http2OverHttp1Conn) LocalAddr() net.Addr  { panic("not implemented") }
func (c *http2OverHttp1Conn) RemoteAddr() net.Addr { panic("not implemented") }

func (c *http2OverHttp1Conn) SetDeadline(t time.Time) error      { panic("not implemented") }
func (c *http2OverHttp1Conn) SetReadDeadline(t time.Time) error  { panic("not implemented") }
func (c *http2OverHttp1Conn) SetWriteDeadline(t time.Time) error { panic("not implemented") }

func (c *http2OverHttp1Conn) Read(b []byte) (n int, err error) {
	fmt.Println("=== read", b)
	<-time.After(1 * time.Hour)
	return 0, nil
}
func (c *http2OverHttp1Conn) Write(b []byte) (n int, err error) {
	fmt.Println("=== write", b)
	<-time.After(1 * time.Hour)
	return 0, nil
}
