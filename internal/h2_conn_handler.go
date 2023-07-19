package internal

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/sagernet/sing-box/log"
	"github.com/zckevin/http2-mitm-proxy/common"
	"github.com/zckevin/http2-mitm-proxy/prefetch"
	"golang.org/x/net/http2"
)

const (
	dumpReqRespSeperator = "=====================\n"
)

type h2MuxHandler struct {
	debug        bool
	isServerSide bool

	logger log.ContextLogger
	client common.HTTPRequestDoer

	pc *prefetch.PrefetchClient
	ps *prefetch.PrefetchServer
}

func newH2MuxHandler(
	isServerSide, debug bool,
	client common.HTTPRequestDoer,
) *h2MuxHandler {
	h := &h2MuxHandler{
		isServerSide: isServerSide,
		debug:        debug,
		logger:       common.NewLogger("h2MuxHandler"),
		client:       client,
	}
	return h
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

func (h *h2MuxHandler) writeInternalError(w http.ResponseWriter, err error) {
	w.Write([]byte("h2MuxHandler: internal Server Error: " + err.Error()))
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusInternalServerError)
}

func (h *h2MuxHandler) Serve(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	common.FixRequest(r)
	r = r.WithContext(ctx)
	if r.Body != nil {
		defer r.Body.Close()
	}

	if h.debug {
		buf, _ := httputil.DumpRequest(r, true)
		h.dump("== dump request for: ", string(buf), r)
	}

	var (
		resp *http.Response
		err  error
	)
	if !h.isServerSide && h.pc.FilterRequest(r) {
		// add client to context for prefetch's racing http client
		r = r.WithContext(context.WithValue(r.Context(), "client", h.client))
		resp, err = h.pc.Do(r)
	} else {
		resp, err = h.client.Do(r)
	}

	if err != nil {
		if !common.IsIgnoredError(err) {
			h.logError(r, "do request err: ", err)
		}
		h.writeInternalError(w, err)
		return
	}
	defer resp.Body.Close()

	if h.isServerSide {
		// TODO: if html load fast and then exit, if we want to cancel all flying prefetch requests?
		h.ps.TryPrefetch( /*ctx,*/ context.Background(), resp)
	}

	if h.debug {
		buf, _ := httputil.DumpResponse(resp, false)
		h.dump("== dump response for: ", string(buf), r)
	}
	if err := common.CopyResponse(w, resp); err != nil /* && !errors.Is(err, io.EOF) */ {
		h.logError(r, "CopyResponse err: ", err)
	}
}

/*
func h2Relay(h2conn net.Conn, client common.HTTPRequestDoer, isServerSide bool) error {
	handler := newH2MuxHandler(isServerSide, common.DebugMode, client)
	server := &http2.Server{}
	server.ServeConn(h2conn, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(handler.Serve),
	})
	return nil
}
*/

func createClientSideH2Relay(
	h2conn net.Conn,
	httpClient common.HTTPRequestDoer,
	pc *prefetch.PrefetchClient,
) error {
	handler := newH2MuxHandler(false, common.DebugMode, httpClient)
	handler.pc = pc
	server := &http2.Server{}
	server.ServeConn(h2conn, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(handler.Serve),
	})
	return nil
}

func createServerSideH2Relay(
	h2conn net.Conn,
	httpClient common.HTTPRequestDoer,
	ps *prefetch.PrefetchServer,
) error {
	handler := newH2MuxHandler(true, common.DebugMode, httpClient)
	handler.ps = ps
	server := &http2.Server{}
	server.ServeConn(h2conn, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(handler.Serve),
	})
	return nil
}
