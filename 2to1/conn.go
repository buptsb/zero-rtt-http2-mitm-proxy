package http2to1

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

var (
	// connectionPreface is the constant value of the connection preface.
	// https://tools.ietf.org/html/rfc7540#section-3.5
	connectionPreface = []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
)

type H2AdaptorConn struct {
	decoder  *hpack.Decoder
	peekBuf  *bytes.Buffer
	writeBuf *bytes.Buffer

	h2conn        net.Conn
	h2ConnCreated chan struct{}
}

func NewH2AdaptorConn() net.Conn {
	c := &H2AdaptorConn{
		decoder:       hpack.NewDecoder(4096, nil),
		peekBuf:       bytes.NewBuffer(nil),
		writeBuf:      bytes.NewBuffer(nil),
		h2ConnCreated: make(chan struct{}),
	}
	return c
}

func (c *H2AdaptorConn) tryPeekHeaders() ([]byte, error) {
	bufCopy := bytes.NewBuffer(c.peekBuf.Bytes())
	framer := http2.NewFramer(nil, bufCopy)
	streamHeaderBufs := make(map[uint32]*bytes.Buffer)
	for {
		f, err := framer.ReadFrame()
		if err != nil {
			log.Println("== read frame err:", err, c.peekBuf)
			return nil, nil
		}
		log.Printf("== read frame: %+v \n", f)
		switch f := f.(type) {
		case *http2.HeadersFrame:
			buf, ok := streamHeaderBufs[f.StreamID]
			if !ok {
				buf = bytes.NewBuffer(nil)
				streamHeaderBufs[f.StreamID] = buf
			}
			buf.Write(f.HeaderBlockFragment())
			if f.HeadersEnded() {
				return buf.Bytes(), nil
			}
		case *http2.ContinuationFrame:
			buf, ok := streamHeaderBufs[f.StreamID]
			if !ok {
				return nil, fmt.Errorf("continuation frame received before headers frame")
			}
			buf.Write(f.HeaderBlockFragment())
			if f.HeadersEnded() {
				return buf.Bytes(), nil
			}
		}
	}
}

func (c *H2AdaptorConn) onHeadersBuf(headersBuf []byte) error {
	decoder := hpack.NewDecoder(4096, nil)
	headers, err := decoder.DecodeFull(headersBuf)
	if err != nil {
		return err
	}
	log.Println("== headers", headers)
	authority := getValueByKeyFromHeaders(headers, ":authority")
	scheme := getValueByKeyFromHeaders(headers, ":scheme")
	if authority == "" || scheme == "" {
		return fmt.Errorf("authority or scheme not found in headers")
	}

	h2conn, _, err := c.dialHTTP2Conn(authority, scheme)
	if err != nil {
		return err
	}
	// send cached frames
	buf := bytes.NewBuffer(nil)
	buf.Write(connectionPreface)
	buf.Write(c.peekBuf.Bytes())
	if _, err := h2conn.Write(buf.Bytes()); err != nil {
		return err
	}
	// h2conn established
	c.h2conn = h2conn
	close(c.h2ConnCreated)
	return nil
}

func (c *H2AdaptorConn) dialHTTP2Conn(host, scheme string) (net.Conn, string, error) {
	if scheme != "https" && scheme != "http" {
		return nil, "", errors.New("unexpected scheme: " + scheme)
	}
	dialFn := func() (net.Conn, error) {
		switch scheme {
		case "https":
			return tls.Dial("tcp", fmt.Sprintf("%s:443", host), &tls.Config{
				NextProtos: []string{"http/1.1"},
			})
		case "http":
			return net.Dial("tcp", fmt.Sprintf("%s:80", host))
		}
		panic("unreachable")
	}

	switch scheme {
	case "https":
		protocolCh := make(chan string, 1)
		tlsConfig := &tls.Config{
			NextProtos: []string{"http/1.1", "h2"},
			VerifyConnection: func(cs tls.ConnectionState) error {
				log.Println("== tls connection NegotiatedProtocol:", cs.NegotiatedProtocol)
				protocolCh <- cs.NegotiatedProtocol
				return nil
			},
		}
		tlsConn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", host), tlsConfig)
		if err != nil {
			return nil, "", err
		}
		var h2conn net.Conn
		// TODO: add timeout and failure handling
		protocol := <-protocolCh
		switch protocol {
		case "h2":
			h2conn = tlsConn
		case "http/1.1":
			h2conn = newHttp2OverHttp1Conn(tlsConn, dialFn)
		default:
			return nil, "", fmt.Errorf("unexpected protocol: %s", protocol)
		}
		return h2conn, protocol, nil
	case "http":
		return newHttp2OverHttp1Conn(nil, dialFn), "http/1.1", nil
	}
	panic("unreachable")
}

func (c *H2AdaptorConn) Write(buf []byte) (int, error) {
	// h2 preface
	if bytes.Equal(buf, connectionPreface) {
		return len(buf), nil
	}

	if c.h2conn == nil {
		n, err := c.peekBuf.Write(buf)
		if err != nil {
			return 0, err
		}
		headersBuf, err := c.tryPeekHeaders()
		if err != nil {
			fmt.Println("== tryPeekHeaders err:", err)
			return 0, err
		}
		if headersBuf != nil {
			if err := c.onHeadersBuf(headersBuf); err != nil {
				return 0, err
			}
		}
		return n, nil
	}
	return c.h2conn.Write(buf)
}

func (c *H2AdaptorConn) Close() error         { return nil }
func (c *H2AdaptorConn) LocalAddr() net.Addr  { panic("not implemented") }
func (c *H2AdaptorConn) RemoteAddr() net.Addr { panic("not implemented") }

func (c *H2AdaptorConn) SetDeadline(t time.Time) error      { panic("not implemented") }
func (c *H2AdaptorConn) SetReadDeadline(t time.Time) error  { panic("not implemented") }
func (c *H2AdaptorConn) SetWriteDeadline(t time.Time) error { panic("not implemented") }

func (c *H2AdaptorConn) Read(buf []byte) (n int, err error) {
	<-c.h2ConnCreated
	return c.h2conn.Read(buf)
}
