package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	_ "net/http/pprof"

	mlog "github.com/google/martian/v3/log"
	"github.com/sagernet/sing-box/log"
	mux "github.com/sagernet/sing-mux"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/zckevin/demo/muxer"
)

const (
	pprof = true
)

var (
	level = flag.Int("log-level", 0, "log level")
)

func newLogger() log.ContextLogger {
	logFactory, err := log.New(log.Options{})
	if err != nil {
		panic(err)
	}
	return logFactory.NewLogger("muxer")
}

func demuxConn(conn net.Conn) {
	handler := muxer.NewH2Handler()
	err := mux.HandleConnection(context.TODO(), handler, newLogger(), conn, M.Metadata{})
	if err != nil {
		log.Error("== demuxConn err", err)
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
	if pprof {
		go func() {
			http.ListenAndServe("localhost:6060", nil)
		}()
	}

	flag.Parse()
	mlog.SetLevel(*level)

	listener()
}
