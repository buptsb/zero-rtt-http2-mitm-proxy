package prefetch

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	ctxio "github.com/dolmen-go/contextio"
	"github.com/kelindar/binary"
	mux "github.com/sagernet/sing-mux"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	eofsignal "github.com/zckevin/go-libs/eof_signal"
	"github.com/zckevin/http2-mitm-proxy/common"
)

type PushChannelClient struct {
	dialFn     func(string) (net.Conn, error)
	cancel     context.CancelFunc
	pushRespCh chan *http.Response
}

func NewPushChannelClient(dialFn func(string) (net.Conn, error), pushRespCh chan *http.Response) *PushChannelClient {
	pc := &PushChannelClient{
		dialFn:     dialFn,
		pushRespCh: pushRespCh,
	}
	ctx, cancel := context.WithCancel(context.Background())
	pc.cancel = cancel
	go pc.run(ctx)
	return pc
}

func (pc *PushChannelClient) handleStream(ctx context.Context, stream net.Conn, metadata M.Metadata) error {
	var hdr PushResponseHeader
	dec := binary.NewDecoder(stream)
	if err := dec.Decode(&hdr); err != nil {
		return fmt.Errorf("failed to decode PushResponseHeader: %w", err)
	}
	req, _ := http.NewRequest(http.MethodGet, hdr.UrlString, nil)
	r := io.MultiReader(bytes.NewBuffer(hdr.ResponseWithoutBody), ctxio.NewReader(ctx, stream))
	resp, err := http.ReadResponse(bufio.NewReader(r), req)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	waitForBodyEof := make(chan error, 1)
	fmt.Println("=====", resp.ContentLength, resp.Header)
	resp.Body = eofsignal.NewBodyEOFSignal(resp.Body, func(err error) error {
		waitForBodyEof <- stream.Close()
		return err
	})
	pc.pushRespCh <- resp
	return <-waitForBodyEof
}

func (pc *PushChannelClient) run(ctx context.Context) error {
	for {
		conn, err := pc.dialFn("")
		if err != nil {
			fmt.Println("dial side channel error: ", err)
			time.Sleep(time.Second)
			continue
		}
		logger := common.NewLogger("pushChannelClient")
		handler := &pushStreamHandler{pc.handleStream}
		err = mux.HandleConnection(ctx, handler, logger, conn, M.Metadata{})
		if err != nil && errors.Is(err, io.EOF) {
			logger.Error("pushChannelClient stop, err: ", err)
		}
		if err == context.Canceled {
			return err
		}
		time.Sleep(time.Second)
	}
}

func (pc *PushChannelClient) Close() error {
	pc.cancel()
	return nil
}

type PushChannelServer struct {
	muxClient *mux.Client
}

func NewPushChannelServer(conn net.Conn) *PushChannelServer {
	muxClient, err := mux.NewClient(mux.Options{
		Dialer:         &singleConnDialer{conn: conn},
		Protocol:       "smux",
		MaxConnections: 1,
	})
	if err != nil {
		panic(err)
	}
	ps := &PushChannelServer{
		muxClient: muxClient,
	}
	return ps
}

var (
	dummyAddr = M.ParseSocksaddr("localhost")
)

func (ps *PushChannelServer) Push(ctx context.Context, resp *http.Response) error {
	st, err := ps.muxClient.DialContext(ctx, N.NetworkTCP, dummyAddr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// stream should be closed by the receiver
	/*
		defer st.Close()
	*/

	dumped, _ := httputil.DumpResponse(resp, false)
	hdr := &PushResponseHeader{
		UrlString:           resp.Request.URL.String(),
		ResponseWithoutBody: dumped,
	}
	if err := binary.MarshalTo(hdr, st); err != nil {
		return fmt.Errorf("failed to marshal PushResponseHeader: %w", err)
	}
	if _, err := io.Copy(st, resp.Body); err != nil {
		return fmt.Errorf("failed to copy body: %w", err)
	}
	return nil
}

func (ps *PushChannelServer) Close() error {
	return ps.muxClient.Close()
}
