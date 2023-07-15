package internal

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/sagernet/sing-box/log"
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
		content, "\n",
		dumpReqRespSeperator)
}

func (h *h2MuxHandler) Serve(w http.ResponseWriter, r *http.Request) {
	fixRequest(r)
	if r.Body != nil {
		defer r.Body.Close()
	}

	if DebugMode {
		buf, _ := httputil.DumpRequest(r, true)
		h.dump("== dump request for: ", string(buf), r)
	}

	resp, err := httpClient.RoundTrip(r)
	if err != nil {
		if !IsIgnoredError(err) {
			h.logError(r, "do request err: ", err)
		}
		return
	}
	defer resp.Body.Close()

	if DebugMode {
		buf, _ := httputil.DumpResponse(resp, false)
		h.dump("== dump response for: ", string(buf), r)
	}
	if err := copyResponse(w, resp); err != nil && !IsIgnoredError(err) && !errors.Is(err, io.EOF) {
		h.logError(r, "", err)
	}
}

func h2Relay(h2conn net.Conn) error {
	handler := &h2MuxHandler{
		logger: NewLogger("h2MuxHandler"),
	}
	server := &http2.Server{}
	server.ServeConn(h2conn, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(handler.Serve),
	})
	return nil
}
