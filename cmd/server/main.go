package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"

	mlog "github.com/google/martian/v3/log"
	"github.com/sagernet/sing-box/log"
	mux "github.com/sagernet/sing-mux"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/zckevin/http2-mitm-proxy/muxer"
)

var (
	pprof     = flag.Bool("pprof", true, "enable pprof")
	pprofPort = flag.Int("pprof-port", 6060, "pprof port")
	debugMode = flag.Bool("debug", false, "debug mode")

	level     = flag.Int("log-level", 0, "log level")
	relayType = flag.String("relay-type", "h2", "relay type")
)

func demuxConn(conn net.Conn) {
	logger := muxer.NewLogger("server")
	muxHandler := muxer.NewMuxHandler(*relayType)
	err := mux.HandleConnection(context.TODO(), muxHandler, logger, conn, M.Metadata{})
	if err != nil {
		logger.Error("demuxConn err: ", err)
	}
}

func listener() {
	l, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go demuxConn(conn)
	}
}

func main() {
	if *pprof {
		go func() {
			http.ListenAndServe(fmt.Sprintf("localhost:%d", *pprofPort), nil)
		}()
	}

	flag.Parse()
	mlog.SetLevel(*level)
	muxer.DebugMode = *debugMode

	listener()
}
