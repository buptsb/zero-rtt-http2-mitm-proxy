package http2to1

import (
	"errors"
	"io"
	"net"
)

type stream struct {
	streamID   uint32
	h1conn     net.Conn
	reqBody    *requestBody
	respWriter io.Writer
}

func newStream(streamID uint32, h1conn net.Conn, respWriter io.Writer) *stream {
	s := &stream{
		streamID:   streamID,
		h1conn:     h1conn,
		respWriter: respWriter,
	}
	go s.pipeResponse()
	return s
}

func (s *stream) SendRequest(headers *h2headers, streamEnded bool) error {
	if s.reqBody != nil {
		return errors.New("request already sent")
	}
	s.reqBody = newRequestBody(streamEnded)
	req, err := headers.NewRequest(s.reqBody)
	if err != nil {
		return err
	}
	return req.Write(s.h1conn)
}

func (s *stream) pipeResponse() {
	// resp, err := http.ReadResponse(bufio.NewReader(s.h1conn), nil)
	// if err != nil {
	// 	log.Println("=== pipeResponse err:", err)
	// 	return
	// }
}

type requestBody struct {
	bodyr       io.Reader
	bodyw       io.WriteCloser
	streamEnded bool
}

func newRequestBody(streamEnded bool) *requestBody {
	// r, w := bufpipe.New(nil)
	r, w := io.Pipe()
	return &requestBody{
		bodyr:       r,
		bodyw:       w,
		streamEnded: streamEnded,
	}
}

func (req *requestBody) Read(b []byte) (n int, err error) {
	return req.bodyr.Read(b)
}

func (req *requestBody) Write(b []byte) (n int, err error) {
	return req.bodyw.Write(b)
}

func (req *requestBody) Close() error {
	return req.bodyw.Close()
}
