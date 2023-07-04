package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/google/martian/v3"
	"github.com/google/martian/v3/h2"
	mlog "github.com/google/martian/v3/log"
	"github.com/google/martian/v3/mitm"
	"github.com/zckevin/demo/muxer"

	_ "net/http/pprof"
)

const (
	pprof = true
)

var (
	addr       = flag.String("addr", ":8080", "host:port of the proxy")
	cert       = flag.String("cert", "", "filepath to the CA certificate used to sign MITM certificates")
	key        = flag.String("key", "", "filepath to the private key of the CA used to sign MITM certificates")
	level      = flag.Int("log-level", 0, "log level")
	serverAddr = flag.String("server-addr", "", "proxy server address")
	debugMode  = flag.Bool("debug", false, "debug mode")
)

func main() {
	if pprof {
		go func() {
			log.Println(http.ListenAndServe("localhost:6061", nil))
		}()
	}

	flag.Parse()
	mlog.SetLevel(*level)
	muxer.DebugMode = *debugMode

	p := martian.NewProxy()
	defer p.Close()

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("martian: starting proxy on %s", l.Addr().String())

	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{
			// InsecureSkipVerify: false,
		},
	}
	p.SetRoundTripper(tr)

	var x509c *x509.Certificate
	var priv interface{}

	if *cert != "" && *key != "" {
		tlsc, err := tls.LoadX509KeyPair(*cert, *key)
		if err != nil {
			log.Fatal(err)
		}
		priv = tlsc.PrivateKey

		x509c, err = x509.ParseCertificate(tlsc.Certificate[0])
		if err != nil {
			log.Fatal(err)
		}
	}

	if x509c != nil && priv != nil {
		mc, err := mitm.NewConfig(x509c, priv)
		if err != nil {
			log.Fatal(err)
		}

		h2Config := &h2.Config{
			AllowedHostsFilter: func(_ string) bool { return true },
			// StreamProcessorFactories: spf,
			EnableDebugLogs: true,
			DialServerConn:  muxer.NewMuxServerConnDialer(*serverAddr, "smux", 1).DialClientStream,
			// use io.Copy() instead of Martian h2 relay
			UseBitwiseCopy: true,
		}
		mc.SetH2Config(h2Config)

		p.SetMITM(mc)
	}

	go p.Serve(l)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)

	<-sigc

	log.Println("martian: shutting down")
	os.Exit(0)
}
