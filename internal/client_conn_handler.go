package internal

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"golang.org/x/net/http2"
)

func BitwiseCopy(cc, sc net.Conn) error {
	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(cc, sc)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(sc, cc)
		errCh <- err
	}()
	return <-errCh
}

func H2ServerCopy(cc, sc net.Conn) error {
	server := &http2.Server{}
	tr := &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			return sc, nil
		},
	}
	client := newHttpClient(tr)
	handler := func(w http.ResponseWriter, r *http.Request) {
		fixRequest(r)
		if r.Body != nil {
			defer r.Body.Close()
		}

		resp, err := client.Do(r)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Println("====ClientHandlerFactory do req err:", err)
			}
			return
		}
		defer resp.Body.Close()

		if err := copyResponse(w, resp); err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Println("====ClientHandlerFactory copy resp err:", err)
			}
		}
	}
	server.ServeConn(cc.(*tls.Conn), &http2.ServeConnOpts{
		Handler: http.HandlerFunc(handler),
	})
	return nil
}
