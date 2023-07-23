package main

import (
	"context"
	"flag"
	"net"
	_ "net/http/pprof"

	mlog "github.com/google/martian/v3/log"
	slog "github.com/sagernet/sing-box/log"
	mux "github.com/sagernet/sing-mux"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/zckevin/http2-mitm-proxy/common"
	"github.com/zckevin/http2-mitm-proxy/internal"
	"github.com/zckevin/http2-mitm-proxy/tracing"
	"go.opentelemetry.io/otel"
)

var (
	pprof     = flag.Bool("pprof", true, "enable pprof")
	pprofPort = flag.Int("pprof-port", 6060, "pprof port")
	debugMode = flag.Bool("debug", false, "debug mode")
	level     = flag.Int("log-level", 0, "log level, 0-3")

	listenAddr = flag.String("listen-addr", ":20001", "listen address")
	relayType  = flag.String("relay-type", "h2", "relay type")
)

func demuxConn(conn net.Conn) {
	logger := common.NewLogger("server")
	muxHandler := internal.NewMuxHandler(*relayType)
	err := mux.HandleConnection(context.TODO(), muxHandler, logger, conn, M.Metadata{})
	if err != nil {
		logger.Error("demuxConn err: ", err)
	}
}

func listener() {
	slog.Info("starting server on ", *listenAddr)
	l, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		slog.Fatal(err)
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			slog.Fatal(err)
		}
		go demuxConn(conn)
	}
}

func main() {
	flag.Parse()
	if *pprof {
		go common.SpawnPprofServer(*pprofPort)
	}
	mlog.SetLevel(*level)
	if *debugMode {
		common.LogFactory.SetLevel(slog.LevelDebug)
	} else {
		common.LogFactory.SetLevel(slog.LevelError)
	}
	common.LogFactory.SetLevel(slog.LevelDebug)
	common.DebugMode = *debugMode

	tp, err := tracing.TraceProvider()
	if err != nil {
		panic(err)
	}
	otel.SetTracerProvider(tp)

	listener()
}
