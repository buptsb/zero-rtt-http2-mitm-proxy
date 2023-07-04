package http2to1

import (
	"log"
	"net"
	"time"

	"github.com/acomagu/bufpipe"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

type http2OverHttp1Conn struct {
	h1connCh chan net.Conn
	dialFn   func() (net.Conn, error)

	decoder         *hpack.Decoder
	outFramer       *http2.Framer
	outFramesWriter *bufpipe.PipeWriter

	inFramesWriter *bufpipe.PipeWriter
	inFramesReader *bufpipe.PipeReader

	streams map[uint32]*stream
}

func newHttp2OverHttp1Conn(h1conn net.Conn, dialFn func() (net.Conn, error)) *http2OverHttp1Conn {
	outr, outw := bufpipe.New(nil)
	inr, inw := bufpipe.New(nil)
	c := &http2OverHttp1Conn{
		h1connCh:        make(chan net.Conn, 1),
		dialFn:          dialFn,
		decoder:         hpack.NewDecoder(4096, nil),
		outFramer:       http2.NewFramer(nil, outr),
		outFramesWriter: outw,
		inFramesWriter:  inw,
		inFramesReader:  inr,
		streams:         make(map[uint32]*stream),
	}
	if h1conn != nil {
		c.h1connCh <- h1conn
	}
	go c.outFramesLoop()
	return c
}

func (c *http2OverHttp1Conn) Close() error         { return nil }
func (c *http2OverHttp1Conn) LocalAddr() net.Addr  { panic("not implemented") }
func (c *http2OverHttp1Conn) RemoteAddr() net.Addr { panic("not implemented") }

func (c *http2OverHttp1Conn) SetDeadline(t time.Time) error      { panic("not implemented") }
func (c *http2OverHttp1Conn) SetReadDeadline(t time.Time) error  { panic("not implemented") }
func (c *http2OverHttp1Conn) SetWriteDeadline(t time.Time) error { panic("not implemented") }

func (c *http2OverHttp1Conn) Read(b []byte) (n int, err error) {
	return c.inFramesReader.Read(b)
}

func (c *http2OverHttp1Conn) Write(b []byte) (n int, err error) {
	return c.outFramesWriter.Write(b)
}

func (c *http2OverHttp1Conn) outFramesLoop() {
	for {
		f, err := c.outFramer.ReadFrame()
		if err != nil {
			log.Println("== http2OverHttp1Conn: read frame err:", err)
			return
		} else {
			streamID := f.Header().StreamID
			if streamID == 0 {
				err = c.handleNonStreamSpecificFrame(f)
			} else {
				err = c.handleStreamSpecificFrame(f)
			}
			if err != nil {
				return
			}
		}
	}
}

func (c *http2OverHttp1Conn) handleNonStreamSpecificFrame(f http2.Frame) error {
	switch f.Header().Type {
	}
	return nil
}

func (c *http2OverHttp1Conn) handleStreamSpecificFrame(fi http2.Frame) error {
	st, ok := c.streams[fi.Header().StreamID]
	if !ok {
		var h1conn net.Conn
		select {
		case c := <-c.h1connCh:
			h1conn = c
		default:
			if c, err := c.dialFn(); err != nil {
				return err
			} else {
				h1conn = c
			}
		}
		st = newStream(fi.Header().StreamID, h1conn, c.inFramesWriter)
	}

	switch fi.Header().Type {
	case http2.FrameHeaders:
		f := fi.(*http2.HeadersFrame)
		if f.HeadersEnded() {
			kvs, err := c.decoder.DecodeFull(f.HeaderBlockFragment())
			if err != nil {
				return err
			}
			headers := newH2headers(kvs)
			if err := st.SendRequest(headers, f.StreamEnded()); err != nil {
				return err
			}
		}
	case http2.FrameData:
		f := fi.(*http2.DataFrame)
		if _, err := st.reqBody.Write(f.Data()); err != nil {
			return err
		}
		if f.StreamEnded() {
			st.reqBody.Close()
		}
	}
	return nil
}
