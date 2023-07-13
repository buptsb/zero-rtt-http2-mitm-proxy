package internal

import (
	"io"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/nadoo/glider/pkg/pool"
	"github.com/sagernet/sing-box/log"
	"golang.org/x/exp/maps"
	"golang.org/x/net/http2"
)

const (
	dumpReqRespSeperator = "=====================\n"
)

type h2MuxHandler struct {
	logger log.ContextLogger
	client *autoFallbackClient
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
	// only in response request, need to reset for sending
	r.RequestURI = ""

	// add missing fields in response request
	r.URL.Host = r.Host
	r.URL.Scheme = "https"

	if DebugMode {
		buf, _ := httputil.DumpRequest(r, true)
		h.dump("== dump request for: ", string(buf), r)
	}

	// Don't send any DATA frame if request does not has any content,
	// which will send END_STREAM in HEADERS instead of DATA frame.
	// Fix #2
	if r.ContentLength == 0 {
		r.Body = nil
	} else {
		defer r.Body.Close()
	}

	resp, err := h.client.Do(r)
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

	// copy headers
	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	fn := func() error {
		buf := pool.GetBuffer(4096)
		defer pool.PutBuffer(buf)

		n, err := resp.Body.Read(buf)
		if err != nil && err != io.EOF {
			if !IsIgnoredError(err) {
				h.logError(r, "body read err: ", err)
			}
			return err
		}
		if _, werr := w.Write(buf[:n]); werr != nil {
			if err != io.EOF && !IsIgnoredError(werr) {
				h.logError(r, "write err: ", werr)
			}
			return werr
		}
		return err
	}
	for {
		if err := fn(); err != nil {
			break
		}
	}
}

func h2Relay(h2conn net.Conn) error {
	logger := NewLogger("h2MuxHandler")
	handler := &h2MuxHandler{
		logger: logger,
		client: newAutoFallbackClient(logger),
	}
	server := &http2.Server{}
	server.ServeConn(h2conn, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(handler.Serve),
	})
	return nil
}
