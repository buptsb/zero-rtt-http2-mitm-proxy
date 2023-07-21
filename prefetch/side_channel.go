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
	"regexp"
	"time"

	ctxio "github.com/dolmen-go/contextio"
	"github.com/kelindar/binary"
	mux "github.com/sagernet/sing-mux"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	eofsignal "github.com/zckevin/go-libs/eof_signal"
	"github.com/zckevin/http2-mitm-proxy/common"
)

var (
	remove_transfer_encoding_regex = regexp.MustCompile(`(?mi)transfer-encoding: chunked\r\n`)
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

// Chunked encoding is handled by net/http(or http2 ?) and it's already de-chunked
// recv it on client side, so we just have to remove it in resp header blob.
func handleHttp1ChunkedEncoding(hdr *PushResponseHeader, stream net.Conn) (io.Reader, error) {
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewBuffer(hdr.ResponseWithoutBody)), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// remove chunked encoding in resp header bytes blob
	if resp.TransferEncoding != nil && resp.TransferEncoding[0] == "chunked" {
		hdr.ResponseWithoutBody = remove_transfer_encoding_regex.ReplaceAll(hdr.ResponseWithoutBody, nil)
	}
	return stream, nil
}

// serve server initiated push stream
func (pc *PushChannelClient) servePushStream(ctx context.Context, stream net.Conn, metadata M.Metadata) (err error) {
	defer func() {
		if err != nil {
			fmt.Println("servePushStream error: ", err)
		}
	}()

	var hdr PushResponseHeader
	dec := binary.NewDecoder(stream)
	if err := dec.Decode(&hdr); err != nil {
		return fmt.Errorf("failed to decode PushResponseHeader: %w", err)
	}
	r, err := handleHttp1ChunkedEncoding(&hdr, stream)
	if err != nil {
		return err
	}
	mr := io.MultiReader(bytes.NewBuffer(hdr.ResponseWithoutBody), ctxio.NewReader(ctx, r))
	req := buildRequest(context.Background(), hdr.UrlString)
	resp, err := http.ReadResponse(bufio.NewReader(mr), req)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	// smux stream handler would close stream after return, so we have to wait for body EOF
	waitForBodyEof := make(chan error, 1)
	resp.Body = eofsignal.NewBodyEOFSignal(resp.Body, func(err error) error {
		waitForBodyEof <- stream.Close()
		return err
	})
	pc.pushRespCh <- resp
	return <-waitForBodyEof
}

func (pc *PushChannelClient) run(ctx context.Context) error {
	for {
		conn, err := pc.dialFn("pushChannelClient")
		if err != nil {
			fmt.Println("dial side channel error: ", err)
			time.Sleep(time.Second)
			continue
		}
		logger := common.NewLogger("pushChannelClient")
		handler := &pushStreamHandler{pc.servePushStream}
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
	defer st.Close()

	dumped, _ := httputil.DumpResponse(resp, false)
	hdr := &PushResponseHeader{
		UrlString:           resp.Request.URL.String(),
		ResponseWithoutBody: dumped,
	}
	if err := binary.MarshalTo(hdr, st); err != nil {
		return fmt.Errorf("failed to marshal PushResponseHeader: %w", err)
	}
	if n, err := io.Copy(st, resp.Body); err != nil {
		return fmt.Errorf("failed to copy body: %w", err)
	} else {
		if resp.ContentLength != -1 && n != resp.ContentLength {
			return fmt.Errorf("content length mismatch, expect %d, got %d", resp.ContentLength, n)
		}
	}
	return nil
}

func (ps *PushChannelServer) Close() error {
	return ps.muxClient.Close()
}
