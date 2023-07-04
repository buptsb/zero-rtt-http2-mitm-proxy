package muxer

import (
	"io"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/sagernet/sing-box/log"
	"golang.org/x/exp/maps"
	"golang.org/x/net/http2"
)

const (
	dumpReqRespSeperator = "=====================\n"
)

type h2MuxHandler struct {
	logger log.ContextLogger
}

func (h *h2MuxHandler) logError(r *http.Request, desc string, err error) {
	url := r.URL.String()
	h.logger.Error("http2HandlerFunc: [", url, "] ", desc, err)
}

func (h *h2MuxHandler) dump(desc, content string, r *http.Request) {
	h.logger.Debug(desc, r.URL.String(), "\n",
		dumpReqRespSeperator,
		content,
		dumpReqRespSeperator)
}

func (h *h2MuxHandler) Serve(w http.ResponseWriter, r *http.Request) {
	// only in response request, need to reset for sending
	r.RequestURI = ""

	// add missing fields in response request
	r.URL.Host = r.Host
	r.URL.Scheme = "https"

	if DebugMode {
		buf, _ := httputil.DumpRequest(r, true)
		h.dump("== dump request for: ", string(buf), r)
	}

	resp, err := defaultHTTPClient.Do(r)
	if err != nil {
		h.logError(r, "do request err: ", err)
		return
	}
	defer resp.Body.Close()

	if DebugMode {
		buf, _ := httputil.DumpResponse(resp, false)
		h.dump("== dump response for: ", string(buf), r)
	}

	// copy headers
	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	for {
		buf := make([]byte, 1024)
		n, err := resp.Body.Read(buf)
		if err != nil && err != io.EOF {
			h.logError(r, "body read err: ", err)
			break
		}
		if _, werr := w.Write(buf[:n]); werr != nil {
			h.logError(r, "write err: ", err)
			break
		}
		if err == io.EOF {
			break
		}
	}
}

func serveHTTP2Conn(h2conn net.Conn) error {
	handler := &h2MuxHandler{
		logger: NewLogger("h2MuxHandler"),
	}
	server := &http2.Server{}
	server.ServeConn(h2conn, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(handler.Serve),
	})
	return nil
}
