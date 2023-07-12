package muxer

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var (
	DebugMode = false
)

func NewLogger(name string) log.ContextLogger {
	logFactory, err := log.New(log.Options{
		Options: option.LogOptions{
			Timestamp: true,
		},
	})
	if err != nil {
		panic(err)
	}
	return logFactory.NewLogger(name)
}

func SpawnPprofServer(port int) {
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
	if err != nil {
		panic(err)
	}
}
