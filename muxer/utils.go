package muxer

import "github.com/sagernet/sing-box/log"

var (
	DebugMode = false
)

func NewLogger(name string) log.ContextLogger {
	logFactory, err := log.New(log.Options{})
	if err != nil {
		panic(err)
	}
	return logFactory.NewLogger(name)
}
