package prefetch

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"

	"github.com/kelindar/binary"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zckevin/http2-mitm-proxy/common"
)

var _ = Describe("PushChannel", func() {
	go func() {
		_ = http.ListenAndServe("localhost:6060", nil)
	}()

	Context("protocol", func() {
		It("kelindar/binary should not read beyond the header", func() {
			// kelindar/binary will copy buffer's bytes if the reader is a bytes.Buffer,
			// using a wrapper to prevent it.
			type bytesBufferWrapper struct {
				*bytes.Buffer
			}

			suffix := common.RandomString(1024)
			makeHeader := func(url string) *bytesBufferWrapper {
				respHdr := &PushResponseHeader{
					UrlString: url,
				}
				buf, err := binary.Marshal(respHdr)
				Expect(err).To(BeNil())
				bb := bytes.NewBuffer(buf)
				bb.WriteString(suffix)
				return &bytesBufferWrapper{bb}
			}
			lens := []int{
				5, 13, 307, 4001, 10001, 100000,
			}

			for _, n := range lens {
				url := common.RandomString(n)
				buf := makeHeader(url)
				dec := binary.NewDecoder(buf)
				var hdr PushResponseHeader
				err := dec.Decode(&hdr)
				Expect(err).To(BeNil())
				Expect(url).To(Equal(hdr.UrlString))
				Expect(suffix).To(Equal(buf.String()))
			}
		})
	})

	Context("client/server", func() {
		respUrl := "http://example.com/"
		newResponse := func(body string) *http.Response {
			req, _ := http.NewRequest(http.MethodGet, respUrl, nil)
			resp := &http.Response{
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Request:       req,
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(bytes.NewBufferString(body)),
				ContentLength: int64(len(body)),
				Header:        make(http.Header),
			}
			resp.Header.Set("Content-Type", "text/html; charset=utf-8")
			return resp
		}

		It("should work", func() {
			cc, sc := net.Pipe()
			body := "hello"
			pushRespCh := make(chan *http.Response, 1)

			client := NewPushChannelClient(func(s string) (net.Conn, error) {
				return cc, nil
			}, pushRespCh)
			server := NewPushChannelServer(sc)
			defer client.Close()
			defer server.Close()

			dumped, _ := httputil.DumpResponse(newResponse(body), true)
			for i := 0; i < 3; i++ {
				go server.Push(context.Background(), newResponse(body))

				resp := <-pushRespCh
				buf, err := httputil.DumpResponse(resp, true)
				Expect(err).To(BeNil())
				Expect(string(dumped)).To(Equal(string(buf)))
			}
		})
	})
})
