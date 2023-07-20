package common

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var (
	DebugMode  = false
	LogFactory log.Factory
)

func init() {
	_ = NewLogger("common")
}

func NewLogger(name string) log.ContextLogger {
	if LogFactory == nil {
		f, err := log.New(log.Options{
			Options: option.LogOptions{
				Timestamp: true,
			},
		})
		if err != nil {
			panic(err)
		}
		LogFactory = f
	}
	return LogFactory.NewLogger(name)
}

func SpawnPprofServer(port int) {
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
	if err != nil {
		panic(err)
	}
}

func IsIgnoredError(err error) bool {
	return errors.Is(err, context.Canceled)
}

func IsNetCancelError(err error) bool {
	return strings.Contains(err.Error(), "operation was canceled")
}

func RandomString(length int) string {
	b := make([]byte, length+2)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[2 : length+2]
}
