package muxer

import (
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
